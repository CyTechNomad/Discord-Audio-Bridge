// Package config handles loading and validating environment configuration.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	DiscordBotToken  string
	DiscordGuildID   string
	DiscordChannelID string
	YouTubeAPIKey    string
}

// Placeholders that indicate unconfigured values
var placeholders = map[string]string{
	"DISCORD_BOT_TOKEN":  "your_discord_bot_token",
	"DISCORD_GUILD_ID":   "your_discord_guild_id",
	"DISCORD_CHANNEL_ID": "your_discord_channel_id",
}

// Load loads configuration from .env file and environment variables.
// Returns the config and any warning message (e.g., if .env file not found).
func Load() (*Config, string) {
	var warning string
	if err := godotenv.Load(); err != nil {
		warning = fmt.Sprintf("Warning: Error loading .env file: %v", err)
	}

	return &Config{
		DiscordBotToken:  os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordGuildID:   os.Getenv("DISCORD_GUILD_ID"),
		DiscordChannelID: os.Getenv("DISCORD_CHANNEL_ID"),
		YouTubeAPIKey:    os.Getenv("YOUTUBE_API_KEY"),
	}, warning
}

// Validate checks that all required configuration values are present and not placeholders.
// Returns a list of missing or invalid variable names.
func (c *Config) Validate() []string {
	var missing []string

	if c.DiscordBotToken == "" || c.DiscordBotToken == placeholders["DISCORD_BOT_TOKEN"] {
		missing = append(missing, "DISCORD_BOT_TOKEN")
	}
	if c.DiscordGuildID == "" || c.DiscordGuildID == placeholders["DISCORD_GUILD_ID"] {
		missing = append(missing, "DISCORD_GUILD_ID")
	}
	if c.DiscordChannelID == "" || c.DiscordChannelID == placeholders["DISCORD_CHANNEL_ID"] {
		missing = append(missing, "DISCORD_CHANNEL_ID")
	}

	return missing
}

// IsValid returns true if the configuration has all required values.
func (c *Config) IsValid() bool {
	return len(c.Validate()) == 0
}
