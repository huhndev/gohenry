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

package chat

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/huhndev/gohenry/internal/config"
	"github.com/huhndev/gohenry/internal/domain"
	"github.com/huhndev/gohenry/internal/ports/services"
)

// MessageHandler processes incoming messages and decides how to respond
type MessageHandler struct {
	config        *config.Config
	matrixService services.MatrixService
	aiService     services.AIService
	// No longer storing message history locally - always fetching from homeserver
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(
	cfg *config.Config,
	matrixService services.MatrixService,
	aiService services.AIService,
	_ map[string][]*domain.Message, // Keeping parameter for backward compatibility
) *MessageHandler {
	return &MessageHandler{
		config:        cfg,
		matrixService: matrixService,
		aiService:     aiService,
		// No longer using message history
	}
}

// HandleMessage processes an incoming Matrix message
func (h *MessageHandler) HandleMessage(
	ctx context.Context,
	senderID, roomID, content string,
) error {
	// Skip empty messages
	if content == "" {
		return nil
	}

	// Check if message is from an allowed domain
	if !h.matrixService.IsFromAllowedDomain(senderID) {
		log.Printf("Ignoring message from non-allowed domain: %s", senderID)
		return nil
	}

	// Get room type
	roomType, err := h.matrixService.GetRoomType(ctx, roomID)
	if err != nil {
		return fmt.Errorf("error determining room type: %v", err)
	}

	// Check if message is addressed to bot (always true for direct messages)
	if !h.matrixService.IsAddressedToBot(content, roomType) {
		return nil
	}

	// Remove bot mention from message in group chats
	messageText := content
	if roomType == domain.GroupRoom {
		// Get the full bot ID without @ prefix (e.g., "henry:henhouse.im")
		fullUsername := strings.TrimPrefix(h.matrixService.GetBotUserID(), "@")

		// Get the local part of the username (e.g., "henry" from "henry:henhouse.im")
		localpart := fullUsername
		if idx := strings.Index(fullUsername, ":"); idx >= 0 {
			localpart = fullUsername[:idx]
		}

		// Remove both full mention and simple name
		fullMention := fmt.Sprintf("@%s", fullUsername) // @henry:henhouse.im

		// First remove the full mention
		messageText = strings.ReplaceAll(messageText, fullMention, "")

		// Then try to remove just the name with word boundaries
		// This is a simplistic approach - a more robust solution would use regex
		messageText = strings.ReplaceAll(messageText, " "+localpart+" ", " ")
		messageText = strings.ReplaceAll(messageText, " "+localpart, "")
		if strings.HasPrefix(messageText, localpart+" ") {
			messageText = messageText[len(localpart)+1:]
		}
		if messageText == localpart {
			messageText = ""
		}
		messageText = strings.TrimSpace(messageText)
	}

	// Skip empty messages after removing mention
	if messageText == "" {
		return nil
	}

	// Get conversation context
	contextMessages, err := h.getConversationContext(
		ctx,
		senderID,
		roomID,
		messageText,
	)
	if err != nil {
		log.Printf("Error getting conversation context: %v", err)
		// Continue with just the current message if there was an error
		contextMessages = []domain.ConversationMessage{
			{
				Role:      domain.RoleUser,
				Content:   messageText,
				Timestamp: time.Now().UnixNano() / 1e6,
				SenderID:  senderID,
			},
		}
	}

	// Send typing indicator before generating response
	// Typical timeout is 30 seconds (30000ms) - should be enough for most responses
	typingTimeout := 30000 // milliseconds
	if err := h.matrixService.SendTyping(roomID, true, typingTimeout); err != nil {
		log.Printf("Error sending typing notification: %v", err)
		// Continue processing even if typing notification fails
	} else {
		log.Printf("Sent typing indicator to room %s", roomID)
	}

	// Generate response using AI service
	log.Printf("Generating response for message in room %s", roomID)
	aiResponse, err := h.aiService.GenerateResponse(ctx, contextMessages)
	if err != nil {
		log.Printf("Error generating response: %v", err)
		// Send error message
		if err := h.matrixService.SendMessage(roomID, "Sorry, I'm having trouble thinking right now."); err != nil {
			log.Printf("Error sending error message: %v", err)
		}
		return fmt.Errorf("error generating response: %v", err)
	}

	// Create a message for the bot's response (no longer storing in history)
	responseTimestamp := time.Now().UnixNano() / 1e6

	// We're logging this for debugging purposes but not storing it
	log.Printf("Generated bot response for room %s at timestamp %d",
		roomID, responseTimestamp)

	// Stop typing indicator before sending response
	if err := h.matrixService.SendTyping(roomID, false, 0); err != nil {
		log.Printf("Error stopping typing notification: %v", err)
		// Continue anyway
	}

	// Send response
	if err := h.matrixService.SendMessage(roomID, aiResponse); err != nil {
		return fmt.Errorf("error sending response: %v", err)
	}

	log.Printf("Response sent to room %s (stored in history)", roomID)
	return nil
}

// getConversationContext retrieves recent messages for context
func (h *MessageHandler) getConversationContext(
	ctx context.Context,
	senderID string,
	roomID string,
	currentMessage string,
) ([]domain.ConversationMessage, error) {
	// Get room type for logging and context size determination
	roomType, err := h.matrixService.GetRoomType(ctx, roomID)
	if err != nil {
		log.Printf(
			"Error getting room type: %v, defaulting to direct message behavior",
			err,
		)
		roomType = domain.DirectRoom // Default to direct message behavior
	}

	var recentMessages []*domain.Message

	// Always fetch fresh history from Matrix for all room types
	log.Printf(
		"Fetching messages from Matrix for room %s (type: %s)",
		roomID,
		roomType,
	)

	// Request exactly the number of messages specified in the config
	// We want exactly HENRY_CONTEXT_MESSAGE_COUNT recent messages
	requestLimit := h.config.ContextMessageCount

	// Get messages directly from Matrix homeserver
	matrixMessages, err := h.matrixService.GetRoomContext(
		ctx,
		roomID,
		requestLimit,
	)
	if err != nil {
		log.Printf("Error getting Matrix messages: %v", err)
		// Continue with empty history
		recentMessages = []*domain.Message{}
	} else {
		// matrixMessages are already domain.Message objects
		for _, msg := range matrixMessages {
			// Skip empty messages
			if msg.Content == "" {
				continue
			}

			// Add to recent messages
			recentMessages = append(recentMessages, msg)
		}

		log.Printf("Retrieved %d messages from Matrix for room %s", len(recentMessages), roomID)
	}

	// Get timestamp for current message
	currentTimestamp := time.Now().
		UnixNano() /
		1e6 // Current time in milliseconds

	// Note: We're not storing messages locally anymore - always fetching from homeserver

	log.Printf(
		"Processing %d messages from Matrix for room %s",
		len(recentMessages),
		roomID,
	)
	conversationMessages := []domain.ConversationMessage{}

	// Messages from Matrix API are in reverse chronological order (newest first)
	// We need to reverse them to get chronological order (oldest first)
	// First make a copy to avoid modifying the original slice
	messagesToProcess := make([]*domain.Message, len(recentMessages))
	for i := 0; i < len(recentMessages); i++ {
		// Reverse the order by reading from the end of the recentMessages slice
		messagesToProcess[i] = recentMessages[len(recentMessages)-1-i]
	}

	log.Printf(
		"Reversed message order to get chronological sequence (oldest first, newest last)",
	)

	// We're including all messages in the history for context
	log.Printf("Using %d previous messages for context in room %s (type: %s)",
		len(messagesToProcess), roomID, roomType)

	// Process messages in chronological order (oldest first)
	for _, msg := range messagesToProcess {
		// Skip empty messages
		if msg.Content == "" {
			continue
		}

		// Determine role based on sender
		role := domain.RoleUser
		if msg.IsFromBot {
			role = domain.RoleAssistant
		}

		// For group chats, clean up bot mentions in messages
		msgContent := msg.Content
		botID := h.matrixService.GetBotUserID()

		// Get the localpart of the bot's username (e.g., "henry" from "henry:henhouse.im")
		localpart := strings.TrimPrefix(botID, "@")
		if idx := strings.Index(localpart, ":"); idx >= 0 {
			localpart = localpart[:idx]
		}

		if !msg.IsFromBot && (strings.Contains(msgContent, botID) ||
			strings.Contains(strings.ToLower(msgContent), strings.ToLower(localpart))) {
			// Remove the bot mentions for cleaner history
			// First remove full mention
			msgContent = strings.ReplaceAll(msgContent, botID, "")

			// Then try to remove just the name with word boundaries
			msgContent = strings.ReplaceAll(msgContent, " "+localpart+" ", " ")
			msgContent = strings.ReplaceAll(msgContent, " "+localpart, "")
			if strings.HasPrefix(msgContent, localpart+" ") {
				msgContent = msgContent[len(localpart)+1:]
			}
			if msgContent == localpart {
				msgContent = ""
			}

			msgContent = strings.TrimSpace(msgContent)
		}

		// Add to context with timestamp and sender information
		if msgContent != "" {
			conversationMessages = append(
				conversationMessages,
				domain.ConversationMessage{
					Role:      role,
					Content:   msgContent,
					Timestamp: msg.Timestamp,
					SenderID:  msg.SenderID,
				},
			)
		}
	}

	// Log the number of context messages being used
	log.Printf(
		"Using %d previous messages for context in room %s",
		len(conversationMessages),
		roomID,
	)

	// Check if the current message already exists in the conversation history
	// This can happen when the latest message is included in the Matrix history
	// We don't want to add it twice
	currentMessageExists := false

	// Matrix doesn't guarantee the exact timestamp matches, so check for content and sender
	for _, msg := range conversationMessages {
		// If the message has the same content and sender, it's likely the same message
		// We allow for a small timestamp difference (5 seconds)
		if msg.Content == currentMessage && msg.SenderID == senderID &&
			(currentTimestamp-msg.Timestamp < 5000) { // 5 seconds
			log.Printf(
				"Current message already exists in history, not adding again",
			)
			currentMessageExists = true
			break
		}
	}

	// Add the current message only if it doesn't already exist
	if !currentMessageExists {
		log.Printf("Adding current message to context")
		conversationMessages = append(
			conversationMessages,
			domain.ConversationMessage{
				Role:      domain.RoleUser,
				Content:   currentMessage,
				Timestamp: currentTimestamp,
				SenderID:  senderID,
			},
		)
	}

	return conversationMessages, nil
}
