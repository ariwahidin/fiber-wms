// realtime/hub.go
package realtime

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/contrib/websocket"
)

type Event struct {
	Type    string      `json:"type"` // e.g. "outbound.packing_confirmed"
	Payload interface{} `json:"payload"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

var GlobalHub = &Hub{
	clients: make(map[*websocket.Conn]bool),
}

func (h *Hub) Register(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = true
}

func (h *Hub) Unregister(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, conn)
	conn.Close()
}

func (h *Hub) Broadcast(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			// koneksi mati, biar dibersihin di read loop masing-masing conn
			continue
		}
	}
}
