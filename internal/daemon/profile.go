package daemon

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	_ "net/http/pprof" // registers handlers on http.DefaultServeMux
)

const envEnablePprof = "MYNET_ENABLE_PPROF"
const envPprofMutex = "MYNET_PPROF_MUTEX"
const envPprofBlock = "MYNET_PPROF_BLOCK"

const defaultProfileSeconds = 30
const maxProfileSeconds = 120

// PprofSocketPath is the Unix socket for net/http/pprof (local only).
func PprofSocketPath(stateDir string) string {
	return filepath.Join(stateDir, "pprof.sock")
}

func profilesDir(stateDir string) string {
	return filepath.Join(stateDir, "profiles")
}

// PprofEnabled reports whether the daemon should expose profiling (opt-in).
func PprofEnabled() bool {
	v := strings.TrimSpace(os.Getenv(envEnablePprof))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// ProfileServer serves standard library pprof over a Unix domain socket.
type ProfileServer struct {
	ln   net.Listener
	srv  *http.Server
	done chan struct{}
}

// StartProfileServer listens on a mode-0600 Unix socket when PprofEnabled().
// Returns (nil, nil) when profiling is disabled.
func StartProfileServer(stateDir string) (*ProfileServer, error) {
	if !PprofEnabled() {
		return nil, nil
	}
	if os.Getenv(envPprofMutex) == "1" || strings.EqualFold(os.Getenv(envPprofMutex), "true") {
		runtime.SetMutexProfileFraction(5)
	}
	if os.Getenv(envPprofBlock) == "1" || strings.EqualFold(os.Getenv(envPprofBlock), "true") {
		runtime.SetBlockProfileRate(1)
	}

	path := PprofSocketPath(stateDir)
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	if err := ChownToConfiguredOwner(path); err != nil {
		_ = ln.Close()
		return nil, err
	}

	srv := &http.Server{
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	ps := &ProfileServer{ln: ln, srv: srv, done: make(chan struct{})}
	go func() {
		_ = srv.Serve(ln)
		close(ps.done)
	}()
	return ps, nil
}

// Close shuts down the pprof HTTP server and removes the socket.
func (p *ProfileServer) Close() error {
	if p == nil {
		return nil
	}
	err := p.srv.Close()
	_ = p.ln.Close()
	return err
}

var profileCPU muProfileCPU

type muProfileCPU struct {
	mu      sync.Mutex
	running bool
}

// WriteCPUProfile records a CPU profile for secs seconds into w.
func WriteCPUProfile(w io.Writer, secs int) error {
	if secs <= 0 {
		secs = defaultProfileSeconds
	}
	if secs > maxProfileSeconds {
		secs = maxProfileSeconds
	}
	profileCPU.mu.Lock()
	if profileCPU.running {
		profileCPU.mu.Unlock()
		return fmt.Errorf("cpu profile already in progress")
	}
	profileCPU.running = true
	profileCPU.mu.Unlock()
	defer func() {
		profileCPU.mu.Lock()
		profileCPU.running = false
		profileCPU.mu.Unlock()
	}()

	if err := pprof.StartCPUProfile(w); err != nil {
		return err
	}
	time.Sleep(time.Duration(secs) * time.Second)
	pprof.StopCPUProfile()
	return nil
}

// WriteSnapshotProfile writes a one-shot profile of kind to w.
func WriteSnapshotProfile(kind string, w io.Writer) error {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "heap":
		return pprof.Lookup("heap").WriteTo(w, 0)
	case "allocs", "alloc":
		return pprof.Lookup("allocs").WriteTo(w, 0)
	case "goroutine":
		return pprof.Lookup("goroutine").WriteTo(w, 0)
	case "mutex":
		return pprof.Lookup("mutex").WriteTo(w, 0)
	case "block":
		return pprof.Lookup("block").WriteTo(w, 0)
	default:
		return fmt.Errorf("unknown profile kind %q", kind)
	}
}

func ensureProfilesDir(stateDir string) error {
	dir := profilesDir(stateDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return ChownToConfiguredOwner(dir)
}

func captureProfileFile(stateDir, prefix string, write func(io.Writer) error) (string, error) {
	if err := ensureProfilesDir(stateDir); err != nil {
		return "", err
	}
	dir := profilesDir(stateDir)
	name := fmt.Sprintf("%s-%d.pprof", prefix, time.Now().Unix())
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	if err := write(f); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	_ = ChownToConfiguredOwner(path)
	return path, nil
}

func clampProfileSeconds(secs int) int {
	if secs <= 0 {
		return defaultProfileSeconds
	}
	if secs > maxProfileSeconds {
		return maxProfileSeconds
	}
	return secs
}
