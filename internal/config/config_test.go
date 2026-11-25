package config

import (
	"os"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected []string
	}{
		{
			name: "all valid",
			config: Config{
				DiscordBotToken:  "valid_token",
				DiscordGuildID:   "123456789",
				DiscordChannelID: "987654321",
			},
			expected: []string{},
		},
		{
			name: "all empty",
			config: Config{
				DiscordBotToken:  "",
				DiscordGuildID:   "",
				DiscordChannelID: "",
			},
			expected: []string{"DISCORD_BOT_TOKEN", "DISCORD_GUILD_ID", "DISCORD_CHANNEL_ID"},
		},
		{
			name: "all placeholders",
			config: Config{
				DiscordBotToken:  "your_discord_bot_token",
				DiscordGuildID:   "your_discord_guild_id",
				DiscordChannelID: "your_discord_channel_id",
			},
			expected: []string{"DISCORD_BOT_TOKEN", "DISCORD_GUILD_ID", "DISCORD_CHANNEL_ID"},
		},
		{
			name: "partial valid",
			config: Config{
				DiscordBotToken:  "valid_token",
				DiscordGuildID:   "",
				DiscordChannelID: "your_discord_channel_id",
			},
			expected: []string{"DISCORD_GUILD_ID", "DISCORD_CHANNEL_ID"},
		},
		{
			name: "token placeholder only",
			config: Config{
				DiscordBotToken:  "your_discord_bot_token",
				DiscordGuildID:   "123456789",
				DiscordChannelID: "987654321",
			},
			expected: []string{"DISCORD_BOT_TOKEN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing := tt.config.Validate()

			if len(missing) != len(tt.expected) {
				t.Errorf("Validate() returned %d missing vars, expected %d", len(missing), len(tt.expected))
				t.Errorf("Got: %v, Expected: %v", missing, tt.expected)
				return
			}

			for i, v := range missing {
				if v != tt.expected[i] {
					t.Errorf("Validate()[%d] = %s, expected %s", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestConfig_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected bool
	}{
		{
			name: "valid config",
			config: Config{
				DiscordBotToken:  "valid_token",
				DiscordGuildID:   "123456789",
				DiscordChannelID: "987654321",
			},
			expected: true,
		},
		{
			name: "invalid config - empty",
			config: Config{
				DiscordBotToken:  "",
				DiscordGuildID:   "123456789",
				DiscordChannelID: "987654321",
			},
			expected: false,
		},
		{
			name: "invalid config - placeholder",
			config: Config{
				DiscordBotToken:  "your_discord_bot_token",
				DiscordGuildID:   "123456789",
				DiscordChannelID: "987654321",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("DISCORD_BOT_TOKEN")
	origGuild := os.Getenv("DISCORD_GUILD_ID")
	origChannel := os.Getenv("DISCORD_CHANNEL_ID")
	origYT := os.Getenv("YOUTUBE_API_KEY")

	defer func() {
		// Restore original env vars
		os.Setenv("DISCORD_BOT_TOKEN", origToken)
		os.Setenv("DISCORD_GUILD_ID", origGuild)
		os.Setenv("DISCORD_CHANNEL_ID", origChannel)
		os.Setenv("YOUTUBE_API_KEY", origYT)
	}()

	// Set test env vars
	os.Setenv("DISCORD_BOT_TOKEN", "test_token")
	os.Setenv("DISCORD_GUILD_ID", "test_guild")
	os.Setenv("DISCORD_CHANNEL_ID", "test_channel")
	os.Setenv("YOUTUBE_API_KEY", "test_yt_key")

	cfg, _ := Load()

	if cfg.DiscordBotToken != "test_token" {
		t.Errorf("DiscordBotToken = %s, expected test_token", cfg.DiscordBotToken)
	}
	if cfg.DiscordGuildID != "test_guild" {
		t.Errorf("DiscordGuildID = %s, expected test_guild", cfg.DiscordGuildID)
	}
	if cfg.DiscordChannelID != "test_channel" {
		t.Errorf("DiscordChannelID = %s, expected test_channel", cfg.DiscordChannelID)
	}
	if cfg.YouTubeAPIKey != "test_yt_key" {
		t.Errorf("YouTubeAPIKey = %s, expected test_yt_key", cfg.YouTubeAPIKey)
	}
}
