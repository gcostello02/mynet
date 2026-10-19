//go:build linux

package netprobe

import (
	"testing"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func TestMatchIPv4EchoReply(t *testing.T) {
	id, seq := 42, 7
	reply, err := (&icmp.Message{
		Type: ipv4.ICMPTypeEchoReply,
		Code: 0,
		Body: &icmp.Echo{ID: id, Seq: seq, Data: []byte("mynet")},
	}).Marshal(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !matchIPv4EchoReply(reply, id, seq) {
		t.Fatal("expected matching echo reply")
	}
	if matchIPv4EchoReply(reply, id, seq+1) {
		t.Fatal("wrong seq should not match")
	}
	if matchIPv4EchoReply(reply, id+1, seq) {
		t.Fatal("wrong id should not match")
	}
	if matchIPv4EchoReply(reply[:2], id, seq) {
		t.Fatal("truncated packet should not match")
	}
	if matchIPv4EchoReply([]byte{0xff}, id, seq) {
		t.Fatal("garbage should not match")
	}
	other, err := (&icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{ID: id, Seq: seq},
	}).Marshal(nil)
	if err != nil {
		t.Fatal(err)
	}
	if matchIPv4EchoReply(other, id, seq) {
		t.Fatal("echo request should not match")
	}
}
