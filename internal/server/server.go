// Package server handles the HTTP server and WebSocket connections.
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	appwebrtc "github.com/CyTechNomad/PipeDream/internal/webrtc"
)

// Server is the HTTP server that serves static files and handles WebRTC signaling.
type Server struct {
	addr          string
	webrtcHandler *appwebrtc.Handler
	youtubeAPIKey string
	httpServer    *http.Server
	upgrader      websocket.Upgrader
}

// New creates a new Server.
func New(addr string, webrtcHandler *appwebrtc.Handler, youtubeAPIKey string) *Server {
	return &Server{
		addr:          addr,
		webrtcHandler: webrtcHandler,
		youtubeAPIKey: youtubeAPIKey,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
	}
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Serve static files
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	// Handle WebRTC connections
	mux.HandleFunc("/rtc", s.handleRTC)

	// API key endpoint
	mux.HandleFunc("/api/youtube-key", s.handleYouTubeKey)

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	// Start the server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Println("Starting web server on", s.addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		return s.Shutdown()
	case err := <-errChan:
		return err
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	log.Println("Shutting down web server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(shutdownCtx)
}

func (s *Server) handleRTC(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	s.webrtcHandler.HandleConnection(conn)
}

func (s *Server) handleYouTubeKey(w http.ResponseWriter, r *http.Request) {
	if s.youtubeAPIKey == "" {
		http.Error(w, "YouTube API key not configured", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"key": s.youtubeAPIKey})
}
