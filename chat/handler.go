package chat

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/huhndev/gohenry/config"
	"github.com/huhndev/gohenry/domain"
)

// MessageHandler processes incoming messages and decides how to respond
type MessageHandler struct {
	config        *config.Config
	matrixService domain.MatrixService
	aiService     domain.AIService
}

func NewMessageHandler(
	cfg *config.Config,
	matrixService domain.MatrixService,
	aiService domain.AIService,
) *MessageHandler {
	return &MessageHandler{
		config:        cfg,
		matrixService: matrixService,
		aiService:     aiService,
	}
}

// HandleMessage processes an incoming Matrix message
func (h *MessageHandler) HandleMessage(
	ctx context.Context,
	senderID, roomID, content string,
) error {
	if content == "" {
		return nil
	}

	if !h.matrixService.IsFromAllowedDomain(senderID) {
		log.Printf("Ignoring message from non-allowed domain: %s", senderID)
		return nil
	}

	roomType, err := h.matrixService.GetRoomType(ctx, roomID)
	if err != nil {
		return fmt.Errorf("error determining room type: %v", err)
	}

	if !h.matrixService.IsAddressedToBot(content, roomType) {
		return nil
	}

	messageText := content
	if roomType == domain.GroupRoom {
		fullUsername := strings.TrimPrefix(h.matrixService.GetBotUserID(), "@")

		localpart := fullUsername
		if idx := strings.Index(fullUsername, ":"); idx >= 0 {
			localpart = fullUsername[:idx]
		}

		fullMention := fmt.Sprintf("@%s", fullUsername)

		messageText = strings.ReplaceAll(messageText, fullMention, "")
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

	if messageText == "" {
		return nil
	}

	contextMessages, err := h.getConversationContext(ctx, senderID, roomID, messageText)
	if err != nil {
		log.Printf("Error getting conversation context: %v", err)
		contextMessages = []domain.ConversationMessage{
			{
				Role:      domain.RoleUser,
				Content:   messageText,
				Timestamp: time.Now().UnixNano() / 1e6,
				SenderID:  senderID,
			},
		}
	}

	typingTimeout := 30000
	if err := h.matrixService.SendTyping(roomID, true, typingTimeout); err != nil {
		log.Printf("Error sending typing notification: %v", err)
	} else {
		log.Printf("Sent typing indicator to room %s", roomID)
	}

	log.Printf("Generating response for message in room %s", roomID)
	aiResponse, err := h.aiService.GenerateResponse(ctx, contextMessages)
	if err != nil {
		log.Printf("Error generating response: %v", err)
		if err := h.matrixService.SendMessage(roomID, "Sorry, I'm having trouble thinking right now."); err != nil {
			log.Printf("Error sending error message: %v", err)
		}
		return fmt.Errorf("error generating response: %v", err)
	}

	responseTimestamp := time.Now().UnixNano() / 1e6
	log.Printf("Generated bot response for room %s at timestamp %d", roomID, responseTimestamp)

	if err := h.matrixService.SendTyping(roomID, false, 0); err != nil {
		log.Printf("Error stopping typing notification: %v", err)
	}

	if err := h.matrixService.SendMessage(roomID, aiResponse); err != nil {
		return fmt.Errorf("error sending response: %v", err)
	}

	log.Printf("Response sent to room %s (stored in history)", roomID)
	return nil
}

func (h *MessageHandler) getConversationContext(
	ctx context.Context,
	senderID string,
	roomID string,
	currentMessage string,
) ([]domain.ConversationMessage, error) {
	roomType, err := h.matrixService.GetRoomType(ctx, roomID)
	if err != nil {
		log.Printf("Error getting room type: %v, defaulting to direct message behavior", err)
		roomType = domain.DirectRoom
	}

	var recentMessages []*domain.Message

	log.Printf("Fetching messages from Matrix for room %s (type: %s)", roomID, roomType)

	requestLimit := h.config.ContextMessageCount

	matrixMessages, err := h.matrixService.GetRoomContext(ctx, roomID, requestLimit)
	if err != nil {
		log.Printf("Error getting Matrix messages: %v", err)
		recentMessages = []*domain.Message{}
	} else {
		for _, msg := range matrixMessages {
			if msg.Content == "" {
				continue
			}
			recentMessages = append(recentMessages, msg)
		}
		log.Printf("Retrieved %d messages from Matrix for room %s", len(recentMessages), roomID)
	}

	currentTimestamp := time.Now().UnixNano() / 1e6

	log.Printf("Processing %d messages from Matrix for room %s", len(recentMessages), roomID)
	conversationMessages := []domain.ConversationMessage{}

	messagesToProcess := make([]*domain.Message, len(recentMessages))
	for i := 0; i < len(recentMessages); i++ {
		messagesToProcess[i] = recentMessages[len(recentMessages)-1-i]
	}

	log.Printf("Reversed message order to get chronological sequence (oldest first, newest last)")

	log.Printf("Using %d previous messages for context in room %s (type: %s)",
		len(messagesToProcess), roomID, roomType)

	for _, msg := range messagesToProcess {
		if msg.Content == "" {
			continue
		}

		role := domain.RoleUser
		if msg.IsFromBot {
			role = domain.RoleAssistant
		}

		msgContent := msg.Content
		botID := h.matrixService.GetBotUserID()

		localpart := strings.TrimPrefix(botID, "@")
		if idx := strings.Index(localpart, ":"); idx >= 0 {
			localpart = localpart[:idx]
		}

		if !msg.IsFromBot && (strings.Contains(msgContent, botID) ||
			strings.Contains(strings.ToLower(msgContent), strings.ToLower(localpart))) {
			msgContent = strings.ReplaceAll(msgContent, botID, "")
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

		if msgContent != "" {
			conversationMessages = append(conversationMessages, domain.ConversationMessage{
				Role:      role,
				Content:   msgContent,
				Timestamp: msg.Timestamp,
				SenderID:  msg.SenderID,
			})
		}
	}

	log.Printf("Using %d previous messages for context in room %s",
		len(conversationMessages), roomID)

	currentMessageExists := false
	for _, msg := range conversationMessages {
		if msg.Content == currentMessage && msg.SenderID == senderID &&
			(currentTimestamp-msg.Timestamp < 5000) {
			log.Printf("Current message already exists in history, not adding again")
			currentMessageExists = true
			break
		}
	}

	if !currentMessageExists {
		log.Printf("Adding current message to context")
		conversationMessages = append(conversationMessages, domain.ConversationMessage{
			Role:      domain.RoleUser,
			Content:   currentMessage,
			Timestamp: currentTimestamp,
			SenderID:  senderID,
		})
	}

	return conversationMessages, nil
}
