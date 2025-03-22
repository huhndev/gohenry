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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	claudeAPIURL    = "https://api.anthropic.com/v1/messages"
	claudeAPIModel  = "claude-3-7-sonnet-20250219"
	claudeMaxTokens = 1024 // Limit response length for conciseness
)

// Client handles interactions with the Claude API
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// Message represents a message sent to or received from Claude
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request represents a request to the Claude API
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	System      string    `json:"system"`
}

// Response represents a response from the Claude API
type Response struct {
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

// NewClient creates a new Claude API client
func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Claude API key is required")
	}

	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// GetSystemPrompt returns the default system prompt for Claude
func (c *Client) GetSystemPrompt() string {
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

// SendRequest sends a request to the Claude API and returns the response
func (c *Client) SendRequest(messages []Message) (string, error) {
	return c.SendRequestWithCustomSystemPrompt(messages, c.GetSystemPrompt())
}

// SendRequestWithCustomSystemPrompt sends a request to the Claude API with a custom system prompt
func (c *Client) SendRequestWithCustomSystemPrompt(
	messages []Message,
	systemPrompt string,
) (string, error) {
	// Create request body
	reqBody := Request{
		Model:       claudeAPIModel,
		Messages:    messages,
		MaxTokens:   claudeMaxTokens,
		Temperature: 0.7,
		System:      systemPrompt,
	}

	// Marshal request to JSON
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", claudeAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"API error: %s (status %d)",
			string(body),
			resp.StatusCode,
		)
	}

	// Parse response
	var claudeResp Response
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Extract text from response
	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}

	// Combine all text parts (should typically just be one)
	var responseText string
	for _, content := range claudeResp.Content {
		if content.Type == "text" {
			responseText += content.Text
		}
	}

	return responseText, nil
}
