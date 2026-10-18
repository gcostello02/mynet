package daemon

import (
	"testing"
	"time"
)

func TestGracefulShutdownFromSignalNoActiveService(t *testing.T) {
	unregisterActiveService()
	if GracefulShutdownFromSignal(time.Second) {
		t.Fatal("expected false with no active service")
	}
}

func TestGracefulShutdownFromSignalClosesCoordinator(t *testing.T) {
	unregisterActiveService()
	dir := t.TempDir()
	node := &Node{StateDir: dir}
	s := &Service{StateDir: dir, Node: node, shutdown: newShutdownCoordinator()}
	registerActiveService(s)
	defer unregisterActiveService()
	if !GracefulShutdownFromSignal(2 * time.Second) {
		t.Fatal("expected graceful stop")
	}
}
