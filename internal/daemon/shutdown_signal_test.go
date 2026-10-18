package daemon

import (
	"testing"
	"time"
)

func TestGracefulShutdownFromSignalNoActiveService(t *testing.T) {
	if GracefulShutdownFromSignal(time.Second) {
		t.Fatal("expected false with no active service")
	}
}

func TestGracefulShutdownFromSignalClosesCoordinator(t *testing.T) {
	dir := t.TempDir()
	node := &Node{StateDir: dir}
	s := &Service{StateDir: dir, Node: node, shutdown: newShutdownCoordinator()}
	registerActiveService(s)
	if !GracefulShutdownFromSignal(2 * time.Second) {
		t.Fatal("expected graceful stop")
	}
	unregisterActiveService()
}
