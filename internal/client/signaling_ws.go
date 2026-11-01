package client

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
)

// replaceSignalingWebSocket installs conn as the sole signaling socket (serialized with writers).
func (r *Runtime) replaceSignalingWebSocket(conn *websocket.Conn, cancel context.CancelFunc) {
	r.wsWriteMu.Lock()
	if r.ws != nil {
		_ = r.ws.Close()
		if r.wsCancel != nil {
			r.wsCancel()
		}
	}
	r.mu.Lock()
	r.ws = conn
	r.wsGen++
	r.wsCancel = cancel
	r.connState = StateOnline
	r.reconnectAttempts = 0
	r.lastWSDisconnect = ""
	r.signalingReadyAt = time.Now()
	r.mu.Unlock()
	r.wsWriteMu.Unlock()
}

// clearSignalingWebSocket clears conn if it is still the active signaling socket.
func (r *Runtime) clearSignalingWebSocket(conn *websocket.Conn) {
	r.wsWriteMu.Lock()
	r.mu.Lock()
	if r.ws == conn {
		r.ws = nil
		r.wsGen++
		if r.wsCancel != nil {
			r.wsCancel()
			r.wsCancel = nil
		}
		if r.dataConn == nil && (r.connState == StateOnline || r.connState == StateConnecting) {
			r.connState = StateRegistered
		}
	}
	r.mu.Unlock()
	r.wsWriteMu.Unlock()
}

// closeSignalingWebSocket shuts down the active signaling connection (daemon shutdown).
func (r *Runtime) closeSignalingWebSocket() {
	r.wsWriteMu.Lock()
	if r.wsCancel != nil {
		r.wsCancel()
		r.wsCancel = nil
	}
	if r.ws != nil {
		_ = r.ws.Close()
		r.ws = nil
		r.wsGen++
	}
	r.mu.Lock()
	if r.connState == StateOnline || r.connState == StateConnecting {
		r.connState = StateRegistered
	}
	r.mu.Unlock()
	r.wsWriteMu.Unlock()
}
