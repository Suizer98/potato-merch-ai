package server

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	adksession "google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	chatv1 "github.com/Suizer98/potato-merch-ai/gen/go/chat/v1"
	storeagent "github.com/Suizer98/potato-merch-ai/internal/agent"
)

type ChatServer struct {
	chatv1.UnimplementedChatServiceServer
	run      *runner.Runner
	sessions *sessionIndex
}

type sessionIndex struct {
	mu    sync.RWMutex
	items map[string]sessionMeta
}

type sessionMeta struct {
	ID        string
	Title     string
	UpdatedAt time.Time
}

func NewChatServer(root agent.Agent) (*ChatServer, error) {
	run, err := runner.New(runner.Config{
		AppName:           storeagent.AppName,
		Agent:             root,
		SessionService:    adksession.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, err
	}
	return &ChatServer{
		run:      run,
		sessions: &sessionIndex{items: map[string]sessionMeta{}},
	}, nil
}

func (s *ChatServer) Chat(req *chatv1.ChatRequest, stream chatv1.ChatService_ChatServer) error {
	return s.streamChat(stream.Context(), req, func(chunk *chatv1.ChatChunk) error {
		return stream.Send(chunk)
	})
}

func (s *ChatServer) streamChat(ctx context.Context, req *chatv1.ChatRequest, send func(*chatv1.ChatChunk) error) error {
	sessionID := strings.TrimSpace(req.GetSessionId())
	message := strings.TrimSpace(req.GetMessage())
	if message == "" {
		return send(&chatv1.ChatChunk{
			SessionId: sessionID,
			Error:     ptr("message is required"),
			Done:      true,
		})
	}
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	s.sessions.touch(sessionID, message)

	msg := genai.NewContentFromText(message, genai.RoleUser)
	for event, err := range s.run.Run(ctx, storeagent.DefaultUserID, sessionID, msg, agent.RunConfig{}) {
		if err != nil {
			return send(&chatv1.ChatChunk{
				SessionId: sessionID,
				Error:     ptr(publicError(err)),
				Done:      true,
			})
		}
		chunk := chunkFromEvent(sessionID, event)
		if chunk == nil {
			continue
		}
		if err := send(chunk); err != nil {
			return err
		}
	}

	return send(&chatv1.ChatChunk{
		SessionId: sessionID,
		Done:      true,
	})
}

func (s *ChatServer) ListSessions(ctx context.Context, _ *chatv1.ListSessionsRequest) (*chatv1.ListSessionsResponse, error) {
	items := s.sessions.list()
	out := &chatv1.ListSessionsResponse{
		Sessions: make([]*chatv1.Session, 0, len(items)),
	}
	for _, item := range items {
		out.Sessions = append(out.Sessions, &chatv1.Session{
			Id:            item.ID,
			Title:         item.Title,
			UpdatedAtUnix: item.UpdatedAt.Unix(),
		})
	}
	return out, nil
}

func chunkFromEvent(sessionID string, event *adksession.Event) *chatv1.ChatChunk {
	if event == nil {
		return nil
	}
	agentID := publicAgentID(event.Author)
	if event.RequestedInput != nil {
		return &chatv1.ChatChunk{
			SessionId: sessionID,
			AgentId:   agentID,
			Event:     "hitl",
			Delta:     event.RequestedInput.Message,
		}
	}
	if dest := publicAgentID(event.Actions.TransferToAgent); dest != "" {
		if status := handoffStatus(dest, eventOutputText(event)); status != "" {
			return &chatv1.ChatChunk{
				SessionId: sessionID,
				AgentId:   dest,
				Event:     "handoff",
				Delta:     status,
			}
		}
	}
	if len(event.Routes) == 1 {
		if dest := publicAgentID(event.Routes[0]); dest != "" {
			if status := handoffStatus(dest, eventOutputText(event)); status != "" {
				return &chatv1.ChatChunk{
					SessionId: sessionID,
					AgentId:   dest,
					Event:     "handoff",
					Delta:     status,
				}
			}
		}
	}
	if event.Content == nil {
		return nil
	}
	var text strings.Builder
	var toolName string
	for _, part := range event.Content.Parts {
		if part == nil {
			continue
		}
		if part.FunctionCall != nil && part.FunctionCall.Name != "" {
			toolName = part.FunctionCall.Name
			continue
		}
		if part.FunctionResponse != nil {
			continue
		}
		if part.Text != "" {
			text.WriteString(part.Text)
		}
	}
	if toolName != "" {
		status := friendlyToolStatus(toolName)
		if status == "" {
			status = friendlyToolStatus(agentID)
		}
		if status == "" {
			return nil
		}
		return &chatv1.ChatChunk{
			SessionId: sessionID,
			AgentId:   agentIDForTool(toolName, agentID),
			Event:     "tool",
			Delta:     status,
		}
	}
	trimmed := strings.TrimSpace(text.String())
	if trimmed == "" || isRawToolName(trimmed) {
		return nil
	}
	return &chatv1.ChatChunk{
		SessionId: sessionID,
		AgentId:   agentID,
		Delta:     text.String(),
	}
}

func publicAgentID(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case storeagent.RootAgentName, storeagent.ShopAgentName, storeagent.BillingAgentName, storeagent.SupportAgentName:
		return strings.ToLower(strings.TrimSpace(name))
	default:
		return ""
	}
}

func isRawToolName(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "shop", "billing", "support", "potato", "transfer_to_agent", "transfertoagent":
		return true
	default:
		return false
	}
}

func agentIDForTool(toolName, fallback string) string {
	if id := publicAgentID(toolName); id != "" {
		return id
	}
	return fallback
}

func eventOutputText(event *adksession.Event) string {
	if event == nil {
		return ""
	}
	switch typed := event.Output.(type) {
	case string:
		return typed
	case *string:
		if typed != nil {
			return *typed
		}
	}
	return ""
}

func handoffStatus(agentID, userText string) string {
	if strings.EqualFold(agentID, storeagent.BillingAgentName) && !storeagent.HasOrderNumber(userText) {
		return ""
	}
	return friendlyToolStatus(agentID)
}

func friendlyToolStatus(name string) string {
	switch strings.ToLower(name) {
	case storeagent.ShopAgentName:
		return "Checking the catalog…"
	case storeagent.BillingAgentName:
		return "Looking up your order…"
	case storeagent.SupportAgentName:
		return "Checking with support…"
	default:
		return ""
	}
}

func publicError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "failed to find agent") {
		return "I can help with tees, orders, or shipping. What do you need?"
	}
	return msg
}

func (s *sessionIndex) touch(id, firstMessage string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		title := firstMessage
		if len(title) > 48 {
			title = title[:48]
		}
		item = sessionMeta{ID: id, Title: title}
	}
	item.UpdatedAt = time.Now().UTC()
	s.items[id] = item
}

func (s *sessionIndex) list() []sessionMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]sessionMeta, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	return out
}

func ptr(value string) *string {
	return &value
}
