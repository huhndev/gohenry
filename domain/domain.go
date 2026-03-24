package domain

import (
	"context"

	"maunium.net/go/mautrix/event"
)

// Message represents a chat message in the system
type Message struct {
	ID        string
	RoomID    string
	SenderID  string
	Content   string
	Timestamp int64
	IsFromBot bool
}

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

// ConversationMessage represents a message in a conversation with Claude
type ConversationMessage struct {
	Role      MessageRole
	Content   string
	Timestamp int64
	SenderID  string
}

type RoomType string

const (
	DirectRoom RoomType = "direct"
	GroupRoom  RoomType = "group"
)

// Room represents a chat room
type Room struct {
	ID      string
	Type    RoomType
	Name    string
	Members []string
}

// AIService defines the interface for AI (Claude) interactions
type AIService interface {
	GenerateResponse(ctx context.Context, messages []ConversationMessage) (string, error)
}

// MatrixService defines the interface for Matrix interactions
type MatrixService interface {
	Connect(ctx context.Context) error
	Disconnect() error
	SetMessageHandler(handler func(ctx context.Context, evt *event.Event))
	ListenForMessages(ctx context.Context) error
	JoinRoom(roomID string) error
	SendMessage(roomID string, content string) error
	GetRoomContext(ctx context.Context, roomID string, limit int) ([]*Message, error)
	GetRoomType(ctx context.Context, roomID string) (RoomType, error)
	CheckAndJoinInvitedRooms(ctx context.Context) error
	IsFromAllowedDomain(userID string) bool
	IsAddressedToBot(content string, roomType RoomType) bool
	CreateRoom(name string, topic string, inviteUsers []string, isDirect bool) (string, error)
	InviteUser(roomID string, userID string) error
	GetBotUserID() string
	SendTyping(roomID string, typing bool, timeout int) error
}
