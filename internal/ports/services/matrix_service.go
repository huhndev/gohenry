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

package services

import (
	"context"

	"maunium.net/go/mautrix/event"

	"github.com/huhndev/gohenry/internal/domain"
)

// MatrixService defines the interface for Matrix interactions
type MatrixService interface {
	// Connect initializes the Matrix client and connects to the homeserver
	Connect(ctx context.Context) error

	// Disconnect logs out from the Matrix server
	Disconnect() error

	// SetMessageHandler sets the function to handle incoming messages
	SetMessageHandler(handler func(ctx context.Context, evt *event.Event))

	// ListenForMessages starts listening for messages
	ListenForMessages(ctx context.Context) error

	// JoinRoom attempts to join a Matrix room
	JoinRoom(roomID string) error

	// SendMessage sends a text message to a Matrix room
	SendMessage(roomID string, content string) error

	// GetRoomContext retrieves past messages from a room for context
	GetRoomContext(
		ctx context.Context,
		roomID string,
		limit int,
	) ([]*domain.Message, error)

	// GetRoomType determines if a room is a direct message or group chat
	GetRoomType(ctx context.Context, roomID string) (domain.RoomType, error)

	// CheckAndJoinInvitedRooms checks for and joins any invited rooms
	CheckAndJoinInvitedRooms(ctx context.Context) error

	// IsFromAllowedDomain checks if a user is from the allowed domain
	IsFromAllowedDomain(userID string) bool

	// IsAddressedToBot checks if a message is addressed to the bot
	IsAddressedToBot(content string, roomType domain.RoomType) bool

	// CreateRoom creates a new Matrix room
	CreateRoom(
		name string,
		topic string,
		inviteUsers []string,
		isDirect bool,
	) (string, error)

	// InviteUser invites a user to a Matrix room
	InviteUser(roomID string, userID string) error

	// GetBotUserID returns the bot's Matrix user ID
	GetBotUserID() string

	// SendTyping sends a typing notification to a Matrix room
	SendTyping(roomID string, typing bool, timeout int) error
}
