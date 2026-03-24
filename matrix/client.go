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

	"github.com/huhndev/gohenry/config"
	"github.com/huhndev/gohenry/domain"
)

func loadSyncToken(tokenFile string) (string, error) {
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		return "", nil
	}
	data, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("failed to read sync token file: %v", err)
	}
	return strings.TrimSpace(string(data)), nil
}

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

func NewClient(cfg *config.Config) *Client {
	syncToken, err := loadSyncToken(cfg.SyncTokenFile)
	if err != nil {
		log.Printf("WARNING: Failed to load sync token: %v", err)
	} else if syncToken != "" {
		log.Printf("Loaded sync token: %s", syncToken)
	}

	startupTime := time.Now().UnixNano() / 1e6

	return &Client{
		config:            cfg,
		allowedDomain:     cfg.AllowedDomain,
		syncToken:         syncToken,
		startupTime:       startupTime,
		lastProcessedTime: startupTime,
	}
}

func (c *Client) Connect(ctx context.Context) error {
	log.Printf(
		"Creating Matrix client for %s with homeserver %s",
		c.config.MatrixUserID,
		c.config.MatrixHomeserver,
	)

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

	if c.config.MatrixPassword != "" {
		log.Printf("Attempting to log in with password")

		username := strings.TrimPrefix(string(c.config.MatrixUserID), "@")
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

			if c.syncToken != "" {
				log.Printf("Clearing sync token due to login failure")
				c.syncToken = ""
				if err := saveSyncToken(c.config.SyncTokenFile, ""); err != nil {
					log.Printf("WARNING: Failed to clear sync token file: %v", err)
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

	log.Printf("Skipping initial sync for faster startup...")

	syncCtx, cancelSync := context.WithTimeout(ctx, 5*time.Second)
	defer cancelSync()

	syncErrorCh := make(chan error, 1)
	go func() {
		syncErrorCh <- client.SyncWithContext(syncCtx)
	}()

	select {
	case err := <-syncErrorCh:
		if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			log.Printf("WARNING: Sync returned error: %v", err)
		} else if err == nil {
			log.Printf("Initial sync completed successfully")
		}
	case <-time.After(100 * time.Millisecond):
		log.Printf("Sync has started in background")
	}

	c.client = client
	c.userID = id.UserID(c.config.MatrixUserID)
	log.Printf("Matrix client initialized with user ID: %s", c.userID)

	return nil
}

func (c *Client) SetMessageHandler(handler func(ctx context.Context, evt *event.Event)) {
	c.messageHandler = handler
}

func (c *Client) ListenForMessages(ctx context.Context) error {
	if c.messageHandler == nil {
		return fmt.Errorf("message handler not set")
	}

	syncer := c.client.Syncer.(*mautrix.DefaultSyncer)

	log.Printf("Setting up Matrix event handlers...")

	syncer.OnEvent(func(source mautrix.EventSource, evt *event.Event) {
		if evt.Timestamp > 0 && evt.Timestamp < c.startupTime {
			return
		}
		if evt.Timestamp > 0 && evt.Timestamp <= c.lastProcessedTime {
			return
		}
		if evt.Timestamp > 0 {
			c.lastProcessedTime = evt.Timestamp
		}

		if evt.Type == event.StateMember {
			member := evt.Content.AsMember()
			if member != nil {
				if member.Membership == event.MembershipInvite &&
					evt.GetStateKey() == string(c.userID) {
					log.Printf(
						"INVITE DETECTED - Received invite to room %s from %s",
						evt.RoomID, evt.Sender,
					)
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

		if evt.Type != event.EventMessage {
			return
		}
		if evt.Sender == c.userID {
			return
		}

		log.Printf("Processing message from %s in room %s (timestamp: %d)",
			evt.Sender, evt.RoomID, evt.Timestamp)

		go c.messageHandler(ctx, evt)
	})

	log.Printf("Starting Matrix sync loop...")

	if c.syncToken != "" {
		log.Printf("Using stored sync token: %s to skip old messages", c.syncToken)
		c.client.Store.SaveNextBatch("", c.syncToken)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Printf("Sync loop terminated due to context cancellation")
				return
			default:
				log.Printf("Starting Matrix continuous sync")
				err := c.client.Sync()

				if err != nil &&
					(strings.Contains(err.Error(), "M_UNKNOWN_TOKEN") ||
						strings.Contains(err.Error(), "Invalid access token") ||
						strings.Contains(err.Error(), "401")) {
					log.Printf(
						"AUTHENTICATION ERROR: %v - clearing sync token and exiting sync loop",
						err,
					)
					c.syncToken = ""
					if saveErr := saveSyncToken(c.config.SyncTokenFile, ""); saveErr != nil {
						log.Printf("WARNING: Failed to clear sync token file: %v", saveErr)
					}
					return
				}

				if nextBatch := c.client.Store.LoadNextBatch(""); nextBatch != "" {
					c.syncToken = nextBatch
					log.Printf("Saving sync token: %s for next restart", c.syncToken)
					if saveErr := saveSyncToken(c.config.SyncTokenFile, c.syncToken); saveErr != nil {
						log.Printf("WARNING: Failed to save sync token: %v", saveErr)
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

func (c *Client) Disconnect() error {
	_, err := c.client.Logout()
	return err
}

func (c *Client) JoinRoom(roomID string) error {
	_, err := c.client.JoinRoom(roomID, "", nil)
	return err
}

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

func (c *Client) GetRoomContext(
	ctx context.Context,
	roomID string,
	limit int,
) ([]*event.Event, error) {
	filter := mautrix.FilterPart{
		Types: []event.Type{event.EventMessage},
	}

	requestLimit := limit + 10
	log.Printf(
		"Requesting %d messages for context from room %s (limit: %d)",
		requestLimit, roomID, limit,
	)

	resp, err := c.client.Messages(
		id.RoomID(roomID), "", "",
		mautrix.DirectionBackward,
		&filter, requestLimit,
	)
	if err != nil {
		log.Printf("Error fetching messages: %v", err)
		return nil, err
	}

	if len(resp.Chunk) == 0 {
		log.Printf("WARNING: No messages returned from Matrix API for room %s", roomID)

		stateResp, err := c.client.State(id.RoomID(roomID))
		if err != nil {
			log.Printf("Error fetching room state: %v", err)
			return nil, err
		}

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
			log.Printf("WARNING: Couldn't retrieve any message history for room %s", roomID)
		}
	}

	var filteredEvents []*event.Event

	log.Printf("Processing %d events from room %s", len(resp.Chunk), roomID)

	for _, evt := range resp.Chunk {
		log.Printf("Event type: %v, sender: %s", evt.Type, evt.Sender)

		if evt.Type != event.EventMessage {
			continue
		}

		contentBytes, _ := evt.Content.MarshalJSON()
		log.Printf("Event content: %s", string(contentBytes))

		content, ok := evt.Content.Parsed.(*event.MessageEventContent)
		if !ok {
			log.Printf("Failed to parse message content for event %s", evt.ID)
			if msgType, hasType := evt.Content.Raw["msgtype"].(string); hasType && msgType == "m.text" {
				if body, hasBody := evt.Content.Raw["body"].(string); hasBody && body != "" {
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

		if content == nil || content.Body == "" {
			continue
		}

		filteredEvents = append(filteredEvents, evt)

		if len(filteredEvents) >= limit {
			break
		}
	}

	log.Printf(
		"Retrieved %d messages (after filtering) for context from room %s",
		len(filteredEvents), roomID,
	)

	if len(filteredEvents) == 0 {
		log.Printf("WARNING: Failed to retrieve any usable messages from room %s", roomID)
	}

	return filteredEvents, nil
}

func (c *Client) IsFromAllowedDomain(userID string) bool {
	parts := strings.Split(userID, ":")
	if len(parts) != 2 {
		return false
	}
	return parts[1] == c.allowedDomain
}

func (c *Client) IsAddressedToBot(content string, roomType domain.RoomType) bool {
	if roomType == domain.DirectRoom {
		return true
	}

	fullUsername := strings.TrimPrefix(string(c.userID), "@")

	localpart := fullUsername
	if idx := strings.Index(fullUsername, ":"); idx >= 0 {
		localpart = fullUsername[:idx]
	}

	fullMention := fmt.Sprintf("@%s", fullUsername)

	content = strings.ToLower(content)
	localpart = strings.ToLower(localpart)

	if strings.Contains(content, fullMention) {
		return true
	}

	if strings.Contains(content, localpart) {
		words := strings.Fields(content)
		for _, word := range words {
			word = strings.ToLower(strings.Trim(word, ",.!?:;\"'()[]{}"))

			if word == localpart {
				return true
			}

			if strings.HasPrefix(word, localpart) && len(word) > len(localpart) {
				nextChar := word[len(localpart)]
				if nextChar == ',' || nextChar == '.' || nextChar == '!' ||
					nextChar == '?' || nextChar == ':' || nextChar == ';' {
					return true
				}
			}
		}
	}

	colonCheck := localpart + ":"
	return strings.Contains(content, colonCheck)
}

func (c *Client) GetBotUserID() string {
	return string(c.userID)
}

func (c *Client) GetMatrixClient() *mautrix.Client {
	return c.client
}

func (c *Client) SendTyping(roomID string, typing bool, timeout int) error {
	timeoutDuration := time.Duration(timeout) * time.Millisecond
	_, err := c.client.UserTyping(id.RoomID(roomID), typing, timeoutDuration)
	return err
}
