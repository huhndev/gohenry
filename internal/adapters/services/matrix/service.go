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
	"log"

	"maunium.net/go/mautrix/event"

	"github.com/huhndev/gohenry/internal/config"
	"github.com/huhndev/gohenry/internal/domain"
)

// Service implements the MatrixService interface
type Service struct {
	client *Client
	config *config.Config
}

// NewService creates a new Matrix service
func NewService(cfg *config.Config) (*Service, error) {
	client := NewClient(cfg)

	return &Service{
		client: client,
		config: cfg,
	}, nil
}

// Connect initializes the Matrix client and connects to the homeserver
func (s *Service) Connect(ctx context.Context) error {
	return s.client.Connect(ctx)
}

// Disconnect logs out from the Matrix server
func (s *Service) Disconnect() error {
	return s.client.Disconnect()
}

// SetMessageHandler sets the function to handle incoming messages
func (s *Service) SetMessageHandler(
	handler func(ctx context.Context, evt *event.Event),
) {
	s.client.SetMessageHandler(handler)
}

// ListenForMessages starts listening for messages
func (s *Service) ListenForMessages(ctx context.Context) error {
	return s.client.ListenForMessages(ctx)
}

// JoinRoom attempts to join a Matrix room
func (s *Service) JoinRoom(roomID string) error {
	return s.client.JoinRoom(roomID)
}

// SendMessage sends a text message to a Matrix room
func (s *Service) SendMessage(roomID string, content string) error {
	return s.client.SendMessage(roomID, content)
}

// GetRoomContext retrieves past messages from a room for context
func (s *Service) GetRoomContext(
	ctx context.Context,
	roomID string,
	limit int,
) ([]*domain.Message, error) {
	events, err := s.client.GetRoomContext(ctx, roomID, limit)
	if err != nil {
		return nil, err
	}

	// Convert Matrix events to domain messages
	messages := make([]*domain.Message, 0, len(events))
	for _, evt := range events {
		// Skip non-message events
		if evt.Type != event.EventMessage {
			continue
		}

		// Parse content
		content, ok := evt.Content.Parsed.(*event.MessageEventContent)
		if !ok {
			// Try to parse it manually
			log.Printf("Attempting manual parse for event %s", evt.ID)
			if msgType, hasType := evt.Content.Raw["msgtype"].(string); hasType &&
				msgType == "m.text" {
				if body, hasBody := evt.Content.Raw["body"].(string); hasBody &&
					body != "" {
					content = &event.MessageEventContent{
						MsgType: event.MessageType(msgType),
						Body:    body,
					}
					ok = true
					log.Printf("Successfully parsed message manually: %s", body)
				}
			}

			if !ok {
				log.Printf(
					"Failed to parse message content for event %s, skipping",
					evt.ID,
				)
				continue
			}
		}

		// Skip empty messages
		if content == nil || content.Body == "" {
			continue
		}

		// Create domain message
		messages = append(messages, &domain.Message{
			ID:        string(evt.ID),
			RoomID:    string(evt.RoomID),
			SenderID:  string(evt.Sender),
			Content:   content.Body,
			Timestamp: evt.Timestamp,
			IsFromBot: string(evt.Sender) == s.client.GetBotUserID(),
		})
	}

	return messages, nil
}

// GetRoomType determines if a room is a direct message or group chat
func (s *Service) GetRoomType(
	ctx context.Context,
	roomID string,
) (domain.RoomType, error) {
	return s.client.GetRoomType(ctx, roomID)
}

// CheckAndJoinInvitedRooms checks for and joins any invited rooms
func (s *Service) CheckAndJoinInvitedRooms(ctx context.Context) error {
	return s.client.CheckAndJoinInvitedRooms(ctx)
}

// IsFromAllowedDomain checks if a user is from the allowed domain
func (s *Service) IsFromAllowedDomain(userID string) bool {
	return s.client.IsFromAllowedDomain(userID)
}

// IsAddressedToBot checks if a message is addressed to the bot
func (s *Service) IsAddressedToBot(
	content string,
	roomType domain.RoomType,
) bool {
	return s.client.IsAddressedToBot(content, roomType)
}

// CreateRoom creates a new Matrix room
func (s *Service) CreateRoom(
	name string,
	topic string,
	inviteUsers []string,
	isDirect bool,
) (string, error) {
	return s.client.CreateRoom(name, topic, inviteUsers, isDirect)
}

// InviteUser invites a user to a Matrix room
func (s *Service) InviteUser(roomID string, userID string) error {
	return s.client.InviteUser(roomID, userID)
}

// GetBotUserID returns the bot's Matrix user ID
func (s *Service) GetBotUserID() string {
	return s.client.GetBotUserID()
}

// SendTyping sends a typing notification to a Matrix room
func (s *Service) SendTyping(roomID string, typing bool, timeout int) error {
	return s.client.SendTyping(roomID, typing, timeout)
}
