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

	"github.com/huhndev/gohenry/domain"
)

func (c *Client) GetRoomType(
	ctx context.Context,
	roomID string,
) (domain.RoomType, error) {
	stateEvents, err := c.client.State(id.RoomID(roomID))
	if err != nil {
		return "", err
	}

	joinedMembers := 0
	for _, evts := range stateEvents {
		for _, evt := range evts {
			if evt.Type == event.StateMember {
				member := evt.Content.AsMember()
				if member != nil && member.Membership == event.MembershipJoin {
					joinedMembers++
				}
			}
		}
	}

	if joinedMembers == 2 {
		return domain.DirectRoom, nil
	}
	return domain.GroupRoom, nil
}

func (c *Client) CheckAndJoinInvitedRooms(ctx context.Context) error {
	log.Printf("Checking for existing room invitations...")

	joinedRooms, err := c.client.JoinedRooms()
	if err != nil {
		log.Printf("Failed to get joined rooms: %v", err)
	} else {
		log.Printf("Currently joined rooms: %d", len(joinedRooms.JoinedRooms))
		for _, room := range joinedRooms.JoinedRooms {
			log.Printf("  Already in room: %s", room)
		}
	}

	log.Printf("Bot user ID: %s", c.userID)
	log.Printf("When inviting the bot, please use this exact ID")

	go c.periodicRoomCheck(ctx)

	log.Printf("Invites will be automatically accepted when detected during sync")
	log.Printf("Also running periodic room checks every 10 seconds")

	return nil
}

func (c *Client) periodicRoomCheck(ctx context.Context) {
	time.Sleep(5 * time.Second)

	var ownerID string
	if c.config.OwnerID != "" {
		ownerID = c.config.OwnerID
	} else {
		parts := strings.Split(string(c.userID), ":")
		if len(parts) == 2 {
			ownerID = "@user:" + parts[1]
		} else {
			ownerID = "@user:matrix.org"
		}
	}
	log.Printf("Using owner ID: %s", ownerID)
	c.sendFeedbackToOwner(
		ownerID,
		fmt.Sprintf("Henry bot started. UserID: %s", c.userID),
	)

	attemptCounter := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
			attemptCounter++
			log.Printf("Performing periodic room check (attempt #%d)...", attemptCounter)

			joinedRooms, err := c.client.JoinedRooms()
			if err != nil {
				log.Printf("Error getting joined rooms: %v", err)
			} else if len(joinedRooms.JoinedRooms) > 0 {
				status := fmt.Sprintf("Currently in %d rooms", len(joinedRooms.JoinedRooms))
				log.Printf(status)

				if attemptCounter%10 == 0 {
					c.sendFeedbackToOwner(ownerID, status)
				}
			}

			time.Sleep(10 * time.Second)
		}
	}
}

func (c *Client) sendFeedbackToOwner(ownerID string, message string) {
	log.Printf("%s", message)
}

func (c *Client) CreateRoom(
	name string,
	topic string,
	inviteUsers []string,
	isDirect bool,
) (string, error) {
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

func (c *Client) InviteUser(roomID string, userID string) error {
	_, err := c.client.InviteUser(id.RoomID(roomID), &mautrix.ReqInviteUser{
		UserID: id.UserID(userID),
	})
	if err != nil {
		return fmt.Errorf("failed to invite user: %v", err)
	}
	return nil
}
