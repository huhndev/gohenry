package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/huhndev/gohenry/claude"
	"github.com/huhndev/gohenry/config"
	"github.com/huhndev/gohenry/matrix"
)

func main() {
	log.SetOutput(os.Stdout)
	log.Println("Starting Henry, the Claude-powered Matrix bot...")

	joinCmd := flag.NewFlagSet("join", flag.ExitOnError)
	joinRoomID := joinCmd.String("room", "", "Room ID or alias to join")

	debugCmd := flag.NewFlagSet("debug", flag.ExitOnError)

	inviteCmd := flag.NewFlagSet("invite", flag.ExitOnError)
	inviteUserID := inviteCmd.String("user", "", "User ID to invite")
	inviteRoomID := inviteCmd.String("room", "", "Room ID to invite user to")
	createRoom := inviteCmd.Bool("create", false, "Create a new room and invite the user")

	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("GoHenry - Claude-powered Matrix chatbot")
		fmt.Println("\nUsage:")
		fmt.Println("  ./gohenry                           Run the bot in normal mode")
		fmt.Println("  ./gohenry join <room_id>            Join a specific Matrix room")
		fmt.Println("  ./gohenry debug                     Show connection status and debugging info")
		fmt.Println("  ./gohenry invite <user_id>          Create a room and invite a user")
		fmt.Println("  ./gohenry invite -user <user_id> -room <room_id> [-create]  Invite a user to a room")
		fmt.Println("\nFor more information, see README.md")
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	matrixService, err := matrix.NewService(cfg)
	if err != nil {
		log.Fatalf("Failed to create Matrix service: %v", err)
	}

	aiService, err := claude.NewService(cfg.ClaudeAPIKey)
	if err != nil {
		log.Fatalf("Failed to create Claude service: %v", err)
	}

	bot := NewBot(cfg, matrixService, aiService)

	ctx := context.Background()

	if len(os.Args) < 2 {
		if err := bot.Run(ctx); err != nil {
			log.Fatalf("Error running bot: %v", err)
		}
		return
	}

	switch os.Args[1] {
	case "join":
		joinCmd.Parse(os.Args[2:])
		if *joinRoomID == "" && joinCmd.NArg() > 0 {
			*joinRoomID = joinCmd.Arg(0)
		}
		if *joinRoomID == "" {
			log.Fatalf("Room ID or alias is required for join command")
		}
		if err := bot.JoinRoom(ctx, *joinRoomID); err != nil {
			log.Fatalf("Failed to join room: %v", err)
		}

	case "debug":
		debugCmd.Parse(os.Args[2:])
		if err := bot.RunDebug(ctx); err != nil {
			log.Fatalf("Debug error: %v", err)
		}

	case "invite":
		inviteCmd.Parse(os.Args[2:])
		if *inviteUserID == "" && inviteCmd.NArg() > 0 {
			*inviteUserID = inviteCmd.Arg(0)
		}
		if *inviteUserID == "" {
			log.Fatalf("User ID to invite is required")
		}
		if *inviteRoomID == "" && inviteCmd.NArg() > 1 {
			*inviteRoomID = inviteCmd.Arg(1)
		}
		if err := bot.InviteUser(ctx, *inviteUserID, *inviteRoomID, *createRoom); err != nil {
			log.Fatalf("Failed to invite user: %v", err)
		}

	default:
		if err := bot.Run(ctx); err != nil {
			log.Fatalf("Error running bot: %v", err)
		}
	}
}
