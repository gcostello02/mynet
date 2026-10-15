package peer

import "testing"

func TestEncodePingLayout(t *testing.T) {
	p := EncodePing(7, "ping")
	if len(p) != 14 {
		t.Fatalf("unexpected length %d", len(p))
	}
	if p[0] != 'M' || p[1] != 'N' || p[3] != msgPing {
		t.Fatalf("bad header %#v", p[:4])
	}
}
