package wgbind

import (
	"net/netip"
	"sync/atomic"
	"time"
)

// PeerTransportStats are counters for one peer VPN address on the ICE bind.
type PeerTransportStats struct {
	VPNIP              netip.Addr
	HasActiveConn      bool
	SendPackets        uint64
	SendBytes          uint64
	SendNoTransport    uint64
	SendErrors         uint64
	ConsecutiveErrors  int
	LastSendError      string
	LastSendErrorAt    time.Time
	LastSendOKAt       time.Time
	LastRecvAt         time.Time
	RecvPackets        uint64
	RecvBytes          uint64
	RecvDropped        uint64
	ReadLoopExits      uint64
}

// BindStats aggregates ICE bind diagnostics (safe for concurrent read).
type BindStats struct {
	Shutdown bool
	Open     bool
	Peers    []PeerTransportStats
}

type peerCounters struct {
	sendPackets     atomic.Uint64
	sendBytes       atomic.Uint64
	sendNoTransport atomic.Uint64
	sendErrors      atomic.Uint64
	recvPackets     atomic.Uint64
	recvBytes       atomic.Uint64
	recvDropped     atomic.Uint64
	readLoopExits   atomic.Uint64
	health          sendHealth
}
