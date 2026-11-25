package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/CyTechNomad/PipeDream/internal/config"
	"github.com/CyTechNomad/PipeDream/internal/discord"
	"github.com/CyTechNomad/PipeDream/internal/server"
	"github.com/CyTechNomad/PipeDream/internal/updater"
	"github.com/CyTechNomad/PipeDream/internal/webrtc"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	doUpdate := flag.Bool("update", false, "Update to the latest version")
	flag.Parse()

	if *showVersion {
		fmt.Println("discord-audio-bridge", version)
		os.Exit(0)
	}

	if *doUpdate {
		u := updater.New(version)
		if err := u.Update(); err != nil {
			log.Printf("Update failed: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Print startup banner
	printBanner()

	// Check for updates in background (non-blocking)
	go func() {
		u := updater.New(version)
		if latest := u.CheckForUpdates(); latest != "" {
			fmt.Printf("\n*** New version %s available! Run with -update to upgrade ***\n\n", latest)
		}
	}()

	// Load and validate configuration
	cfg, warning := config.Load()
	if warning != "" {
		log.Println(warning)
		log.Println("Continuing with existing environment variables...")
	}

	if missing := cfg.Validate(); len(missing) > 0 {
		log.Println("ERROR: Missing required environment variables:")
		for _, v := range missing {
			log.Printf("  - %s", v)
		}
		log.Println("\nPlease edit your .env file and add the missing values.")
		log.Println("Press Enter to exit...")
		fmt.Scanln()
		os.Exit(1)
	}

	// Set up context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, cancelling tasks...")
		cancel()
	}()

	// Initialize Discord connection
	discordConn, err := discord.New(cfg.DiscordBotToken)
	if err != nil {
		log.Fatalf("Failed to create Discord session: %v", err)
	}
	defer discordConn.Close()

	log.Println("Discord session created successfully")

	// Initialize WebRTC handler
	webrtcHandler := webrtc.NewHandler(discordConn)

	// Initialize HTTP server
	srv := server.New(":8080", webrtcHandler, cfg.YouTubeAPIKey)

	// Start services
	var wg sync.WaitGroup

	// Start web server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Start(ctx); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Join voice channel
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := discordConn.JoinVoice(ctx, cfg.DiscordGuildID, cfg.DiscordChannelID, 3); err != nil {
			log.Printf("Failed to join voice channel: %v", err)
			cancel()
		}

		// Wait for context cancellation
		<-ctx.Done()
	}()

	wg.Wait()
	log.Println("All tasks completed")
}

func printBanner() {
	fmt.Println("╔═══════════════════════════════════════════╗")
	fmt.Println("║       Discord Audio Bridge                ║")
	fmt.Printf("║       Version: %-26s ║\n", version)
	fmt.Println("╠═══════════════════════════════════════════╣")
	fmt.Println("║  Web UI:     http://localhost:8080        ║")
	fmt.Println("║  Mixer:      http://localhost:8080/mixer  ║")
	fmt.Println("╚═══════════════════════════════════════════╝")
	fmt.Println()
}
