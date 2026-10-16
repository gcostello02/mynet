package wgbind

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
)

// ErrNoTransport is returned when WireGuard sends before an ICE conn is attached.
var ErrNoTransport = errors.New("no ice transport for peer")

const defaultWGPort = 51820
const maxWGPacket = 65535

// PeerEndpoint is the logical WireGuard endpoint (VPN IP + fixed port).
type PeerEndpoint struct {
	VPNIP netip.Addr
}

var (
	_ conn.Bind     = (*ICEBind)(nil)
	_ conn.Endpoint = (*PeerEndpoint)(nil)
)

func (e *PeerEndpoint) ClearSrc()           {}
func (e *PeerEndpoint) SrcToString() string { return "" }
func (e *PeerEndpoint) DstToString() string {
	return fmt.Sprintf("%s:%d", e.VPNIP, defaultWGPort)
}
func (e *PeerEndpoint) DstToBytes() []byte {
	ap := netip.AddrPortFrom(e.VPNIP, defaultWGPort)
	b, _ := ap.MarshalBinary()
	return b
}
func (e *PeerEndpoint) DstIP() netip.Addr { return e.VPNIP }
func (e *PeerEndpoint) SrcIP() netip.Addr { return netip.Addr{} }

type peerTransport struct {
	conn      net.Conn
	closed    chan struct{} // closed when readLoop exits (conn dead or replaced)
	counters  *peerCounters
	closeOnce sync.Once
}

// ICEBind sends WireGuard datagrams over nominated ice.Conn instances.
type ICEBind struct {
	mu          sync.RWMutex
	byIP        map[netip.Addr]*peerTransport
	byConn      map[net.Conn]netip.Addr
	inbound     chan inboundPkt
	closeSignal chan struct{}
	shutdown    bool
	open        bool
	recvFns     []conn.ReceiveFunc
	port        uint16
	ipv6Wait    chan []byte
}

type inboundPkt struct {
	buf []byte // len maxWGPacket; first n bytes are the datagram
	n   int
	ep  conn.Endpoint
}

func NewICEBind() *ICEBind {
	return &ICEBind{
		byIP:        make(map[netip.Addr]*peerTransport),
		byConn:      make(map[net.Conn]netip.Addr),
		inbound:     make(chan inboundPkt, 256),
		ipv6Wait:    make(chan []byte),
		closeSignal: make(chan struct{}),
	}
}

// SetPeerConn attaches an ICE data conn for a peer VPN address.
func (b *ICEBind) SetPeerConn(vpnIP netip.Addr, c net.Conn) {
	b.mu.Lock()
	if b.shutdown {
		b.mu.Unlock()
		return
	}
	var ctr *peerCounters
	if old, ok := b.byIP[vpnIP]; ok {
		if old.conn == c {
			b.mu.Unlock()
			return
		}
		ctr = old.counters
		delete(b.byConn, old.conn)
		_ = old.conn.Close()
	} else {
		ctr = &peerCounters{}
	}
	pt := &peerTransport{conn: c, closed: make(chan struct{}), counters: ctr}
	b.byIP[vpnIP] = pt
	b.byConn[c] = vpnIP
	b.mu.Unlock()
	go b.readLoop(vpnIP, pt)
}

func (b *ICEBind) RemovePeer(vpnIP netip.Addr) {
	b.mu.Lock()
	if pt, ok := b.byIP[vpnIP]; ok {
		delete(b.byIP, vpnIP)
		delete(b.byConn, pt.conn)
		_ = pt.conn.Close()
	}
	b.mu.Unlock()
}

// WaitPeerTransport blocks until the ICE read loop for vpnIP exits (conn closed/replaced).
func (b *ICEBind) WaitPeerTransport(ctx context.Context, vpnIP netip.Addr) bool {
	b.mu.RLock()
	pt, ok := b.byIP[vpnIP]
	b.mu.RUnlock()
	if !ok || pt == nil {
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case <-pt.closed:
		return true
	}
}

// Stats returns a snapshot of bind and per-peer transport counters.
func (b *ICEBind) Stats() BindStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	st := BindStats{
		Shutdown: b.shutdown,
		Open:     b.open,
	}
	for ip, pt := range b.byIP {
		if pt == nil {
			continue
		}
		lastErr, lastErrAt, lastSendOK, lastRecv, consecutive := pt.counters.health.snapshot()
		st.Peers = append(st.Peers, PeerTransportStats{
			VPNIP:             ip,
			HasActiveConn:       pt.conn != nil,
			SendPackets:         pt.counters.sendPackets.Load(),
			SendBytes:           pt.counters.sendBytes.Load(),
			SendNoTransport:     pt.counters.sendNoTransport.Load(),
			SendErrors:          pt.counters.sendErrors.Load(),
			ConsecutiveErrors:   consecutive,
			LastSendError:       lastErr,
			LastSendErrorAt:     lastErrAt,
			LastSendOKAt:        lastSendOK,
			LastRecvAt:          lastRecv,
			RecvPackets:         pt.counters.recvPackets.Load(),
			RecvBytes:           pt.counters.recvBytes.Load(),
			RecvDropped:         pt.counters.recvDropped.Load(),
			ReadLoopExits:       pt.counters.readLoopExits.Load(),
		})
	}
	return st
}

// HasPeerTransport reports whether an ICE conn is currently attached for vpnIP.
func (b *ICEBind) HasPeerTransport(vpnIP netip.Addr) bool {
	b.mu.RLock()
	pt, ok := b.byIP[vpnIP]
	active := ok && pt != nil && pt.conn != nil
	b.mu.RUnlock()
	return active
}

func (b *ICEBind) readLoop(vpnIP netip.Addr, pt *peerTransport) {
	defer func() {
		pt.counters.readLoopExits.Add(1)
		close(pt.closed)
	}()
	c := pt.conn
	ep := &PeerEndpoint{VPNIP: vpnIP}
	for {
		buf := getPacketBuf()
		n, err := c.Read(buf)
		if err != nil {
			putPacketBuf(buf)
			return
		}
		if n == 0 {
			putPacketBuf(buf)
			continue
		}
		pt.counters.recvPackets.Add(1)
		pt.counters.recvBytes.Add(uint64(n))
		pt.counters.health.recordRecv()
		pkt := inboundPkt{buf: buf, n: n, ep: ep}
		b.mu.RLock()
		shutdown := b.shutdown
		inbound := b.inbound
		sig := b.closeSignal
		b.mu.RUnlock()
		if shutdown || inbound == nil {
			putPacketBuf(buf)
			return
		}
		if sig != nil {
			select {
			case <-sig:
				putPacketBuf(buf)
				return
			default:
			}
		}
		select {
		case inbound <- pkt:
		default:
			pt.counters.recvDropped.Add(1)
			if sig != nil {
				select {
				case inbound <- pkt:
				case <-sig:
					putPacketBuf(buf)
					return
				}
			} else {
				putPacketBuf(buf)
			}
		}
	}
}

func (b *ICEBind) Open(_ uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	if b.shutdown {
		b.mu.Unlock()
		return nil, 0, net.ErrClosed
	}
	if b.open && len(b.recvFns) > 0 {
		fns := b.recvFns
		port := b.port
		b.mu.Unlock()
		return fns, port, nil
	}
	if b.closeSignal == nil {
		b.closeSignal = make(chan struct{})
	}
	if b.inbound == nil {
		b.inbound = make(chan inboundPkt, 256)
	}
	sig := b.closeSignal
	b.open = true
	b.mu.Unlock()

	recv4 := func(packets [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		select {
		case <-sig:
			return 0, net.ErrClosed
		case pkt, ok := <-b.inbound:
			if !ok {
				return 0, net.ErrClosed
			}
			if len(packets) == 0 {
				return 0, errors.New("no buffers")
			}
			n := copy(packets[0], pkt.buf[:pkt.n])
			sizes[0] = n
			eps[0] = pkt.ep
			putPacketBuf(pkt.buf)
			return 1, nil
		}
	}
	recv6 := func(packets [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		select {
		case <-sig:
			return 0, net.ErrClosed
		case <-b.ipv6Wait:
			return 0, net.ErrClosed
		}
	}
	fns := []conn.ReceiveFunc{recv4, recv6}
	b.mu.Lock()
	b.recvFns = fns
	b.port = defaultWGPort
	b.mu.Unlock()
	return fns, defaultWGPort, nil
}

// Close stops WireGuard receive routines. wireguard-go calls Close then Open on
// bind updates; ICE peer connections are left intact until Shutdown.
func (b *ICEBind) Close() error {
	b.mu.Lock()
	if b.shutdown {
		b.mu.Unlock()
		return nil
	}
	sig := b.closeSignal
	b.closeSignal = nil
	b.open = false
	b.recvFns = nil
	b.mu.Unlock()
	if sig != nil {
		select {
		case <-sig:
		default:
			close(sig)
		}
	}
	return nil
}

// Shutdown tears down peer ICE connections and permanently closes the bind.
func (b *ICEBind) Shutdown() error {
	b.mu.Lock()
	if b.shutdown {
		b.mu.Unlock()
		return nil
	}
	b.shutdown = true
	b.open = false
	sig := b.closeSignal
	b.closeSignal = nil
	inbound := b.inbound
	b.inbound = nil
	b.recvFns = nil
	conns := make([]net.Conn, 0, len(b.byIP))
	for _, pt := range b.byIP {
		if pt != nil {
			conns = append(conns, pt.conn)
		}
	}
	b.byIP = make(map[netip.Addr]*peerTransport)
	b.byConn = make(map[net.Conn]netip.Addr)
	b.mu.Unlock()

	for _, c := range conns {
		_ = c.Close()
	}
	if sig != nil {
		select {
		case <-sig:
		default:
			close(sig)
		}
	}
	if inbound != nil {
		close(inbound)
		for pkt := range inbound {
			putPacketBuf(pkt.buf)
		}
	}
	return nil
}

func (b *ICEBind) SetMark(_ uint32) error { return nil }

func (b *ICEBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	pe, ok := ep.(*PeerEndpoint)
	if !ok {
		return conn.ErrWrongEndpointType
	}
	b.mu.RLock()
	shutdown := b.shutdown
	pt, hasConn := b.byIP[pe.VPNIP]
	b.mu.RUnlock()
	if shutdown {
		return net.ErrClosed
	}
	if !hasConn || pt == nil || pt.conn == nil {
		if pt != nil {
			pt.counters.sendNoTransport.Add(1)
		}
		return ErrNoTransport
	}
	for _, buf := range bufs {
		if len(buf) > maxWGPacket {
			return fmt.Errorf("packet too large: %d", len(buf))
		}
		if _, err := pt.conn.Write(buf); err != nil {
			pt.counters.sendErrors.Add(1)
			terminal, _ := pt.counters.health.recordSendErr(err)
			if terminal {
				b.closePeerTransport(pe.VPNIP, pt)
			}
			return err
		}
		pt.counters.sendPackets.Add(1)
		pt.counters.sendBytes.Add(uint64(len(buf)))
		pt.counters.health.recordSendOK()
	}
	return nil
}

func (b *ICEBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	ap, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}
	return &PeerEndpoint{VPNIP: ap.Addr()}, nil
}

func (b *ICEBind) BatchSize() int { return 1 }

func (b *ICEBind) closePeerTransport(vpnIP netip.Addr, pt *peerTransport) {
	pt.closeOnce.Do(func() {
		b.mu.Lock()
		cur, ok := b.byIP[vpnIP]
		if ok && cur == pt && cur.conn != nil {
			_ = cur.conn.Close()
		}
		b.mu.Unlock()
	})
}

// DuplexPair creates two binds each with one peer conn over net.Pipe halves.
func DuplexPair(localIP, remoteIP netip.Addr) (local *ICEBind, remote *ICEBind, cleanup func()) {
	a, b := net.Pipe()
	local = NewICEBind()
	remote = NewICEBind()
	local.SetPeerConn(remoteIP, a)
	remote.SetPeerConn(localIP, b)
	return local, remote, func() {
		_ = local.Shutdown()
		_ = remote.Shutdown()
	}
}

// DuplexPairUDP uses loopback UDP (closer to ICE than net.Pipe for WireGuard tests).
func DuplexPairUDP(localIP, remoteIP netip.Addr) (local *ICEBind, remote *ICEBind, cleanup func()) {
	a, b, closeUDP := udpPair()
	local = NewICEBind()
	remote = NewICEBind()
	local.SetPeerConn(remoteIP, a)
	remote.SetPeerConn(localIP, b)
	return local, remote, func() {
		_ = local.Shutdown()
		_ = remote.Shutdown()
		closeUDP()
	}
}
