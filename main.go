package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/pion/webrtc/v3"
)

const (
	repoOwner = "CyTechNomad"
	repoName  = "Discord-Audio-Bridge"
)

var (
	version     = "dev"
	cancelFuncs []context.CancelFunc
	cancelMutex sync.Mutex
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins in development
		},
	}
)

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	doUpdate := flag.Bool("update", false, "Update to the latest version")
	flag.Parse()

	if *showVersion {
		fmt.Println("discord-audio-bridge", version)
		os.Exit(0)
	}

	if *doUpdate {
		if err := selfUpdate(); err != nil {
			log.Printf("Update failed: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Print startup banner
	fmt.Println("╔═══════════════════════════════════════════╗")
	fmt.Println("║       Discord Audio Bridge                ║")
	fmt.Printf("║       Version: %-26s ║\n", version)
	fmt.Println("╠═══════════════════════════════════════════╣")
	fmt.Println("║  Web UI:     http://localhost:8080        ║")
	fmt.Println("║  Mixer:      http://localhost:8080/mixer  ║")
	fmt.Println("╚═══════════════════════════════════════════╝")
	fmt.Println()

	// Check for updates in background (non-blocking)
	go checkForUpdates()

	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Println("Continuing with existing environment variables...")
	}

	// Validate required environment variables
	missingVars := []string{}
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" || token == "your_discord_bot_token" {
		missingVars = append(missingVars, "DISCORD_BOT_TOKEN")
	}
	guildID := os.Getenv("DISCORD_GUILD_ID")
	if guildID == "" || guildID == "your_discord_guild_id" {
		missingVars = append(missingVars, "DISCORD_GUILD_ID")
	}
	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	if channelID == "" || channelID == "your_discord_channel_id" {
		missingVars = append(missingVars, "DISCORD_CHANNEL_ID")
	}
	if len(missingVars) > 0 {
		log.Println("ERROR: Missing required environment variables:")
		for _, v := range missingVars {
			log.Printf("  - %s", v)
		}
		log.Println("\nPlease edit your .env file and add the missing values.")
		log.Println("Press Enter to exit...")
		fmt.Scanln()
		os.Exit(1)
	}

	cancelChan := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())

	// Add the cancel function to the slice
	cancelMutex.Lock()
	cancelFuncs = append(cancelFuncs, cancel)
	cancelMutex.Unlock()

	wg := new(sync.WaitGroup)

	// Start HTTP server to serve the web page and handle WebRTC
	wg.Add(1)
	go startWebServer(ctx, wg)

	// Discord connection (use existing one)
	wg.Add(1)
	go discord(ctx, wg)

	// Give components time to initialize
	time.Sleep(1 * time.Second)

	// Wait for a signal to cancel
	signal.Notify(cancelChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-cancelChan
		log.Println("Received shutdown signal, cancelling tasks...")

		cancelMutex.Lock()
		for _, cancel := range cancelFuncs {
			cancel()
		}
		cancelMutex.Unlock()
	}()

	wg.Wait()
	log.Println("All tasks completed")
}

func startWebServer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Serve static files
	http.Handle("/", http.FileServer(http.Dir("./static")))
	// Handle WebRTC connections
	http.HandleFunc("/rtc", func(w http.ResponseWriter, r *http.Request) {
		handleWebRTC(w, r)
	})

	// API key endpoint
	http.HandleFunc("/api/youtube-key", func(w http.ResponseWriter, r *http.Request) {
		apiKey := os.Getenv("YOUTUBE_API_KEY")
		if apiKey == "" {
			http.Error(w, "YouTube API key not configured", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"key": apiKey})
	})

	// Create server with proper context handling
	server := &http.Server{
		Addr: ":8080",
	}

	// Start the server in a goroutine
	go func() {
		log.Println("Starting web server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown the server gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	log.Println("Shutting down web server...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}

// Global variable to store Discord voice connection
var discordVC *discordgo.VoiceConnection

func handleWebRTC(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Println("WebSocket connection established")

	// Create WebRTC configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Printf("Error creating WebRTC peer connection: %v", err)
		return
	}
	defer peerConnection.Close()

	// Set the handler for ICE connection state
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("ICE Connection State has changed: %s", connectionState.String())
	})

	// Set the handler for Peer connection state
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Peer Connection State has changed: %s", state.String())
	})

	// Handle incoming audio tracks
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Received track: %s", track.Codec().MimeType)

		if discordVC == nil {
			log.Println("Discord voice connection not established yet")
			return
		}

		// Check if the track is Opus audio, which Discord requires
		if track.Codec().MimeType == webrtc.MimeTypeOpus {
			log.Println("Forwarding Opus audio to Discord")

			// Pre-set Discord to Speaking mode to ensure audio channels are open
			if err := discordVC.Speaking(true); err != nil {
				log.Printf("Error setting speaking mode: %v", err)
			}

			// Use a buffered channel to process packets more reliably
			opusChan := make(chan []byte, 100)

			// Start a separate goroutine to send to Discord
			go func() {
				for payload := range opusChan {
					// Check if Discord connection is still valid
					if discordVC == nil || !discordVC.Ready {
						log.Println("Discord voice connection not ready")
						time.Sleep(100 * time.Millisecond)
						continue
					}

					// Send Opus data to Discord with timeout and proper framing
					select {
					case discordVC.OpusSend <- payload:
						// Successfully sent to Discord
					case <-time.After(100 * time.Millisecond):
						log.Println("Timeout sending audio to Discord")
					}
				}
			}()

			for {
				// Read RTP packets containing the Opus payload
				rtp, _, readErr := track.ReadRTP()
				if readErr != nil {
					if readErr == io.EOF {
						close(opusChan)
						return
					}
					log.Printf("Error reading RTP: %v", readErr)
					close(opusChan)
					return
				}

				// Extract the Opus payload from the RTP packet
				opusPayload := rtp.Payload

				// Discord expects Opus frames in a specific format
				// Make sure frames have correct Opus frame size (20ms at 48kHz)
				if len(opusPayload) == 0 || len(opusPayload) > 1000 {
					// Skip invalid frames silently
					continue
				}

				// Send to our processing channel
				select {
				case opusChan <- opusPayload:
					// Successfully sent for processing
				default:
					// Channel buffer full, skip this frame
				}
			}
		} else {
			log.Printf("Received non-Opus track: %s, cannot forward to Discord", track.Codec().MimeType)
		}
	})

	// Create a channel for signaling
	done := make(chan struct{})

	// Handle incoming messages
	go func() {
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				close(done)
				return
			}

			if messageType != websocket.TextMessage {
				log.Println("Received non-text message, ignoring")
				continue
			}

			// Try to parse as an SDP offer
			var offer webrtc.SessionDescription
			err = json.Unmarshal(message, &offer)

			if err == nil && (offer.Type == webrtc.SDPTypeOffer) {
				log.Println("Received SDP offer")

				err = peerConnection.SetRemoteDescription(offer)
				if err != nil {
					log.Printf("Error setting remote description: %v", err)
					continue
				}

				// Create answer
				answer, err := peerConnection.CreateAnswer(nil)
				if err != nil {
					log.Printf("Error creating answer: %v", err)
					continue
				}

				// Set local description
				err = peerConnection.SetLocalDescription(answer)
				if err != nil {
					log.Printf("Error setting local description: %v", err)
					continue
				}

				// Send answer
				log.Println("Sending SDP answer to client")
				err = conn.WriteJSON(answer)
				if err != nil {
					log.Printf("Error sending answer: %v", err)
				}
			}
		}
	}()

	// Send any local ICE candidates to the client
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		log.Println("Sending ICE candidate to client")
		candidateJSON, err := json.Marshal(candidate.ToJSON())
		if err != nil {
			log.Printf("Error marshaling ICE candidate: %v", err)
			return
		}

		// Ensure the connection is still open before sending
		if err = conn.WriteMessage(websocket.TextMessage, candidateJSON); err != nil {
			log.Printf("Error sending ICE candidate: %v", err)
		} else {
			log.Printf("ICE candidate sent successfully, length: %d bytes", len(candidateJSON))
		}
	})

	// Wait until done
	<-done
}

func discord(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	sesh := discordLogin()
	if sesh == nil {
		log.Fatal("Failed to create Discord session")
		return
	}
	defer sesh.Close()

	log.Println("Discord session created successfully")

	// Join a voice channel
	voiceChannelID := os.Getenv("DISCORD_CHANNEL_ID")
	guildID := os.Getenv("DISCORD_GUILD_ID")

	if voiceChannelID == "" || guildID == "" {
		log.Fatal("Discord voice channel ID or guild ID not set in environment variables")
		return
	}

	// Wait for Discord connection to stabilize
	time.Sleep(2 * time.Second)

	// Try to connect with retry logic
	maxRetries := 3
	var vc *discordgo.VoiceConnection
	var err error

	for i := 0; i < maxRetries; i++ {
		log.Printf("Attempting to join voice channel (attempt %d/%d)", i+1, maxRetries)

		vc, err = sesh.ChannelVoiceJoin(guildID, voiceChannelID, false, false)
		if err == nil {
			break
		}
	}

	// Set speaking mode to indicate we're sending audio
	if err := vc.Speaking(true); err != nil {
		log.Printf("Failed to set speaking mode: %v", err)
	} else {
		log.Println("Set Discord speaking mode to true")
	}

	// Log voice connection details for debugging
	log.Printf("Discord voice connection ready: %v", vc.Ready)
	log.Printf("Discord voice connection speaking: %v", vc.Speaking)

	// Store the voice connection globally so the WebRTC handler can use it
	discordVC = vc

	fmt.Println("Joined voice channel successfully")

	// Wait for context cancellation
	<-ctx.Done()
}

func discordLogin() *discordgo.Session {
	// Load token from environment variable
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Println("Error: DISCORD_BOT_TOKEN environment variable not set")
		return nil
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Printf("Error creating Discord session: %v", err)
		return nil
	}
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates

	err = dg.Open()
	if err != nil {
		log.Printf("Error opening Discord connection: %v", err)
		return nil
	}

	return dg
}

// GitHubRelease represents the GitHub API response for a release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// checkForUpdates checks GitHub for a newer version and notifies the user
func checkForUpdates() {
	if version == "dev" {
		return // Skip update check for dev builds
	}

	latest, err := getLatestRelease()
	if err != nil {
		return // Silently fail - don't interrupt startup
	}

	if latest.TagName != version && latest.TagName > version {
		fmt.Printf("\n*** New version %s available! Run with -update to upgrade ***\n\n", latest.TagName)
	}
}

// getLatestRelease fetches the latest release info from GitHub
func getLatestRelease() (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// selfUpdate downloads and installs the latest version
func selfUpdate() error {
	fmt.Println("Checking for updates...")

	latest, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if latest.TagName == version {
		fmt.Println("Already running the latest version:", version)
		return nil
	}

	fmt.Printf("Updating from %s to %s...\n", version, latest.TagName)

	// Find the right asset for this platform
	assetName := getAssetName()
	var downloadURL string
	var isZip bool
	for _, asset := range latest.Assets {
		if strings.Contains(asset.Name, assetName) {
			downloadURL = asset.BrowserDownloadURL
			isZip = strings.HasSuffix(asset.Name, ".zip")
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no release found for platform: %s", assetName)
	}

	fmt.Printf("Downloading %s...\n", downloadURL)

	// Download the archive
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Save archive to temp file
	archiveData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read download: %w", err)
	}

	// Extract binary from archive
	var binaryData []byte
	binaryName := "discord-audio-bridge"
	if runtime.GOOS == "windows" {
		binaryName = "discord-audio-bridge.exe"
	}

	if isZip {
		binaryData, err = extractFromZip(archiveData, binaryName)
	} else {
		binaryData, err = extractFromTarGz(archiveData, binaryName)
	}
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create temp file for new binary
	tmpFile, err := os.CreateTemp("", "discord-audio-bridge-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Write binary to temp file
	_, err = tmpFile.Write(binaryData)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write update: %w", err)
	}

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Replace old binary with new one
	backupPath := execPath + ".old"
	os.Remove(backupPath) // Remove any existing backup

	if err := os.Rename(execPath, backupPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		// Try to restore backup
		os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to install update: %w", err)
	}

	os.Remove(backupPath) // Clean up backup

	fmt.Println("Update successful! Please restart the application.")
	return nil
}

// getAssetName returns the expected asset name fragment for the current platform
func getAssetName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "windows":
		return "windows-amd64"
	case "darwin":
		if goarch == "arm64" {
			return "darwin-arm64"
		}
		return "darwin-amd64"
	case "linux":
		if goarch == "arm64" {
			return "linux-arm64"
		}
		return "linux-amd64"
	default:
		return fmt.Sprintf("%s-%s", goos, goarch)
	}
}

// extractFromZip extracts a file from a zip archive
func extractFromZip(data []byte, filename string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, filename) {
			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}

	return nil, fmt.Errorf("file %s not found in archive", filename)
}

// extractFromTarGz extracts a file from a tar.gz archive
func extractFromTarGz(data []byte, filename string) ([]byte, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if strings.HasSuffix(header.Name, filename) {
			return io.ReadAll(tarReader)
		}
	}

	return nil, fmt.Errorf("file %s not found in archive", filename)
}
