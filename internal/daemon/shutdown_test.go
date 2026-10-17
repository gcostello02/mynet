package daemon

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestDownCompletesWhenClientDisconnects(t *testing.T) {
	dir := testStateDir(t)
	node := &Node{StateDir: dir}
	node.mu.Lock()
	node.running = true
	node.mu.Unlock()
	go func() { _ = ServeIPC(dir, node) }()
	waitReachable(t, dir)
	t.Cleanup(func() { _ = RequestQuit(dir, DefaultQuitWait) })

	conn, err := net.Dial("unix", SocketPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewEncoder(conn).Encode(Request{Method: "down"})
	_ = conn.Close()
	time.Sleep(200 * time.Millisecond)
	if node.VPNActive() {
		t.Fatal("expected vpn marked down")
	}
}

func TestIPCDuringShutdown(t *testing.T) {
	dir := startTestDaemon(t)
	go func() { _ = RequestQuit(dir, DefaultQuitWait) }()
	time.Sleep(20 * time.Millisecond)
	_, _ = CallIPC(dir, "ping")
	waitNotReachable(t, dir)
}
