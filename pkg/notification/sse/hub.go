package sse

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/notification/model"
)

// Client represents a connected SSE client
type Client struct {
	ID       string
	ClinicID uuid.UUID
	UserID   uuid.UUID
	Channel  chan []byte
}

// Hub manages SSE connections and broadcasts notifications
type Hub struct {
	// clients maps clinic_id -> user_id -> client
	clients map[uuid.UUID]map[uuid.UUID]*Client
	mu      sync.RWMutex

	// Channels for managing clients
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
}

// BroadcastMessage represents a message to broadcast
type BroadcastMessage struct {
	ClinicID     uuid.UUID
	UserID       *uuid.UUID // nil = broadcast to all users in clinic
	Notification *model.NotificationResponse
}

// NewHub creates a new SSE hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]map[uuid.UUID]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage, 256),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.addClient(client)
		case client := <-h.unregister:
			h.removeClient(client)
		case msg := <-h.broadcast:
			h.broadcastMessage(msg)
		}
	}
}

// Register adds a new client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast sends a notification to relevant clients
func (h *Hub) Broadcast(clinicID uuid.UUID, userID *uuid.UUID, notification *model.NotificationResponse) {
	h.broadcast <- &BroadcastMessage{
		ClinicID:     clinicID,
		UserID:       userID,
		Notification: notification,
	}
}

func (h *Hub) addClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.ClinicID]; !ok {
		h.clients[client.ClinicID] = make(map[uuid.UUID]*Client)
	}

	// If there's an existing connection, just replace it (don't close the channel)
	// The old connection will close naturally when its context is done
	h.clients[client.ClinicID][client.UserID] = client
}

func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clinicClients, ok := h.clients[client.ClinicID]; ok {
		if existing, ok := clinicClients[client.UserID]; ok && existing.ID == client.ID {
			close(existing.Channel)
			delete(clinicClients, client.UserID)
		}
	}
}

func (h *Hub) broadcastMessage(msg *BroadcastMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clinicClients, ok := h.clients[msg.ClinicID]
	if !ok {
		return
	}

	data, err := json.Marshal(msg.Notification)
	if err != nil {
		return
	}

	sseData := []byte(fmt.Sprintf("event: notification\ndata: %s\n\n", data))

	if msg.UserID != nil {
		// Send to specific user
		if client, ok := clinicClients[*msg.UserID]; ok {
			select {
			case client.Channel <- sseData:
			default:
				// Channel full, skip
			}
		}
	} else {
		// Broadcast to all users in clinic
		for _, client := range clinicClients {
			select {
			case client.Channel <- sseData:
			default:
				// Channel full, skip
			}
		}
	}
}

// GetClientCount returns the number of connected clients (for debugging)
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, clinicClients := range h.clients {
		count += len(clinicClients)
	}
	return count
}
