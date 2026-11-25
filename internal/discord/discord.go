// Package discord handles Discord bot connections and voice channels.
package discord

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Connection manages the Discord bot session and voice connection.
type Connection struct {
	Session         *discordgo.Session
	VoiceConnection *discordgo.VoiceConnection
	GuildID         string
	ChannelID       string
	mu              sync.RWMutex
}

// New creates a new Discord connection with the given bot token.
func New(token string) (*Connection, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %w", err)
	}

	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildVoiceStates

	if err := session.Open(); err != nil {
		return nil, fmt.Errorf("error opening Discord connection: %w", err)
	}

	return &Connection{
		Session: session,
	}, nil
}

// JoinVoice joins a voice channel with retry logic.
func (c *Connection) JoinVoice(ctx context.Context, guildID, channelID string, maxRetries int) error {
	c.GuildID = guildID
	c.ChannelID = channelID

	// Wait for Discord connection to stabilize
	time.Sleep(2 * time.Second)

	var vc *discordgo.VoiceConnection
	var err error

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Printf("Attempting to join voice channel (attempt %d/%d)", i+1, maxRetries)

		vc, err = c.Session.ChannelVoiceJoin(guildID, channelID, false, false)
		if err == nil {
			break
		}

		if i < maxRetries-1 {
			time.Sleep(time.Second)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to join voice channel after %d attempts: %w", maxRetries, err)
	}

	// Set speaking mode
	if err := vc.Speaking(true); err != nil {
		log.Printf("Failed to set speaking mode: %v", err)
	} else {
		log.Println("Set Discord speaking mode to true")
	}

	log.Printf("Discord voice connection ready: %v", vc.Ready)

	c.mu.Lock()
	c.VoiceConnection = vc
	c.mu.Unlock()

	fmt.Println("Joined voice channel successfully")
	return nil
}

// GetVoiceConnection returns the current voice connection.
func (c *Connection) GetVoiceConnection() *discordgo.VoiceConnection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.VoiceConnection
}

// SendOpus sends Opus audio data to Discord.
// Returns false if the connection is not ready.
func (c *Connection) SendOpus(data []byte, timeout time.Duration) bool {
	c.mu.RLock()
	vc := c.VoiceConnection
	c.mu.RUnlock()

	if vc == nil || !vc.Ready {
		return false
	}

	select {
	case vc.OpusSend <- data:
		return true
	case <-time.After(timeout):
		return false
	}
}

// IsReady returns true if the voice connection is ready.
func (c *Connection) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.VoiceConnection != nil && c.VoiceConnection.Ready
}

// Close closes the Discord session.
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.VoiceConnection != nil {
		c.VoiceConnection.Disconnect()
		c.VoiceConnection = nil
	}

	if c.Session != nil {
		return c.Session.Close()
	}

	return nil
}
