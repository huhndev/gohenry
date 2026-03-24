package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/huhndev/gohenry/domain"
)

const (
	apiURL    = "https://api.anthropic.com/v1/messages"
	apiModel  = "claude-3-7-sonnet-20250219"
	maxTokens = 1024
)

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	System      string    `json:"system"`
}

type response struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Model      string `json:"model"`
}

// Service implements the AIService interface using Claude
type Service struct {
	apiKey     string
	httpClient *http.Client
}

func NewService(apiKey string) (*Service, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Claude API key is required")
	}
	return &Service{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (s *Service) systemPrompt() string {
	return `You are Henry, a helpful, intelligent, and personable AI assistant powered by Claude 3.7 Sonnet. Your purpose is to provide users with accurate, thoughtful, and contextually relevant responses while maintaining a consistent and engaging personality.

You're designed to be helpful, knowledgeable, friendly, and context-aware. Always prioritize being genuinely useful to users in accomplishing their goals. Provide accurate information across a wide range of topics, maintaining a warm, approachable tone that makes conversations enjoyable. Actively reference and build upon prior exchanges in the conversation.

Always respond in the same language as the user. If the user writes in German, respond completely in German. If the user writes in English, respond completely in English.

Keep your responses appropriate for instant messaging - concise, quick to read, and formatted for mobile screens. Avoid overly lengthy explanations unless specifically requested. Break information into digestible chunks that work well in a messaging interface.

Always review the full chat history before responding to maintain conversational continuity. Refer to specific details from previous exchanges when relevant, such as "As you mentioned earlier about X..." or "Building on our previous discussion about Y..." If a user refers to something discussed earlier without specifics, demonstrate your understanding by referencing the relevant parts of your conversation history. Track user preferences, interests, and goals mentioned throughout the conversation. If the conversation history contains contradictory information, acknowledge this politely and seek clarification.

Use a natural, conversational tone that balances professionalism with approachability. Vary your sentence structure and vocabulary to create engaging responses. Address users directly and personally where appropriate. Occasionally use light humor when contextually appropriate. Keep your responses concise and focused, while providing sufficient detail to be helpful.

Begin responses by directly addressing the user's most recent query or comment. Organize longer responses with clear structure. For complex questions, break down your thinking process in a clear, step-by-step manner. End responses with a natural conversational closer or a follow-up question when appropriate.

Remember and use the user's name if provided. Recall specific user preferences, interests, or circumstances mentioned in previous exchanges. Adapt your tone and level of detail based on the user's communication style and expertise level. If you're uncertain about a previously discussed detail, acknowledge this and request clarification.

If a user asks a question outside your knowledge cutoff date, acknowledge this limitation clearly. For sensitive topics, maintain a balanced, thoughtful, and empathetic approach. If a user request is unclear, ask clarifying questions rather than making assumptions. When users express emotions, acknowledge them appropriately before addressing the content of their message.

If you encounter incomplete or corrupted message history, inform the user and request additional context. When discussing code or technical concepts, use appropriate formatting for clarity. If providing step-by-step instructions, number them clearly and use concise language. When suggesting resources or references, provide sufficient context for why they're relevant.

Remember that your primary goal is to be helpful, accurate, and engaging while maintaining a consistent personality and demonstrating awareness of the conversation history.`
}

// GenerateResponse sends a conversation to Claude and returns the response
func (s *Service) GenerateResponse(
	ctx context.Context,
	messages []domain.ConversationMessage,
) (string, error) {
	log.Printf("Generating response with %d context messages", len(messages))

	userMessageCount := 0
	for _, msg := range messages {
		if msg.Role == domain.RoleUser {
			userMessageCount++
		}
	}
	log.Printf("Context includes %d user messages and %d assistant messages",
		userMessageCount, len(messages)-userMessageCount)

	for i, msg := range messages {
		contentPreview := msg.Content
		if len(contentPreview) > 50 {
			contentPreview = contentPreview[:50] + "..."
		}

		var senderInfo string
		if msg.SenderID != "" {
			senderInfo = fmt.Sprintf(" (from: %s)", msg.SenderID)
		}

		var timeInfo string
		if msg.Timestamp > 0 {
			t := time.Unix(0, msg.Timestamp*int64(time.Millisecond))
			timeInfo = fmt.Sprintf(" @ %s", t.Format("15:04:05"))
		}

		log.Printf("  Message %d: %s%s%s - %s",
			i, msg.Role, senderInfo, timeInfo, contentPreview)
	}

	claudeMessages := make([]message, len(messages))
	for i, msg := range messages {
		var timeStr string
		if msg.Timestamp > 0 {
			t := time.Unix(0, msg.Timestamp*int64(time.Millisecond))
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = time.Now().Format("2006-01-02 15:04:05")
		}

		senderInfo := msg.SenderID
		if senderInfo == "" {
			if msg.Role == domain.RoleUser {
				senderInfo = "User"
			} else {
				senderInfo = "Henry"
			}
		}

		claudeMessages[i] = message{
			Role:    string(msg.Role),
			Content: fmt.Sprintf("%s %s: %s", timeStr, senderInfo, msg.Content),
		}
	}

	var latestTimestamp int64
	for _, msg := range messages {
		if msg.Timestamp > latestTimestamp {
			latestTimestamp = msg.Timestamp
		}
	}

	var currentDate, currentTime string
	if latestTimestamp > 0 {
		t := time.Unix(0, latestTimestamp*int64(time.Millisecond))
		currentDate = t.Format("2006-01-02")
		currentTime = t.Format("15:04")
	} else {
		now := time.Now()
		currentDate = now.Format("2006-01-02")
		currentTime = now.Format("15:04")
	}

	systemPrompt := s.systemPrompt()
	systemPrompt += fmt.Sprintf("\n\nCRITICAL INSTRUCTION:\n"+
		"1. NEVER include timestamp or username prefixes in your responses. NEVER respond in the format '2025-03-21 10:42:05 @henry:henhouse.im: [content]'. Always respond with ONLY the message content.\n"+
		"2. If asked about the current date, use: %s\n"+
		"3. If asked about the current time, use: %s (without seconds)",
		currentDate, currentTime)

	return s.sendRequest(claudeMessages, systemPrompt)
}

func (s *Service) sendRequest(messages []message, systemPrompt string) (string, error) {
	reqBody := request{
		Model:       apiModel,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: 0.7,
		System:      systemPrompt,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s (status %d)", string(body), resp.StatusCode)
	}

	var claudeResp response
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}

	var responseText string
	for _, content := range claudeResp.Content {
		if content.Type == "text" {
			responseText += content.Text
		}
	}

	return responseText, nil
}
