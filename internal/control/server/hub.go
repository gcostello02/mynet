package server

import (
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/grantcostello/mynet/internal/control"
)

type wsClient struct {
	id       uint64
	deviceID string
	conn     *websocket.Conn
	send     chan []byte
}

// Hub routes signaling messages between connected devices.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*wsClient
	log     *slog.Logger
	nextID  atomic.Uint64
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[string]*wsClient),
		log:     log,
	}
}

// Register adds a signaling connection for deviceID, replacing any prior connection.
// Returns a connection id used for safe Unregister on close.
func (h *Hub) Register(deviceID string, conn *websocket.Conn) uint64 {
	id := h.nextID.Add(1)
	c := &wsClient{
		id:       id,
		deviceID: deviceID,
		conn:     conn,
		send:     make(chan []byte, 32),
	}
	h.mu.Lock()
	if old, ok := h.clients[deviceID]; ok {
		close(old.send)
		_ = old.conn.Close()
	}
	h.clients[deviceID] = c
	h.mu.Unlock()

	go h.writePump(c)
	return id
}

// Unregister removes presence only if connID is still the active connection for deviceID.
func (h *Hub) Unregister(deviceID string, connID uint64) {
	h.mu.Lock()
	c, ok := h.clients[deviceID]
	if ok && c.id == connID {
		close(c.send)
		delete(h.clients, deviceID)
	}
	h.mu.Unlock()
}

func (h *Hub) IsOnline(deviceID string) bool {
	h.mu.RLock()
	_, ok := h.clients[deviceID]
	h.mu.RUnlock()
	return ok
}

func (h *Hub) writePump(c *wsClient) {
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			h.log.Debug("websocket write failed", "device", c.deviceID, "err", err)
			return
		}
	}
}

func (h *Hub) SendTo(deviceID string, msg control.WSMessage) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	h.mu.RLock()
	c, ok := h.clients[deviceID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case c.send <- data:
		return true
	default:
		h.log.Warn("signaling queue full", "device", deviceID)
		return false
	}
}

// RelaySignal forwards a validated signaling message to the peer in the same session.
func (h *Hub) RelaySignal(fromDevice, sessionID string, msg control.WSMessage, store *Store) error {
	sess, ok := store.GetSession(sessionID)
	if !ok {
		return errSessionNotFound
	}
	var peer string
	switch fromDevice {
	case sess.Initiator:
		peer = sess.Responder
	case sess.Responder:
		peer = sess.Initiator
	default:
		return errUnauthorizedSession
	}
	msg.SessionID = sessionID
	if !h.SendTo(peer, msg) {
		return errPeerOffline
	}
	return nil
}

var (
	errSessionNotFound     = &hubError{"session not found"}
	errUnauthorizedSession = &hubError{"not a session participant"}
	errPeerOffline         = &hubError{"peer not connected for signaling"}
)

type hubError struct{ msg string }

func (e *hubError) Error() string { return e.msg }
