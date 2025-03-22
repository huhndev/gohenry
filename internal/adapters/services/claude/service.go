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

package claude

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/huhnsystems/gohenry/internal/domain"
)

// Service implements the AIService interface using Claude
type Service struct {
	client *Client
}

// NewService creates a new Claude service
func NewService(apiKey string) (*Service, error) {
	client, err := NewClient(apiKey)
	if err != nil {
		return nil, err
	}

	return &Service{
		client: client,
	}, nil
}

// GenerateResponse sends a conversation to Claude and returns the response
func (s *Service) GenerateResponse(
	ctx context.Context,
	messages []domain.ConversationMessage,
) (string, error) {
	// Log the conversation context being used
	log.Printf("Generating response with %d context messages", len(messages))

	// Calculate if we have multiple user messages to determine if history is available
	userMessageCount := 0
	for _, msg := range messages {
		if msg.Role == domain.RoleUser {
			userMessageCount++
		}
	}
	log.Printf("Context includes %d user messages and %d assistant messages",
		userMessageCount, len(messages)-userMessageCount)

	// Log each message in the context (with truncated content for readability)
	for i, msg := range messages {
		// Only log a snippet of the message content to avoid cluttering logs
		contentPreview := msg.Content
		if len(contentPreview) > 50 {
			contentPreview = contentPreview[:50] + "..."
		}

		// Include metadata in log
		var senderInfo string
		if msg.SenderID != "" {
			senderInfo = fmt.Sprintf(" (from: %s)", msg.SenderID)
		}

		// Format timestamp if available
		var timeInfo string
		if msg.Timestamp > 0 {
			t := time.Unix(0, msg.Timestamp*int64(time.Millisecond))
			timeInfo = fmt.Sprintf(" @ %s", t.Format("15:04:05"))
		}

		log.Printf("  Message %d: %s%s%s - %s",
			i, msg.Role, senderInfo, timeInfo, contentPreview)
	}

	// Convert domain messages to Claude messages with timestamp and sender prefix for all messages
	claudeMessages := make([]Message, len(messages))
	for i, msg := range messages {
		// Format timestamp if available
		var timeStr string
		if msg.Timestamp > 0 {
			t := time.Unix(0, msg.Timestamp*int64(time.Millisecond))
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			// Use current time as fallback
			timeStr = time.Now().Format("2006-01-02 15:04:05")
		}

		// Get sender information
		senderInfo := msg.SenderID
		if senderInfo == "" {
			if msg.Role == domain.RoleUser {
				senderInfo = "User"
			} else {
				senderInfo = "Henry"
			}
		}

		// Combine timestamp, sender, and content for all messages
		contentToSend := fmt.Sprintf(
			"%s %s: %s",
			timeStr,
			senderInfo,
			msg.Content,
		)

		claudeMessages[i] = Message{
			Role:    string(msg.Role),
			Content: contentToSend,
		}
	}

	// Find the latest timestamp from the messages for current date/time reference
	var latestTimestamp int64
	for _, msg := range messages {
		if msg.Timestamp > latestTimestamp {
			latestTimestamp = msg.Timestamp
		}
	}

	// Format the current date and time (without seconds)
	var currentDate, currentTime string
	if latestTimestamp > 0 {
		t := time.Unix(0, latestTimestamp*int64(time.Millisecond))
		currentDate = t.Format("2006-01-02")
		currentTime = t.Format("15:04") // Hour:Minute format without seconds
	} else {
		// Fallback to system time if no timestamps available
		now := time.Now()
		currentDate = now.Format("2006-01-02")
		currentTime = now.Format("15:04")
	}

	// Update system prompt to instruct Claude about time/date handling and not to include prefixes
	systemPrompt := s.client.GetSystemPrompt()
	systemPrompt += fmt.Sprintf("\n\nCRITICAL INSTRUCTION:\n"+
		"1. NEVER include timestamp or username prefixes in your responses. NEVER respond in the format '2025-03-21 10:42:05 @henry:henhouse.im: [content]'. Always respond with ONLY the message content.\n"+
		"2. If asked about the current date, use: %s\n"+
		"3. If asked about the current time, use: %s (without seconds)",
		currentDate, currentTime)

	// Send the request to Claude
	return s.client.SendRequestWithCustomSystemPrompt(
		claudeMessages,
		systemPrompt,
	)
}
