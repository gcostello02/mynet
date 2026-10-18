package client

import (
	"errors"
	"testing"
)

func TestEffectiveLastErrorClearsWhenDataPathReady(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.recordConnError("peer-a", "sess", "ice_credentials", errors.New("timed out waiting for peer ICE credentials"))
	if rt.EffectiveLastError("peer-a", false) == "" {
		t.Fatal("expected active error")
	}
	if rt.EffectiveLastError("peer-a", true) != "" {
		t.Fatal("expected no current error when data path ready")
	}
	hist, _ := rt.historicalLastError()
	if hist == "" {
		t.Fatal("expected historical error preserved")
	}
}

func TestEffectiveLastErrorScopedToPeer(t *testing.T) {
	rt := NewRuntime(t.TempDir(), nil)
	rt.recordConnError("peer-a", "sess", "ice_connect", errors.New("fail a"))
	if rt.EffectiveLastError("peer-b", false) != "" {
		t.Fatal("expected no error for unrelated peer")
	}
}
