package client

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// SignalingSnapshot is daemon-safe signaling state for status/diagnostics.
type SignalingSnapshot struct {
	State                string // Connected | Connecting | Disconnected
	LastDisconnectReason string
	ReconnectAttempts    int
	LastMessageType      string
	LastMessageAt        time.Time
}

func (r *Runtime) SignalingSnapshot() SignalingSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.signalingSnapshotLocked()
}

func (r *Runtime) signalingSnapshotLocked() SignalingSnapshot {
	snap := SignalingSnapshot{State: "Disconnected"}
	if r.ws != nil {
		snap.State = "Connected"
	} else if r.signalingConnecting {
		snap.State = "Connecting"
	}
	snap.LastDisconnectReason = r.lastWSDisconnect
	snap.ReconnectAttempts = r.reconnectAttempts
	snap.LastMessageType = r.lastWSMessageType
	snap.LastMessageAt = r.lastWSMessageAt
	return snap
}

// StartSignaling maintains an authenticated WebSocket until ctx is cancelled.
func (r *Runtime) StartSignaling(ctx context.Context) {
	r.signalingMu.Lock()
	if r.signalingRunning {
		r.signalingMu.Unlock()
		return
	}
	r.signalingRunning = true
	r.signalingMu.Unlock()

	go r.signalingLoop(ctx)
}

func (r *Runtime) signalingLoop(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		if ctx.Err() != nil {
			break
		}
		if err := r.load(); err != nil || r.api == nil || r.api.Token == "" {
			r.mu.Lock()
			r.lastWSDisconnect = "not authenticated"
			r.mu.Unlock()
			select {
			case <-ctx.Done():
			case <-time.After(backoff):
			}
			continue
		}

		r.mu.Lock()
		r.signalingConnecting = true
		r.mu.Unlock()

		err := r.runOneSignalingSession(ctx)
		r.mu.Lock()
		r.signalingConnecting = false
		if err != nil && ctx.Err() == nil {
			r.lastWSDisconnect = err.Error()
			r.reconnectAttempts++
		}
		r.mu.Unlock()

		if ctx.Err() != nil {
			break
		}
		jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
		sleep := backoff + jitter
		if sleep > maxBackoff {
			sleep = maxBackoff
		}
		select {
		case <-ctx.Done():
			break
		case <-time.After(sleep):
		}
		if backoff < maxBackoff {
			backoff *= 2
		}
	}

	r.signalingMu.Lock()
	r.signalingRunning = false
	r.signalingMu.Unlock()
}

func (r *Runtime) runOneSignalingSession(ctx context.Context) error {
	conn, err := r.api.ConnectWebSocket()
	if err != nil {
		return err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	r.replaceSignalingWebSocket(conn, cancel)

	defer func() {
		cancel()
		r.clearSignalingWebSocket(conn)
		_ = conn.Close()
	}()

	done := make(chan struct{})
	go func() {
		r.wsReadLoop(sessionCtx, conn)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return errors.New("websocket closed")
	}
}

// WaitForSignaling blocks until the persistent signaling socket is up or timeout.
func (r *Runtime) WaitForSignaling(ctx context.Context, timeout time.Duration) error {
	sub, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return r.waitForSignaling(sub)
}

func (r *Runtime) waitForSignaling(ctx context.Context) error {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		r.mu.Lock()
		ok := r.ws != nil
		r.mu.Unlock()
		if ok {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("signaling websocket not connected")
}

func (r *Runtime) noteWSMessage(msgType string) {
	r.mu.Lock()
	r.lastWSMessageType = msgType
	r.lastWSMessageAt = time.Now()
	r.mu.Unlock()
}
