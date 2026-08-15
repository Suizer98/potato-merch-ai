package server

import (
	"encoding/json"
	"log"
	"net/http"

	chatv1 "github.com/Suizer98/potato-merch-ai/gen/go/chat/v1"
)

type httpChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	Model     string `json:"model,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
}

func (s *ChatServer) ServeHTTP(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /v1/chat", s.handleHTTPChat)
	log.Printf("chat HTTP listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("chat HTTP: %v", err)
	}
}

func (s *ChatServer) handleHTTPChat(w http.ResponseWriter, r *http.Request) {
	var body httpChatRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming unsupported"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	req := &chatv1.ChatRequest{
		SessionId: body.SessionID,
		Message:   body.Message,
	}
	if body.Model != "" {
		req.Model = &body.Model
	}
	if body.AgentID != "" {
		req.AgentId = &body.AgentID
	}

	err := s.streamChat(r.Context(), req, func(chunk *chatv1.ChatChunk) error {
		payload, err := json.Marshal(map[string]any{
			"session_id": chunk.GetSessionId(),
			"delta":      chunk.GetDelta(),
			"done":       chunk.GetDone(),
			"error":      chunk.GetError(),
			"agent_id":   chunk.GetAgentId(),
			"event":      chunk.GetEvent(),
		})
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte("data: ")); err != nil {
			return err
		}
		if _, err := w.Write(payload); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n\n")); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
	if err != nil && r.Context().Err() == nil {
		log.Printf("http chat: %v", err)
	}
}
