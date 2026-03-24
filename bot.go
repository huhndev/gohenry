package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"maunium.net/go/mautrix/event"

	"github.com/huhndev/gohenry/chat"
	"github.com/huhndev/gohenry/config"
	"github.com/huhndev/gohenry/domain"
	"github.com/huhndev/gohenry/room"
)

// Bot represents the main application
type Bot struct {
	config         *config.Config
	matrixService  domain.MatrixService
	aiService      domain.AIService
	messageHandler *chat.MessageHandler
	joinService    *room.JoinService
	inviteService  *room.InviteService
}

func NewBot(
	cfg *config.Config,
	matrixService domain.MatrixService,
	aiService domain.AIService,
) *Bot {
	messageHandler := chat.NewMessageHandler(cfg, matrixService, aiService)
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

func (b *Bot) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	if err := b.matrixService.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Matrix: %v", err)
	}

	b.matrixService.SetMessageHandler(
		func(ctx context.Context, evt *event.Event) {
			content, ok := evt.Content.Parsed.(*event.MessageEventContent)
			if !ok || content.Body == "" {
				return
			}
			if err := b.messageHandler.HandleMessage(ctx, string(evt.Sender), string(evt.RoomID), content.Body); err != nil {
				log.Printf("Error handling message: %v", err)
			}
		},
	)

	if err := b.matrixService.CheckAndJoinInvitedRooms(ctx); err != nil {
		log.Printf("Error checking for invited rooms: %v", err)
	}

	go func() {
		log.Printf("Starting Matrix event listener...")
		if err := b.matrixService.ListenForMessages(ctx); err != nil {
			log.Printf("Error in message listener: %v", err)
			cancel()
		}
	}()

	log.Printf("Henry is now running and will auto-accept room invitations. Press Ctrl+C to exit.")
	log.Printf("Bot userID: %s", b.matrixService.GetBotUserID())

	go func() {
		<-shutdown
		fmt.Println("\nShutting down gracefully...")

		shutdownTimeout := time.NewTimer(10 * time.Second)

		shutdownDone := make(chan bool)
		go func() {
			cancel()
			if err := b.matrixService.Disconnect(); err != nil {
				log.Printf("Error during disconnect: %v", err)
			}
			shutdownDone <- true
		}()

		select {
		case <-shutdownDone:
			log.Println("Henry has left the building gracefully.")
		case <-shutdownTimeout.C:
			log.Println("Shutdown timed out, forcing exit.")
		}

		os.Exit(0)
	}()

	<-ctx.Done()
	return ctx.Err()
}

func (b *Bot) JoinRoom(ctx context.Context, roomID string) error {
	if err := b.matrixService.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Matrix: %v", err)
	}
	defer b.matrixService.Disconnect()
	return b.joinService.JoinRoom(ctx, roomID)
}

func (b *Bot) InviteUser(
	ctx context.Context,
	userID string,
	roomID string,
	createRoom bool,
) error {
	if err := b.matrixService.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Matrix: %v", err)
	}
	defer b.matrixService.Disconnect()
	_, err := b.inviteService.InviteUser(ctx, userID, roomID, createRoom)
	return err
}

func (b *Bot) RunDebug(ctx context.Context) error {
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
			return b.config.ClaudeAPIKey[:8] + "..."
		}
		return "not set"
	}())

	log.Printf("Attempting to connect to Matrix homeserver at %s...", b.config.MatrixHomeserver)
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

	log.Printf("Bot user ID: %s", b.matrixService.GetBotUserID())

	log.Printf("Checking for invited rooms...")
	if err := b.matrixService.CheckAndJoinInvitedRooms(ctx); err != nil {
		log.Printf("Error checking for invited rooms: %v", err)
	} else {
		log.Printf("Successfully checked for invited rooms")
	}

	log.Printf("\nDebugging complete. If you're still having issues:")
	log.Printf("1. Try joining a room directly with: ./gohenry join \"!roomid:matrix.org\"")
	log.Printf("2. Try creating a room and inviting Henry with: ./gohenry invite \"@user:matrix.org\"")
	log.Printf("3. Check the Matrix server logs if possible")
	log.Printf("4. Make sure your Matrix server allows bot accounts and auto-join")

	return nil
}
