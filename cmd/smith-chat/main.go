package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"smith/internal/chat"
	"smith/internal/chat/goosed"
	"smith/internal/chat/prompts"
	"smith/internal/chat/sessions"
	"smith/internal/chat/smithbridge"
	"smith/internal/source/store"
)

type config struct {
	port          int
	etcdEndpoints []string
}

type server struct {
	cfg      config
	sessions *sessions.Manager
	engine   chat.Engine
	prompts  *prompts.Manager
	bridge   smithbridge.Bridge
}

func main() {
	var (
		port          = flag.Int("port", 8081, "Listen port")
		etcdEndpoints = flag.String("etcd-endpoints", "http://127.0.0.1:2379", "etcd endpoints")
	)
	flag.Parse()

	cfg := config{
		port:          *port,
		etcdEndpoints: strings.Split(*etcdEndpoints, ","),
	}

	// Initialize etcd store
	etcdStore, err := store.New(context.Background(), cfg.etcdEndpoints, 5*time.Second)
	if err != nil {
		log.Fatalf("failed to connect to etcd: %v", err)
	}

	bridge := smithbridge.NewEtcdBridge(etcdStore)

	s := &server{
		cfg:      cfg,
		sessions: sessions.NewManager(),
		engine:   goosed.NewEngine(),
		prompts:  prompts.NewManager(bridge),
		bridge:   bridge,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/sessions", s.handleSessions)
	mux.HandleFunc("/v1/chat/sessions/", s.handleSessionByID)
	mux.HandleFunc("/v1/chat/actions/commit", s.handleCommit)
	mux.HandleFunc("/v1/chat/ui/resolve", s.handleUIResolve)

	// Add CORS for development
	handler := corsMiddleware(mux)

	addr := fmt.Sprintf(":%d", cfg.port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("smith-chat listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("smith-chat failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("smith-chat shutdown requested")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("smith-chat shutdown failed: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Type    chat.SessionType  `json:"type"`
		Context map[string]string `json:"context"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	session, err := s.sessions.CreateSession(req.Type, req.Context)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"sessionId": session.ID})
}

func (s *server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/chat/sessions/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		http.Error(w, "session id required", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]
	session, ok := s.sessions.GetSession(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if len(parts) == 1 {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(session)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	switch parts[1] {
	case "messages":
		s.handleMessages(w, r, session)
	case "stream":
		s.handleStream(w, r, session)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (s *server) handleMessages(w http.ResponseWriter, r *http.Request, session *chat.Session) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	msg := chat.Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Role:      chat.RoleUser,
		Content:   req.Message,
		Timestamp: time.Now(),
	}

	if err := s.sessions.AddMessage(session.ID, msg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *server) handleStream(w http.ResponseWriter, r *http.Request, session *chat.Session) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event chat.EventType, payload any) error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	// Build system prompt with context
	systemPrompt, err := s.prompts.BuildSystemPrompt(r.Context(), session)
	if err != nil {
		log.Printf("failed to build system prompt: %v", err)
	}

	// Prepend system message to temporary copy of session for engine
	// In a real scenario, we might want to store this in the session
	engineSession := *session
	engineSession.Messages = append([]chat.Message{{
		ID:      "msg_system",
		Role:    chat.RoleSystem,
		Content: systemPrompt,
	}}, engineSession.Messages...)

	var lastUserMsg string
	if len(session.Messages) > 0 {
		for i := len(session.Messages) - 1; i >= 0; i-- {
			if session.Messages[i].Role == chat.RoleUser {
				lastUserMsg = session.Messages[i].Content
				break
			}
		}
	}

	if lastUserMsg == "" {
		_ = send(chat.EventError, map[string]string{"error": "no message to process"})
		return
	}

	events := make(chan chat.ChatEvent)
	go func() {
		defer close(events)
		if err := s.engine.Stream(r.Context(), &engineSession, lastUserMsg, events); err != nil {
			log.Printf("engine stream error: %v", err)
		}
	}()

	var fullResponse strings.Builder
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				assistantMsg := chat.Message{
					ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
					Role:      chat.RoleAssistant,
					Content:   fullResponse.String(),
					Timestamp: time.Now(),
				}
				_ = s.sessions.AddMessage(session.ID, assistantMsg)
				return
			}
			if ev.Event == chat.EventMessageDelta {
				if delta, ok := ev.Data.(chat.MessageDelta); ok {
					fullResponse.WriteString(delta.Delta)
				} else if m, ok := ev.Data.(map[string]any); ok {
					if d, ok := m["delta"].(string); ok {
						fullResponse.WriteString(d)
					}
				}
			}
			_ = send(ev.Event, ev.Data)
		}
	}
}
func (s *server) handleCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action  string         `json:"action"`
		Payload map[string]any `json:"payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	switch req.Action {
	case "create-loop":
		s.proxyCreateLoop(w, r, req.Payload)
	default:
		http.Error(w, fmt.Sprintf("unsupported action: %s", req.Action), http.StatusBadRequest)
	}
}

func (s *server) proxyCreateLoop(w http.ResponseWriter, r *http.Request, payload map[string]any) {
	apiURL := "http://localhost:8080/v1/loops"

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	// Copy authorization if present
	if auth := r.Header.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (s *server) handleUIResolve(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
