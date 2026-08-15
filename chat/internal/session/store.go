package session

import (
	"sync"
	"time"

	"github.com/Suizer98/potato-merch-ai/internal/llm"
)

type Session struct {
	ID        string
	Title     string
	Messages  []llm.Message
	UpdatedAt time.Time
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
	}
}

func (s *Store) GetOrCreate(id, firstMessage string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.sessions[id]; ok {
		return existing
	}

	title := firstMessage
	if len(title) > 48 {
		title = title[:48]
	}
	session := &Session{
		ID:        id,
		Title:     title,
		Messages:  nil,
		UpdatedAt: time.Now().UTC(),
	}
	s.sessions[id] = session
	return session
}

func (s *Store) Append(id string, messages ...llm.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return
	}
	session.Messages = append(session.Messages, messages...)
	session.UpdatedAt = time.Now().UTC()
}

func (s *Store) Messages(id string) []llm.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil
	}
	copied := make([]llm.Message, len(session.Messages))
	copy(copied, session.Messages)
	return copied
}

func (s *Store) List() []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		out = append(out, Session{
			ID:        session.ID,
			Title:     session.Title,
			UpdatedAt: session.UpdatedAt,
		})
	}
	return out
}
