//Copyright (c) 2025, Julian Huhn
//
//Permission to use, copy, modify, and/or distribute this software for any
//purpose with or without fee is hereby granted, provided that the above
//copyright notice and this permission notice appear in all copies.
//
//THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
//WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
//MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
//ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
//WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
//ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
//OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

package matrix

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/huhnsystems/gohenry/internal/config"
	"github.com/huhnsystems/gohenry/internal/domain"
)

// loadSyncToken loads the sync token from a file
func loadSyncToken(tokenFile string) (string, error) {
	// Check if token file exists
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		return "", nil // No token file, return empty string
	}

	// Read token from file
	data, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("failed to read sync token file: %v", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// saveSyncToken saves the sync token to a file
func saveSyncToken(tokenFile, token string) error {
	return ioutil.WriteFile(tokenFile, []byte(token), 0600)
}

// Client handles interactions with the Matrix server
type Client struct {
	client            *mautrix.Client
	userID            id.UserID
	config            *config.Config
	allowedDomain     string
	messageHandler    func(ctx context.Context, evt *event.Event)
	syncToken         string
	lastProcessedTime int64
	startupTime       int64
}

// NewClient creates a new Matrix client
func NewClient(cfg *config.Config) *Client {
	// Load the sync token if it exists
	syncToken, err := loadSyncToken(cfg.SyncTokenFile)
	if err != nil {
		log.Printf("WARNING: Failed to load sync token: %v", err)
	} else if syncToken != "" {
		log.Printf("Loaded sync token: %s", syncToken)
	}

	// Record the startup time in milliseconds (Matrix timestamp format)
	startupTime := time.Now().UnixNano() / 1e6

	return &Client{
		config:            cfg,
		allowedDomain:     cfg.AllowedDomain,
		syncToken:         syncToken,
		startupTime:       startupTime,
		lastProcessedTime: startupTime,
	}
}

// Connect initializes the Matrix client and connects to the homeserver
func (c *Client) Connect(ctx context.Context) error {
	log.Printf(
		"Creating Matrix client for %s with homeserver %s",
		c.config.MatrixUserID,
		c.config.MatrixHomeserver,
	)

	// Create client
	client, err := mautrix.NewClient(
		c.config.MatrixHomeserver,
		id.UserID(c.config.MatrixUserID),
		c.config.MatrixAccessToken,
	)
	if err != nil {
		log.Printf("ERROR: Failed to create Matrix client: %v", err)
		return fmt.Errorf("failed to create Matrix client: %v", err)
	}
	log.Printf("Matrix client created successfully")

	// Always try to login with password if available (token could be expired)
	if c.config.MatrixPassword != "" {
		log.Printf("Attempting to log in with password")

		// Get username without @ prefix
		username := strings.TrimPrefix(string(c.config.MatrixUserID), "@")
		// If the username contains a domain part, remove it
		if idx := strings.Index(username, ":"); idx >= 0 {
			username = username[:idx]
		}

		log.Printf("Logging in as user: %s", username)

		resp, err := client.Login(&mautrix.ReqLogin{
			Type: "m.login.password",
			Identifier: mautrix.UserIdentifier{
				Type: "m.id.user",
				User: username,
			},
			Password:         c.config.MatrixPassword,
			StoreCredentials: true,
		})
		if err != nil {
			log.Printf("ERROR: Failed to log in to Matrix: %v", err)

			// Clear the sync token if login fails
			if c.syncToken != "" {
				log.Printf("Clearing sync token due to login failure")
				c.syncToken = ""
				if err := saveSyncToken(c.config.SyncTokenFile, ""); err != nil {
					log.Printf(
						"WARNING: Failed to clear sync token file: %v",
						err,
					)
				}
			}

			return fmt.Errorf("failed to log in to Matrix: %v", err)
		}
		client.SetCredentials(resp.UserID, resp.AccessToken)
		log.Printf("Successfully logged in as %s", resp.UserID)
		log.Printf("Access token: %s", resp.AccessToken)
	} else if c.config.MatrixAccessToken != "" {
		log.Printf("Using existing access token for authentication (no password available)")
	} else {
		return fmt.Errorf("no access token or password available for authentication")
	}

	// Skip the initial sync for debugging - it can hang if the homeserver is unreachable
	log.Printf("Skipping initial sync for faster startup...")

	// Start a background sync with a short timeout just to test connectivity
	syncCtx, cancelSync := context.WithTimeout(ctx, 5*time.Second)
	defer cancelSync()

	syncErrorCh := make(chan error, 1)
	go func() {
		syncErrorCh <- client.SyncWithContext(syncCtx)
	}()

	// Don't wait for sync to complete - just make sure the client is created
	select {
	case err := <-syncErrorCh:
		if err != nil && err != context.DeadlineExceeded &&
			err != context.Canceled {
			log.Printf("WARNING: Sync returned error: %v", err)
			// Don't fail here, just log the warning
		} else if err == nil {
			log.Printf("Initial sync completed successfully")
		}
	case <-time.After(100 * time.Millisecond):
		// Sync has started but hasn't returned an error after 100ms
		log.Printf("Sync has started in background")
	}

	c.client = client
	c.userID = id.UserID(c.config.MatrixUserID)
	log.Printf("Matrix client initialized with user ID: %s", c.userID)

	return nil
}

// SetMessageHandler sets the function to handle incoming messages
func (c *Client) SetMessageHandler(
	handler func(ctx context.Context, evt *event.Event),
) {
	c.messageHandler = handler
}

// ListenForMessages starts listening for messages in Matrix rooms
func (c *Client) ListenForMessages(ctx context.Context) error {
	if c.messageHandler == nil {
		return fmt.Errorf("message handler not set")
	}

	// Set up sync callback
	syncer := c.client.Syncer.(*mautrix.DefaultSyncer)

	log.Printf("Setting up Matrix event handlers...")

	// Register event handler
	syncer.OnEvent(func(source mautrix.EventSource, evt *event.Event) {
		// Skip old events when restarting
		// 1. Check if the event has a timestamp and it's older than our startup time
		if evt.Timestamp > 0 && evt.Timestamp < c.startupTime {
			return
		}

		// 2. Check if we've already processed a newer event
		if evt.Timestamp > 0 && evt.Timestamp <= c.lastProcessedTime {
			return
		}

		// Update our last processed timestamp if this event has a timestamp
		if evt.Timestamp > 0 {
			c.lastProcessedTime = evt.Timestamp
		}

		// Handle room membership events (invites)
		if evt.Type == event.StateMember {
			member := evt.Content.AsMember()
			if member != nil {
				if member.Membership == event.MembershipInvite &&
					evt.GetStateKey() == string(c.userID) {
					log.Printf(
						"INVITE DETECTED - Received invite to room %s from %s",
						evt.RoomID,
						evt.Sender,
					)

					// Auto-accept the invitation
					log.Printf("Attempting to join room %s", evt.RoomID)
					_, err := c.client.JoinRoom(string(evt.RoomID), "", nil)
					if err != nil {
						log.Printf("Failed to join room: %v", err)
					} else {
						log.Printf("Successfully joined room %s", evt.RoomID)
					}
				}
			}
		}

		// Only process message events for chat messages
		if evt.Type != event.EventMessage {
			return
		}

		// Skip messages from ourselves
		if evt.Sender == c.userID {
			return
		}

		log.Printf("Processing message from %s in room %s (timestamp: %d)",
			evt.Sender, evt.RoomID, evt.Timestamp)

		// Process the message with the handler in a separate goroutine
		go c.messageHandler(ctx, evt)
	})

	log.Printf("Starting Matrix sync loop...")

	// If we have a token, use it as the starting point to avoid processing old messages
	if c.syncToken != "" {
		log.Printf(
			"Using stored sync token: %s to skip old messages",
			c.syncToken,
		)
		c.client.Store.SaveNextBatch("", c.syncToken)
	}

	// Use a more robust sync method with automatic reconnection
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Printf("Sync loop terminated due to context cancellation")
				return
			default:
				// Start the main sync process - this runs continuously until error
				log.Printf("Starting Matrix continuous sync")
				err := c.client.Sync()

				// We should only get here if Sync() returns, which means an error or cancellation

				// Check for token errors which require re-authentication
				if err != nil &&
					(strings.Contains(err.Error(), "M_UNKNOWN_TOKEN") ||
						strings.Contains(err.Error(), "Invalid access token") ||
						strings.Contains(err.Error(), "401")) {
					log.Printf(
						"AUTHENTICATION ERROR: %v - clearing sync token and exiting sync loop",
						err,
					)
					// Clear the sync token since it's likely associated with the invalid token
					c.syncToken = ""
					if saveErr := saveSyncToken(c.config.SyncTokenFile, ""); saveErr != nil {
						log.Printf(
							"WARNING: Failed to clear sync token file: %v",
							saveErr,
						)
					}
					// Exit the loop to allow reconnection with fresh login
					return
				}

				// Get the latest token and save it before restarting
				if nextBatch := c.client.Store.LoadNextBatch(""); nextBatch != "" {
					c.syncToken = nextBatch
					log.Printf(
						"Saving sync token: %s for next restart",
						c.syncToken,
					)
					if saveErr := saveSyncToken(c.config.SyncTokenFile, c.syncToken); saveErr != nil {
						log.Printf(
							"WARNING: Failed to save sync token: %v",
							saveErr,
						)
					}
				}

				if err != nil {
					log.Printf("Sync error: %v - will retry in 5 seconds", err)
					time.Sleep(5 * time.Second)
				} else {
					log.Printf("Sync completed normally, which should not happen - will restart")
				}
			}
		}
	}()

	return nil
}

// Disconnect logs out from the Matrix server
func (c *Client) Disconnect() error {
	_, err := c.client.Logout()
	return err
}

// JoinRoom attempts to join a Matrix room
func (c *Client) JoinRoom(roomID string) error {
	_, err := c.client.JoinRoom(roomID, "", nil)
	return err
}

// SendMessage sends a message to a Matrix room
func (c *Client) SendMessage(roomID string, message string) error {
	content := map[string]interface{}{
		"msgtype": "m.text",
		"body":    message,
	}
	_, err := c.client.SendMessageEvent(
		id.RoomID(roomID),
		event.EventMessage,
		content,
	)
	return err
}

// GetRoomContext retrieves past messages from a room for context
func (c *Client) GetRoomContext(
	ctx context.Context,
	roomID string,
	limit int,
) ([]*event.Event, error) {
	// Get the most recent messages, using a filter to only get message events
	filter := mautrix.FilterPart{
		Types: []event.Type{event.EventMessage},
	}

	// Request limit+10 messages to account for potential filtering
	requestLimit := limit + 10
	log.Printf(
		"Requesting %d messages for context from room %s (limit: %d)",
		requestLimit,
		roomID,
		limit,
	)

	// Try to get messages from the room history
	resp, err := c.client.Messages(
		id.RoomID(roomID),
		"",
		"",
		mautrix.DirectionBackward,
		&filter,
		requestLimit,
	)
	if err != nil {
		log.Printf("Error fetching messages: %v", err)
		return nil, err
	}

	// Check if we got any messages at all
	if len(resp.Chunk) == 0 {
		log.Printf(
			"WARNING: No messages returned from Matrix API for room %s",
			roomID,
		)

		// Try to get messages another way by looking at room state/timeline
		// This is a fallback method when the Messages API doesn't work
		stateResp, err := c.client.State(id.RoomID(roomID))
		if err != nil {
			log.Printf("Error fetching room state: %v", err)
			return nil, err
		}

		// Try to get events from the room's state
		var events []*event.Event
		for _, evts := range stateResp {
			for _, evt := range evts {
				if evt.Type == event.EventMessage {
					events = append(events, evt)
				}
			}
		}

		if len(events) > 0 {
			log.Printf("Found %d messages in room state instead", len(events))
			resp.Chunk = events
		} else {
			log.Printf("WARNING: Still couldn't find messages in room state")
			// Just log that we couldn't get any messages
			log.Printf("WARNING: Couldn't retrieve any message history for room %s", roomID)
		}
	}

	// Filter and process the events
	var filteredEvents []*event.Event

	log.Printf("Processing %d events from room %s", len(resp.Chunk), roomID)

	for _, evt := range resp.Chunk {
		// Debug event information
		log.Printf("Event type: %v, sender: %s", evt.Type, evt.Sender)

		// Skip non-message events
		if evt.Type != event.EventMessage {
			continue
		}

		// For debug only - check raw content
		contentBytes, _ := evt.Content.MarshalJSON()
		log.Printf("Event content: %s", string(contentBytes))

		// Parse the message content
		content, ok := evt.Content.Parsed.(*event.MessageEventContent)
		if !ok {
			log.Printf("Failed to parse message content for event %s", evt.ID)
			// Try to parse it manually
			if msgType, hasType := evt.Content.Raw["msgtype"].(string); hasType &&
				msgType == "m.text" {
				if body, hasBody := evt.Content.Raw["body"].(string); hasBody &&
					body != "" {
					content = &event.MessageEventContent{
						MsgType: event.MessageType(msgType),
						Body:    body,
					}
					ok = true
				}
			}

			if !ok {
				continue
			}
		}

		// Skip empty messages
		if content == nil || content.Body == "" {
			continue
		}

		// Include messages from the bot for proper context
		// We need to include bot messages to maintain conversation flow
		filteredEvents = append(filteredEvents, evt)

		// Only keep up to the limit
		if len(filteredEvents) >= limit {
			break
		}
	}

	// Log retrieval of context messages for debugging
	log.Printf(
		"Retrieved %d messages (after filtering) for context from room %s",
		len(filteredEvents),
		roomID,
	)

	// If we still have no messages, log a warning
	if len(filteredEvents) == 0 {
		log.Printf(
			"WARNING: Failed to retrieve any usable messages from room %s",
			roomID,
		)
	}

	return filteredEvents, nil
}

// IsFromAllowedDomain checks if a user is from the allowed domain
func (c *Client) IsFromAllowedDomain(userID string) bool {
	parts := strings.Split(userID, ":")
	if len(parts) != 2 {
		return false
	}
	return parts[1] == c.allowedDomain
}

// IsAddressedToBot checks if a message is addressed to the bot in a group chat
func (c *Client) IsAddressedToBot(
	content string,
	roomType domain.RoomType,
) bool {
	// In direct messages, always respond
	if roomType == domain.DirectRoom {
		return true
	}

	// Get bot's full username without the @ prefix (e.g., "henry:henhouse.im")
	fullUsername := strings.TrimPrefix(string(c.userID), "@")

	// Get the local part of the username (e.g., "henry" from "henry:henhouse.im")
	localpart := fullUsername
	if idx := strings.Index(fullUsername, ":"); idx >= 0 {
		localpart = fullUsername[:idx]
	}

	// Check for various mention forms
	fullMention := fmt.Sprintf("@%s", fullUsername) // @henry:henhouse.im

	content = strings.ToLower(content)
	localpart = strings.ToLower(localpart)

	// Check if content contains either the full mention or just the name

	// First check for full mention (@henry:henhouse.im)
	if strings.Contains(content, fullMention) {
		return true
	}

	// Case-insensitive check for just the name
	// First just do a basic check if the lowercase name is in the content
	if strings.Contains(content, localpart) {
		// If we have a simple match, let's do some more checks to verify it's not
		// part of another word (like "henrysmith")

		// We'll split the content into words and check each one
		words := strings.Fields(content)
		for _, word := range words {
			// Clean the word from punctuation
			word = strings.ToLower(strings.Trim(word, ",.!?:;\"'()[]{}"))

			// Direct match
			if word == localpart {
				return true
			}

			// Match with trailing punctuation
			if strings.HasPrefix(word, localpart) &&
				len(word) > len(localpart) {
				nextChar := word[len(localpart)]
				// If the next char is punctuation, it's likely a match
				if nextChar == ',' || nextChar == '.' || nextChar == '!' ||
					nextChar == '?' || nextChar == ':' || nextChar == ';' {
					return true
				}
			}
		}
	}

	// One more check: check if the content has the bot's name with a colon after it
	// like "henry: do something" which is a common way to address bots
	colonCheck := localpart + ":"
	return strings.Contains(content, colonCheck)
}

// GetBotUserID returns the bot's Matrix user ID
func (c *Client) GetBotUserID() string {
	return string(c.userID)
}

// GetMatrixClient returns the underlying mautrix client
func (c *Client) GetMatrixClient() *mautrix.Client {
	return c.client
}

// SendTyping sends a typing notification to a Matrix room
func (c *Client) SendTyping(roomID string, typing bool, timeout int) error {
	// Convert timeout from milliseconds to time.Duration
	timeoutDuration := time.Duration(timeout) * time.Millisecond
	_, err := c.client.UserTyping(id.RoomID(roomID), typing, timeoutDuration)
	return err
}
