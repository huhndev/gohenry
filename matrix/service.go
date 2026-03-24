package matrix

import (
	"context"
	"log"

	"maunium.net/go/mautrix/event"

	"github.com/huhndev/gohenry/config"
	"github.com/huhndev/gohenry/domain"
)

// Service implements the MatrixService interface
type Service struct {
	client *Client
	config *config.Config
}

func NewService(cfg *config.Config) (*Service, error) {
	client := NewClient(cfg)
	return &Service{
		client: client,
		config: cfg,
	}, nil
}

func (s *Service) Connect(ctx context.Context) error {
	return s.client.Connect(ctx)
}

func (s *Service) Disconnect() error {
	return s.client.Disconnect()
}

func (s *Service) SetMessageHandler(handler func(ctx context.Context, evt *event.Event)) {
	s.client.SetMessageHandler(handler)
}

func (s *Service) ListenForMessages(ctx context.Context) error {
	return s.client.ListenForMessages(ctx)
}

func (s *Service) JoinRoom(roomID string) error {
	return s.client.JoinRoom(roomID)
}

func (s *Service) SendMessage(roomID string, content string) error {
	return s.client.SendMessage(roomID, content)
}

func (s *Service) GetRoomContext(
	ctx context.Context,
	roomID string,
	limit int,
) ([]*domain.Message, error) {
	events, err := s.client.GetRoomContext(ctx, roomID, limit)
	if err != nil {
		return nil, err
	}

	messages := make([]*domain.Message, 0, len(events))
	for _, evt := range events {
		if evt.Type != event.EventMessage {
			continue
		}

		content, ok := evt.Content.Parsed.(*event.MessageEventContent)
		if !ok {
			log.Printf("Attempting manual parse for event %s", evt.ID)
			if msgType, hasType := evt.Content.Raw["msgtype"].(string); hasType && msgType == "m.text" {
				if body, hasBody := evt.Content.Raw["body"].(string); hasBody && body != "" {
					content = &event.MessageEventContent{
						MsgType: event.MessageType(msgType),
						Body:    body,
					}
					ok = true
					log.Printf("Successfully parsed message manually: %s", body)
				}
			}

			if !ok {
				log.Printf("Failed to parse message content for event %s, skipping", evt.ID)
				continue
			}
		}

		if content == nil || content.Body == "" {
			continue
		}

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

func (s *Service) GetRoomType(ctx context.Context, roomID string) (domain.RoomType, error) {
	return s.client.GetRoomType(ctx, roomID)
}

func (s *Service) CheckAndJoinInvitedRooms(ctx context.Context) error {
	return s.client.CheckAndJoinInvitedRooms(ctx)
}

func (s *Service) IsFromAllowedDomain(userID string) bool {
	return s.client.IsFromAllowedDomain(userID)
}

func (s *Service) IsAddressedToBot(content string, roomType domain.RoomType) bool {
	return s.client.IsAddressedToBot(content, roomType)
}

func (s *Service) CreateRoom(
	name string,
	topic string,
	inviteUsers []string,
	isDirect bool,
) (string, error) {
	return s.client.CreateRoom(name, topic, inviteUsers, isDirect)
}

func (s *Service) InviteUser(roomID string, userID string) error {
	return s.client.InviteUser(roomID, userID)
}

func (s *Service) GetBotUserID() string {
	return s.client.GetBotUserID()
}

func (s *Service) SendTyping(roomID string, typing bool, timeout int) error {
	return s.client.SendTyping(roomID, typing, timeout)
}
