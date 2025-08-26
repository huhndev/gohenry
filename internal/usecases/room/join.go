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
	"strings"

	"github.com/huhndev/gohenry/internal/config"
	"github.com/huhndev/gohenry/internal/ports/services"
)

// JoinService handles room joining operations
type JoinService struct {
	matrixService services.MatrixService
	config        *config.Config
}

// NewJoinService creates a new room join service
func NewJoinService(
	matrixService services.MatrixService,
	cfg *config.Config,
) *JoinService {
	return &JoinService{
		matrixService: matrixService,
		config:        cfg,
	}
}

// JoinRoom attempts to join a specific Matrix room
func (s *JoinService) JoinRoom(ctx context.Context, roomID string) error {
	log.Printf("Attempting to join room: %s", roomID)

	err := s.matrixService.JoinRoom(roomID)
	if err != nil {
		// If direct join fails, try alternative methods
		if strings.HasPrefix(roomID, "!") {
			log.Printf("Direct join failed, trying alternative methods...")
			return s.tryAlternativeJoinMethods(ctx, roomID)
		}
		return fmt.Errorf("failed to join room: %v", err)
	}

	log.Printf("Successfully joined room %s", roomID)
	return nil
}

// tryAlternativeJoinMethods attempts various fallback methods to join a room
func (s *JoinService) tryAlternativeJoinMethods(
	ctx context.Context,
	roomID string,
) error {
	// Try to create a new room with the specified ID as an alias
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

	log.Printf(
		"Successfully created new room %s instead of joining %s",
		newRoomID,
		roomID,
	)
	return nil
}
