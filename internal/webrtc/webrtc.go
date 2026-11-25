// Package webrtc handles WebRTC peer connections and audio forwarding.
package webrtc

import (
	"encoding/json"
	"io"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

// AudioSender is an interface for sending Opus audio data.
type AudioSender interface {
	SendOpus(data []byte, timeout time.Duration) bool
	IsReady() bool
}

// Handler manages WebRTC connections and audio forwarding.
type Handler struct {
	audioSender AudioSender
	config      webrtc.Configuration
}

// NewHandler creates a new WebRTC handler.
func NewHandler(audioSender AudioSender) *Handler {
	return &Handler{
		audioSender: audioSender,
		config: webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{
					URLs: []string{"stun:stun.l.google.com:19302"},
				},
			},
		},
	}
}

// HandleConnection handles a WebSocket connection for WebRTC signaling.
func (h *Handler) HandleConnection(conn *websocket.Conn) {
	defer conn.Close()

	log.Println("WebSocket connection established")

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(h.config)
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
		h.handleTrack(track)
	})

	// Create a channel for signaling
	done := make(chan struct{})

	// Handle incoming messages
	go h.handleMessages(conn, peerConnection, done)

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

		if err = conn.WriteMessage(websocket.TextMessage, candidateJSON); err != nil {
			log.Printf("Error sending ICE candidate: %v", err)
		} else {
			log.Printf("ICE candidate sent successfully, length: %d bytes", len(candidateJSON))
		}
	})

	// Wait until done
	<-done
}

func (h *Handler) handleTrack(track *webrtc.TrackRemote) {
	log.Printf("Received track: %s", track.Codec().MimeType)

	if !h.audioSender.IsReady() {
		log.Println("Discord voice connection not established yet")
		return
	}

	// Check if the track is Opus audio, which Discord requires
	if track.Codec().MimeType != webrtc.MimeTypeOpus {
		log.Printf("Received non-Opus track: %s, cannot forward to Discord", track.Codec().MimeType)
		return
	}

	log.Println("Forwarding Opus audio to Discord")

	// Use a buffered channel to process packets more reliably
	opusChan := make(chan []byte, 100)

	// Start a separate goroutine to send to Discord
	go func() {
		for payload := range opusChan {
			if !h.audioSender.IsReady() {
				log.Println("Discord voice connection not ready")
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if !h.audioSender.SendOpus(payload, 100*time.Millisecond) {
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
			continue // Skip invalid frames silently
		}

		// Send to our processing channel
		select {
		case opusChan <- opusPayload:
			// Successfully sent for processing
		default:
			// Channel buffer full, skip this frame
		}
	}
}

func (h *Handler) handleMessages(conn *websocket.Conn, peerConnection *webrtc.PeerConnection, done chan struct{}) {
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

		if err == nil && offer.Type == webrtc.SDPTypeOffer {
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
}
