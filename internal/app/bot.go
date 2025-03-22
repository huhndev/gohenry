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

package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"maunium.net/go/mautrix/event"

	"github.com/huhnsystems/gohenry/internal/config"
	"github.com/huhnsystems/gohenry/internal/ports/services"
	"github.com/huhnsystems/gohenry/internal/usecases/chat"
	"github.com/huhnsystems/gohenry/internal/usecases/room"
)

// Bot represents the main application
type Bot struct {
	config         *config.Config
	matrixService  services.MatrixService
	aiService      services.AIService
	messageHandler *chat.MessageHandler
	joinService    *room.JoinService
	inviteService  *room.InviteService
	// No longer tracking message history locally - always fetching from homeserver
}

// NewBot creates a new bot instance
func NewBot(
	cfg *config.Config,
	matrixService services.MatrixService,
	aiService services.AIService,
) *Bot {
	// Pass nil for message history (parameter kept for backward compatibility)
	messageHandler := chat.NewMessageHandler(cfg, matrixService, aiService, nil)
	joinService := room.NewJoinService(matrixService, cfg)
	inviteService := room.NewInviteService(matrixService, cfg)

	return &Bot{
		config:         cfg,
		matrixService:  matrixService,
		aiService:      aiService,
		messageHandler: messageHandler,
		joinService:    joinService,
		inviteService:  inviteService,
	}
}

// Run starts the bot in normal mode
func (b *Bot) Run(ctx context.Context) error {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Connect to Matrix
	if err := b.matrixService.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Matrix: %v", err)
	}

	// Setup message handler
	b.matrixService.SetMessageHandler(
		func(ctx context.Context, evt *event.Event) {
			// Parse message content
			content, ok := evt.Content.Parsed.(*event.MessageEventContent)
			if !ok || content.Body == "" {
				return
			}

			if err := b.messageHandler.HandleMessage(ctx, string(evt.Sender), string(evt.RoomID), content.Body); err != nil {
				log.Printf("Error handling message: %v", err)
			}
		},
	)

	// Check for and join any existing invite rooms
	if err := b.matrixService.CheckAndJoinInvitedRooms(ctx); err != nil {
		log.Printf("Error checking for invited rooms: %v", err)
	}

	// Start listening for messages
	go func() {
		log.Printf("Starting Matrix event listener...")
		if err := b.matrixService.ListenForMessages(ctx); err != nil {
			log.Printf("Error in message listener: %v", err)
			cancel()
		}
	}()

	log.Printf(
		"Henry is now running and will auto-accept room invitations. Press Ctrl+C to exit.",
	)
	log.Printf("Bot userID: %s", b.matrixService.GetBotUserID())

	// Wait for shutdown signal with timeout
	go func() {
		<-shutdown
		fmt.Println("\nShutting down gracefully...")

		// Set a timeout for graceful shutdown
		shutdownTimeout := time.NewTimer(10 * time.Second)

		shutdownDone := make(chan bool)
		go func() {
			// Cancel context and perform cleanup
			cancel()
			if err := b.matrixService.Disconnect(); err != nil {
				log.Printf("Error during disconnect: %v", err)
			}
			shutdownDone <- true
		}()

		// Wait for either cleanup to finish or timeout
		select {
		case <-shutdownDone:
			log.Println("Henry has left the building gracefully.")
		case <-shutdownTimeout.C:
			log.Println("Shutdown timed out, forcing exit.")
		}

		// Forcibly exit after cleanup or timeout
		os.Exit(0)
	}()

	// Block until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

// JoinRoom performs a manual room join
func (b *Bot) JoinRoom(ctx context.Context, roomID string) error {
	// Connect to Matrix
	if err := b.matrixService.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Matrix: %v", err)
	}
	defer b.matrixService.Disconnect()

	// Join the room
	return b.joinService.JoinRoom(ctx, roomID)
}

// InviteUser invites a user to a room
func (b *Bot) InviteUser(
	ctx context.Context,
	userID string,
	roomID string,
	createRoom bool,
) error {
	// Connect to Matrix
	if err := b.matrixService.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Matrix: %v", err)
	}
	defer b.matrixService.Disconnect()

	// Invite the user
	_, err := b.inviteService.InviteUser(ctx, userID, roomID, createRoom)
	return err
}

// RunDebug performs diagnostics
func (b *Bot) RunDebug(ctx context.Context) error {
	// Print initial configuration before connecting
	log.Printf("Bot diagnostic information:")
	log.Printf("  Configuration:")
	log.Printf("    Matrix homeserver: %s", b.config.MatrixHomeserver)
	log.Printf("    Matrix user ID: %s", b.config.MatrixUserID)
	log.Printf("    Authentication: Using %s", func() string {
		if b.config.MatrixAccessToken != "" {
			return "access token"
		}
		return "password"
	}())
	log.Printf("    Allowed domain: %s", b.config.AllowedDomain)
	log.Printf("    Context message count: %d", b.config.ContextMessageCount)
	log.Printf("    Claude API Key: %s", func() string {
		if b.config.ClaudeAPIKey != "" {
			return b.config.ClaudeAPIKey[:8] + "..." // Show only first 8 characters
		}
		return "not set"
	}())

	// Connect to Matrix with detailed logging
	log.Printf(
		"Attempting to connect to Matrix homeserver at %s...",
		b.config.MatrixHomeserver,
	)
	if err := b.matrixService.Connect(ctx); err != nil {
		log.Printf("ERROR: Failed to connect to Matrix: %v", err)
		return fmt.Errorf("failed to connect to Matrix: %v", err)
	}
	log.Printf("Successfully connected to Matrix homeserver")
	defer func() {
		log.Printf("Disconnecting from Matrix...")
		if err := b.matrixService.Disconnect(); err != nil {
			log.Printf("Error during disconnect: %v", err)
		} else {
			log.Printf("Successfully disconnected")
		}
	}()

	// Get bot user ID
	log.Printf("Bot user ID: %s", b.matrixService.GetBotUserID())

	// Check for and join any existing invite rooms
	log.Printf("Checking for invited rooms...")
	if err := b.matrixService.CheckAndJoinInvitedRooms(ctx); err != nil {
		log.Printf("Error checking for invited rooms: %v", err)
	} else {
		log.Printf("Successfully checked for invited rooms")
	}

	log.Printf("\nDebugging complete. If you're still having issues:")
	log.Printf(
		"1. Try joining a room directly with: ./gohenry join \"!roomid:matrix.org\"",
	)
	log.Printf(
		"2. Try creating a room and inviting Henry with: ./gohenry invite \"@user:matrix.org\"",
	)
	log.Printf("3. Check the Matrix server logs if possible")
	log.Printf(
		"4. Make sure your Matrix server allows bot accounts and auto-join",
	)

	return nil
}
