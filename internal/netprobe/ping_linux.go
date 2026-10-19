//go:build linux

package netprobe

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const icmpReadBuf = 1500

// PingIPv4 sends one ICMP echo to addr and returns RTT (requires CAP_NET_RAW or unprivileged ping).
func PingIPv4(addr string, timeout time.Duration) (time.Duration, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	dst := net.ParseIP(addr)
	if dst == nil || dst.To4() == nil {
		return 0, fmt.Errorf("invalid ipv4 address")
	}
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return 0, err
	}
	defer c.Close()

	id := os.Getpid() & 0xffff
	seq := int(time.Now().UnixNano() & 0xffff)
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: []byte("mynet"),
		},
	}
	b, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	deadline := start.Add(timeout)
	if err := c.SetDeadline(deadline); err != nil {
		return 0, err
	}
	if _, err := c.WriteTo(b, &net.IPAddr{IP: dst}); err != nil {
		return 0, err
	}

	buf := make([]byte, icmpReadBuf)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, fmt.Errorf("ping timeout")
		}
		if err := c.SetDeadline(time.Now().Add(remaining)); err != nil {
			return 0, err
		}
		n, _, err := c.ReadFrom(buf)
		if err != nil {
			return 0, err
		}
		if n <= 0 {
			continue
		}
		if matchIPv4EchoReply(buf[:n], id, seq) {
			return time.Since(start), nil
		}
	}
}

// matchIPv4EchoReply reports whether packet is an ICMP echo reply for id/seq.
func matchIPv4EchoReply(packet []byte, wantID int, wantSeq int) bool {
	if len(packet) == 0 {
		return false
	}
	rm, err := icmp.ParseMessage(1, packet)
	if err != nil {
		return false
	}
	if rm.Type != ipv4.ICMPTypeEchoReply {
		return false
	}
	echo, ok := rm.Body.(*icmp.Echo)
	if !ok {
		return false
	}
	return echo.ID == wantID && echo.Seq == wantSeq
}
