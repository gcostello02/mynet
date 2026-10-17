package daemon

import (
	"os"
	"testing"
	"time"
)

func TestPprofDisabledByDefault(t *testing.T) {
	t.Setenv(envEnablePprof, "")
	if PprofEnabled() {
		t.Fatal("expected disabled")
	}
	ps, err := StartProfileServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ps != nil {
		t.Fatal("expected nil server")
	}
}

func TestPprofUnixSocketWhenEnabled(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "mynetp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv(envEnablePprof, "1")
	ps, err := StartProfileServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ps == nil {
		t.Fatal("expected server")
	}
	defer ps.Close()
	sock := PprofSocketPath(dir)
	if _, err := os.Stat(sock); err != nil {
		t.Fatal(err)
	}
	data, err := FetchPprofOverUnix(sock, "/debug/pprof/heap", 5*time.Second)
	if err != nil || len(data) == 0 {
		t.Fatalf("heap fetch: %v len=%d", err, len(data))
	}
}

func TestCPUProfileIPC(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "mynetp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv(envEnablePprof, "1")
	node := &Node{StateDir: dir}
	go func() {
		_ = ServeIPC(dir, node)
	}()
	time.Sleep(50 * time.Millisecond)
	resp, err := CallIPCRequest(dir, Request{Method: "profile_cpu", Seconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.Profile == nil || resp.Profile.DataB64 == "" {
		t.Fatalf("resp: %+v err=%s", resp, resp.Error)
	}
	data, err := decodeProfileBytes(resp.Profile.DataB64)
	if err != nil || len(data) == 0 {
		t.Fatalf("decode profile: %v len=%d", err, len(data))
	}
	_ = RequestQuit(dir, DefaultQuitWait)
}
