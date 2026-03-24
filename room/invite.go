package room

import (
	"context"
	"fmt"
	"log"

	"github.com/huhndev/gohenry/config"
	"github.com/huhndev/gohenry/domain"
)

// InviteService handles room invitation operations
type InviteService struct {
	matrixService domain.MatrixService
	config        *config.Config
}

func NewInviteService(matrixService domain.MatrixService, cfg *config.Config) *InviteService {
	return &InviteService{
		matrixService: matrixService,
		config:        cfg,
	}
}

func (s *InviteService) InviteUser(
	ctx context.Context,
	userID string,
	roomID string,
	createRoom bool,
) (string, error) {
	var targetRoomID string

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

		log.Printf("Successfully created room %s and invited %s", targetRoomID, userID)
	} else {
		log.Printf("Using existing room %s", roomID)
		targetRoomID = roomID

		log.Printf("Inviting %s to room %s", userID, targetRoomID)
		if err := s.matrixService.InviteUser(targetRoomID, userID); err != nil {
			return "", fmt.Errorf("failed to invite user: %v", err)
		}

		log.Printf("Successfully invited %s to room %s", userID, targetRoomID)
	}

	welcomeMsg := fmt.Sprintf("Hello %s! This is a test message.", userID)
	if err := s.matrixService.SendMessage(targetRoomID, welcomeMsg); err != nil {
		log.Printf("Failed to send welcome message: %v", err)
	} else {
		log.Printf("Sent welcome message")
	}

	return targetRoomID, nil
}
