package room

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/huhndev/gohenry/config"
	"github.com/huhndev/gohenry/domain"
)

// JoinService handles room joining operations
type JoinService struct {
	matrixService domain.MatrixService
	config        *config.Config
}

func NewJoinService(matrixService domain.MatrixService, cfg *config.Config) *JoinService {
	return &JoinService{
		matrixService: matrixService,
		config:        cfg,
	}
}

func (s *JoinService) JoinRoom(ctx context.Context, roomID string) error {
	log.Printf("Attempting to join room: %s", roomID)

	err := s.matrixService.JoinRoom(roomID)
	if err != nil {
		if strings.HasPrefix(roomID, "!") {
			log.Printf("Direct join failed, trying alternative methods...")
			return s.tryAlternativeJoinMethods(ctx, roomID)
		}
		return fmt.Errorf("failed to join room: %v", err)
	}

	log.Printf("Successfully joined room %s", roomID)
	return nil
}

func (s *JoinService) tryAlternativeJoinMethods(ctx context.Context, roomID string) error {
	log.Printf("Trying to create a direct room...")
	newRoomID, err := s.matrixService.CreateRoom(
		"New Direct Chat",
		"Chat created by join command",
		[]string{},
		true,
	)
	if err != nil {
		return fmt.Errorf("all join methods failed: %v", err)
	}

	log.Printf("Successfully created new room %s instead of joining %s", newRoomID, roomID)
	return nil
}
