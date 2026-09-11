package ws

import (
	"errors"
	"sync"
	"time"

	"log/slog"

	"github.com/coder/websocket"
)

type Hub struct {
	mu       sync.RWMutex
	sessions map[string]*AgentSession
}

// NodeInfo is a lightweight, serializable view of a connected agent used by
// the administration console to render the client roster.
type NodeInfo struct {
	ID        string    `json:"id"`
	Connected time.Time `json:"connected"`
}

func NewHub() *Hub {
	return &Hub{
		sessions: make(map[string]*AgentSession),
	}
}

func (h *Hub) Register(id string, conn *websocket.Conn) *AgentSession {
	session := NewAgentSession(id, conn)

	h.mu.Lock()
	h.sessions[id] = session
	h.mu.Unlock()

	slog.Info("registered node in hub", "node_id", id, "total_sessions", len(h.sessions))
	return session
}

func (h *Hub) Deregister(id string) {
	h.mu.Lock()
	session, exists := h.sessions[id]
	if exists {
		delete(h.sessions, id)
	}
	h.mu.Unlock()

	if exists {
		slog.Info("deregistered node from hub", "node_id", id, "total_sessions", len(h.sessions)-1)
		session.Close("session terminated by server")
	}
}

func (h *Hub) GetSession(id string) (*AgentSession, error) {
	h.mu.RLock()
	session, exists := h.sessions[id]
	h.mu.RUnlock()

	if !exists {
		return nil, errors.New("requested endpoint agent is currently offline")
	}
	return session, nil
}

// List returns a snapshot of all currently connected agent nodes.
func (h *Hub) List() []NodeInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	slog.Info("listing connected nodes", "total_sessions", len(h.sessions))
	infos := make([]NodeInfo, 0, len(h.sessions))
	for id, session := range h.sessions {
		infos = append(infos, NodeInfo{ID: id, Connected: session.Connected})
	}
	return infos
}

func (h *Hub) CloseAll(reason string) {
	h.mu.RLock()
	sessions := make([]*AgentSession, 0, len(h.sessions))
	for _, session := range h.sessions {
		sessions = append(sessions, session)
	}
	h.mu.RUnlock()

	for _, session := range sessions {
		session.Close(reason)
	}
}
