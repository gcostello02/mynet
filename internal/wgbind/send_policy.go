package wgbind

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pion/ice/v4"
)

const consecutiveSendFailureThreshold = 5

func isTerminalSendError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ice.ErrNoCandidatePairs) {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "closed") {
		return true
	}
	return false
}

type sendHealth struct {
	mu                 sync.Mutex
	consecutiveErrors  int
	lastError          string
	lastErrorAt        time.Time
	lastSendOK         time.Time
	lastRecvAt         time.Time
}

func (h *sendHealth) recordSendOK() {
	h.mu.Lock()
	h.consecutiveErrors = 0
	h.lastSendOK = time.Now()
	h.mu.Unlock()
}

func (h *sendHealth) recordRecv() {
	h.mu.Lock()
	h.lastRecvAt = time.Now()
	h.mu.Unlock()
}

func (h *sendHealth) recordSendErr(err error) (terminal bool, consecutive int) {
	h.mu.Lock()
	h.lastError = err.Error()
	h.lastErrorAt = time.Now()
	if isTerminalSendError(err) {
		h.consecutiveErrors = consecutiveSendFailureThreshold
	} else {
		h.consecutiveErrors++
	}
	consecutive = h.consecutiveErrors
	terminal = isTerminalSendError(err) || consecutive >= consecutiveSendFailureThreshold
	h.mu.Unlock()
	return terminal, consecutive
}

func (h *sendHealth) snapshot() (lastErr string, lastErrAt time.Time, lastSendOK time.Time, lastRecv time.Time, consecutive int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastError, h.lastErrorAt, h.lastSendOK, h.lastRecvAt, h.consecutiveErrors
}
