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
	"log"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/huhndev/gohenry/internal/domain"
)

// GetRoomType determines if a room is a direct message or group chat
func (c *Client) GetRoomType(
	ctx context.Context,
	roomID string,
) (domain.RoomType, error) {
	stateEvents, err := c.client.State(id.RoomID(roomID))
	if err != nil {
		return "", err
	}

	// Count the number of joined members
	joinedMembers := 0

	// Handle state events - the structure may be different in this version
	for _, evts := range stateEvents {
		for _, evt := range evts {
			// Check membership events
			if evt.Type == event.StateMember {
				member := evt.Content.AsMember()
				if member != nil && member.Membership == event.MembershipJoin {
					joinedMembers++
				}
			}
		}
	}

	// If there are only 2 members (the bot and one user), it's a direct message
	if joinedMembers == 2 {
		return domain.DirectRoom, nil
	}
	return domain.GroupRoom, nil
}

// CheckAndJoinInvitedRooms checks for and joins any rooms the bot has been invited to
func (c *Client) CheckAndJoinInvitedRooms(ctx context.Context) error {
	log.Printf("Checking for existing room invitations...")

	// Get all joined rooms first (for logging purposes)
	joinedRooms, err := c.client.JoinedRooms()
	if err != nil {
		log.Printf("Failed to get joined rooms: %v", err)
	} else {
		log.Printf("Currently joined rooms: %d", len(joinedRooms.JoinedRooms))
		for _, room := range joinedRooms.JoinedRooms {
			log.Printf("  Already in room: %s", room)
		}
	}

	// Log the bot user ID for debugging
	log.Printf("Bot user ID: %s", c.userID)
	log.Printf("When inviting the bot, please use this exact ID")

	// Setup periodic room check for invites
	go c.periodicRoomCheck(ctx)

	log.Printf(
		"Invites will be automatically accepted when detected during sync",
	)
	log.Printf("Also running periodic room checks every 10 seconds")

	return nil
}

// periodicRoomCheck periodically checks for and joins invited rooms
func (c *Client) periodicRoomCheck(ctx context.Context) {
	// Wait a bit before starting to give the initial sync time to complete
	time.Sleep(5 * time.Second)

	// Send status message to owner - try to get the owner ID from environment
	var ownerID string
	if c.config.OwnerID != "" {
		ownerID = c.config.OwnerID
	} else {
		// Default to a username based on the bot's domain
		parts := strings.Split(string(c.userID), ":")
		if len(parts) == 2 {
			ownerID = "@user:" + parts[1]
		} else {
			ownerID = "@user:matrix.org" // Fallback
		}
	}
	log.Printf("Using owner ID: %s", ownerID)
	c.sendFeedbackToOwner(
		ownerID,
		fmt.Sprintf("Henry bot started. UserID: %s", c.userID),
	)

	// Create a counter for failed attempts to help debug issues
	attemptCounter := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
			attemptCounter++
			log.Printf(
				"Performing periodic room check (attempt #%d)...",
				attemptCounter,
			)

			// Get joined rooms for status reporting
			joinedRooms, err := c.client.JoinedRooms()
			if err != nil {
				log.Printf("Error getting joined rooms: %v", err)
			} else if len(joinedRooms.JoinedRooms) > 0 {
				status := fmt.Sprintf("Currently in %d rooms", len(joinedRooms.JoinedRooms))
				log.Printf(status)

				// Send a status update every 10 attempts
				if attemptCounter%10 == 0 {
					c.sendFeedbackToOwner(ownerID, status)
				}
			}

			// Automatic room creation has been removed - Henry should not create rooms automatically

			time.Sleep(10 * time.Second)
		}
	}
}

// sendFeedbackToOwner logs feedback without creating rooms
func (c *Client) sendFeedbackToOwner(ownerID string, message string) {
	// Just log the message without mentioning who it's for
	log.Printf("%s", message)
}

// CreateRoom creates a new Matrix room
func (c *Client) CreateRoom(
	name string,
	topic string,
	inviteUsers []string,
	isDirect bool,
) (string, error) {
	// Convert string IDs to Matrix user IDs
	invites := make([]id.UserID, len(inviteUsers))
	for i, user := range inviteUsers {
		invites[i] = id.UserID(user)
	}

	resp, err := c.client.CreateRoom(&mautrix.ReqCreateRoom{
		Preset:   "private_chat",
		Name:     name,
		Topic:    topic,
		Invite:   invites,
		IsDirect: isDirect,
	})

	if err != nil {
		return "", fmt.Errorf("failed to create room: %v", err)
	}

	return string(resp.RoomID), nil
}

// InviteUser invites a user to a Matrix room
func (c *Client) InviteUser(roomID string, userID string) error {
	_, err := c.client.InviteUser(id.RoomID(roomID), &mautrix.ReqInviteUser{
		UserID: id.UserID(userID),
	})

	if err != nil {
		return fmt.Errorf("failed to invite user: %v", err)
	}

	return nil
}
