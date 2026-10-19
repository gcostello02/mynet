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
	_ = c.SetDeadline(time.Now().Add(timeout))

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
	if _, err := c.WriteTo(b, &net.IPAddr{IP: dst}); err != nil {
		return 0, err
	}
	for {
		remaining := time.Until(start.Add(timeout))
		if remaining <= 0 {
			return 0, fmt.Errorf("ping timeout")
		}
		_ = c.SetDeadline(time.Now().Add(remaining))
		n, _, err := c.ReadFrom(make([]byte, 1500))
		if err != nil {
			return 0, err
		}
		rm, err := icmp.ParseMessage(1, n)
		if err != nil {
			continue
		}
		if rm.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		echo, ok := rm.Body.(*icmp.Echo)
		if !ok || echo.ID != id || echo.Seq != seq {
			continue
		}
		return time.Since(start), nil
	}
}
