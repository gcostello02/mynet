package control

import (
	"errors"
	"testing"
)

func TestWrapVPNConfigError(t *testing.T) {
	err412 := errors.New("vpn not registered; POST /v1/vpn/register (http 412)")
	w := WrapVPNConfigError(err412)
	if w == nil || !containsAll(w.Error(), "mynet vpn register", "412") {
		t.Fatalf("got %v", w)
	}
	err404 := errors.New("http 404")
	w404 := WrapVPNConfigError(err404)
	if w404 == nil || !containsAll(w404.Error(), "V0.2") {
		t.Fatalf("got %v", w404)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !stringsContains(s, sub) {
			return false
		}
	}
	return true
}

func stringsContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
