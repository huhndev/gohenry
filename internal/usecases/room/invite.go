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

package room

import (
	"context"
	"fmt"
	"log"

	"github.com/huhndev/gohenry/internal/config"
	"github.com/huhndev/gohenry/internal/ports/services"
)

// InviteService handles room invitation operations
type InviteService struct {
	matrixService services.MatrixService
	config        *config.Config
}

// NewInviteService creates a new room invite service
func NewInviteService(
	matrixService services.MatrixService,
	cfg *config.Config,
) *InviteService {
	return &InviteService{
		matrixService: matrixService,
		config:        cfg,
	}
}

// InviteUser invites a user to an existing room or creates a new room and invites them
func (s *InviteService) InviteUser(
	ctx context.Context,
	userID string,
	roomID string,
	createRoom bool,
) (string, error) {
	var targetRoomID string

	// Either create a new room or use existing
	if createRoom || roomID == "" {
		log.Printf("Creating new room and inviting %s", userID)

		var err error
		targetRoomID, err = s.matrixService.CreateRoom(
			"Test Room for Bot",
			"This room is for testing a Matrix bot",
			[]string{userID},
			true,
		)

		if err != nil {
			return "", fmt.Errorf("failed to create room: %v", err)
		}

		log.Printf(
			"Successfully created room %s and invited %s",
			targetRoomID,
			userID,
		)
	} else {
		log.Printf("Using existing room %s", roomID)
		targetRoomID = roomID

		// Invite the user to the room
		log.Printf("Inviting %s to room %s", userID, targetRoomID)
		if err := s.matrixService.InviteUser(targetRoomID, userID); err != nil {
			return "", fmt.Errorf("failed to invite user: %v", err)
		}

		log.Printf("Successfully invited %s to room %s", userID, targetRoomID)
	}

	// Send a welcome message
	welcomeMsg := fmt.Sprintf("Hello %s! This is a test message.", userID)
	if err := s.matrixService.SendMessage(targetRoomID, welcomeMsg); err != nil {
		log.Printf("Failed to send welcome message: %v", err)
	} else {
		log.Printf("Sent welcome message")
	}

	return targetRoomID, nil
}
