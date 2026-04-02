package ws

import (
	"sync"

	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
)

type Hub struct {
	mu      sync.Mutex
	clients map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*Client]struct{})}
}

func (h *Hub) Register(client *Client) {
	if h == nil || client == nil {
		return
	}

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) Unregister(client *Client) {
	if h == nil || client == nil {
		return
	}

	h.mu.Lock()
	if _, exists := h.clients[client]; exists {
		delete(h.clients, client)
		client.closeSend()
	}
	h.mu.Unlock()
}

func (h *Hub) Broadcast(event service.ProgressEvent) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		select {
		case client.send <- event:
		default:
			delete(h.clients, client)
			client.closeSend()
		}
	}
}
