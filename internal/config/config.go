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

package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config holds all the configuration for the bot
type Config struct {
	// Matrix configuration
	MatrixHomeserver  string
	MatrixUserID      string
	MatrixAccessToken string
	MatrixPassword    string
	SyncTokenFile     string

	// Claude configuration
	ClaudeAPIKey string

	// Bot configuration
	ContextMessageCount int
	AllowedDomain       string
	OwnerID             string
}

// LoadConfig loads the configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		MatrixHomeserver:  os.Getenv("HENRY_MATRIX_HOMESERVER"),
		MatrixUserID:      os.Getenv("HENRY_MATRIX_USER_ID"),
		MatrixAccessToken: os.Getenv("HENRY_MATRIX_ACCESS_TOKEN"),
		MatrixPassword:    os.Getenv("HENRY_MATRIX_PASSWORD"),
		ClaudeAPIKey:      os.Getenv("HENRY_CLAUDE_API_KEY"),
		OwnerID:           os.Getenv("HENRY_OWNER_ID"),
		AllowedDomain:     "henhouse.im", // Default allowed domain
		SyncTokenFile:     os.Getenv("HENRY_SYNC_TOKEN_FILE"),
	}

	// Validate required configuration
	if config.MatrixHomeserver == "" {
		return nil, errors.New(
			"HENRY_MATRIX_HOMESERVER environment variable must be set",
		)
	}

	if config.MatrixUserID == "" {
		return nil, errors.New(
			"HENRY_MATRIX_USER_ID environment variable must be set",
		)
	}

	if config.MatrixAccessToken == "" && config.MatrixPassword == "" {
		return nil, errors.New(
			"either HENRY_MATRIX_ACCESS_TOKEN or HENRY_MATRIX_PASSWORD must be set",
		)
	}

	if config.ClaudeAPIKey == "" {
		return nil, errors.New(
			"HENRY_CLAUDE_API_KEY environment variable must be set",
		)
	}

	// Parse context message count with a default of 10
	contextCountStr := os.Getenv("HENRY_CONTEXT_MESSAGE_COUNT")
	if contextCountStr == "" {
		config.ContextMessageCount = 10
	} else {
		contextCount, err := strconv.Atoi(contextCountStr)
		if err != nil {
			return nil, fmt.Errorf("invalid HENRY_CONTEXT_MESSAGE_COUNT: %v", err)
		}
		config.ContextMessageCount = contextCount
	}

	// Allow override of allowed domain
	if allowedDomain := os.Getenv("HENRY_ALLOWED_DOMAIN"); allowedDomain != "" {
		config.AllowedDomain = allowedDomain
	}

	// Set default sync token file if not provided
	if config.SyncTokenFile == "" {
		config.SyncTokenFile = "sync_token.txt"
	}

	return config, nil
}
