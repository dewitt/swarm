package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sync"

	"github.com/dewitt/swarm/pkg/sdk"
)

//go:embed static/*
var staticFiles embed.FS

// Server handles Server-Sent Events (SSE) for broadcasting Agent telemetry to browsers.
type Server struct {
	addr        string
	clients     map[chan sdk.ObservableEvent]bool
	clientsMu   sync.RWMutex
	eventStream chan sdk.ObservableEvent
}

// NewServer creates a new SSE Server on the given address.
func NewServer(addr string) *Server {
	return &Server{
		addr:        addr,
		clients:     make(map[chan sdk.ObservableEvent]bool),
		eventStream: make(chan sdk.ObservableEvent, 100),
	}
}

// Broadcast pushes an ObservableEvent to all connected browser clients.
func (s *Server) Broadcast(event sdk.ObservableEvent) {
	// Push to our local router channel
	select {
	case s.eventStream <- event:
	default:
		// Drop event if router channel is full to prevent blocking the SDK
	}
}

// Start launches the background HTTP server and the message router loop.
func (s *Server) Start() error {
	go s.router()

	mux := http.NewServeMux()

	importFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}

	// Serve static files (index.html)
	mux.Handle("/", http.FileServer(http.FS(importFS)))

	// Default route goes to index.html if user navigates manually
	mux.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		index, _ := staticFiles.ReadFile("static/index.html")
		w.Header().Set("Content-Type", "text/html")
		w.Write(index)
	})

	// SSE Endpoint
	mux.HandleFunc("/events", s.handleSSE)

	srv := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	return srv.ListenAndServe()
}

// router handles fan-out to all connected browser tabs.
func (s *Server) router() {
	for event := range s.eventStream {
		s.clientsMu.RLock()
		for clientChan := range s.clients {
			select {
			case clientChan <- event:
			default:
				// If a client's specific buffer is full, drop just for them
			}
		}
		s.clientsMu.RUnlock()
	}
}

// handleSSE upgrades the HTTP connection to a persistent event stream.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE compatibility
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Allow CORS just in case
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a unique channel for this client connection
	clientChan := make(chan sdk.ObservableEvent, 50)

	s.clientsMu.Lock()
	s.clients[clientChan] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, clientChan)
		s.clientsMu.Unlock()
		close(clientChan)
	}()

	// Flush immediately to establish connection
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Wait for events to broadcast or connection to drop
	for {
		select {
		case <-r.Context().Done():
			return // Client disconnected
		case event := <-clientChan:
			eventJSON, err := json.Marshal(event)
			if err != nil {
				continue
			}

			// SSE format requires "data: <payload>\n\n"
			fmt.Fprintf(w, "data: %s\n\n", string(eventJSON))

			// Flush data to network boundary immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}
