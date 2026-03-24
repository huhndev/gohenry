package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	MatrixHomeserver    string
	MatrixUserID        string
	MatrixAccessToken   string
	MatrixPassword      string
	SyncTokenFile       string
	ClaudeAPIKey        string
	ContextMessageCount int
	AllowedDomain       string
	OwnerID             string
}

func LoadConfig() (*Config, error) {
	config := &Config{
		MatrixHomeserver:  os.Getenv("HENRY_MATRIX_HOMESERVER"),
		MatrixUserID:      os.Getenv("HENRY_MATRIX_USER_ID"),
		MatrixAccessToken: os.Getenv("HENRY_MATRIX_ACCESS_TOKEN"),
		MatrixPassword:    os.Getenv("HENRY_MATRIX_PASSWORD"),
		ClaudeAPIKey:      os.Getenv("HENRY_CLAUDE_API_KEY"),
		OwnerID:           os.Getenv("HENRY_OWNER_ID"),
		AllowedDomain:     "henhouse.im",
		SyncTokenFile:     os.Getenv("HENRY_SYNC_TOKEN_FILE"),
	}

	if config.MatrixHomeserver == "" {
		return nil, errors.New("HENRY_MATRIX_HOMESERVER environment variable must be set")
	}
	if config.MatrixUserID == "" {
		return nil, errors.New("HENRY_MATRIX_USER_ID environment variable must be set")
	}
	if config.MatrixAccessToken == "" && config.MatrixPassword == "" {
		return nil, errors.New("either HENRY_MATRIX_ACCESS_TOKEN or HENRY_MATRIX_PASSWORD must be set")
	}
	if config.ClaudeAPIKey == "" {
		return nil, errors.New("HENRY_CLAUDE_API_KEY environment variable must be set")
	}

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

	if allowedDomain := os.Getenv("HENRY_ALLOWED_DOMAIN"); allowedDomain != "" {
		config.AllowedDomain = allowedDomain
	}

	if config.SyncTokenFile == "" {
		config.SyncTokenFile = "sync_token.txt"
	}

	return config, nil
}
