package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// DefaultQuitWait is how long the CLI waits for the daemon process to exit after quit.
const DefaultQuitWait = 10 * time.Second

func profileStatus(stateDir string) *ProfileInfo {
	info := &ProfileInfo{Enabled: PprofEnabled()}
	if info.Enabled {
		info.SocketPath = PprofSocketPath(stateDir)
	}
	return info
}

// Service is the long-lived mynetd IPC server for one state directory.
type Service struct {
	StateDir string
	Node     *Node

	mu        sync.Mutex
	ln        net.Listener
	lock      *os.File
	pprof     *ProfileServer
	startedAt time.Time
	shutdown  *shutdownCoordinator
	activeIPC int
}

// Run serves IPC until quit shuts down the listener and node.
func (s *Service) Run() error {
	lock, err := acquireDaemonLock(s.StateDir)
	if err != nil {
		return err
	}
	s.lock = lock
	s.startedAt = time.Now()
	s.shutdown = newShutdownCoordinator()

	path := SocketPath(s.StateDir)
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		releaseDaemonLock(lock)
		return err
	}
	s.ln = ln
	_ = os.Chmod(path, 0o600)
	_ = chownToSudoInvoker(path)

	ps, err := StartProfileServer(s.StateDir)
	if err != nil {
		_ = ln.Close()
		releaseDaemonLock(s.lock)
		return fmt.Errorf("pprof: %w", err)
	}
	s.pprof = ps

	defer func() {
		if s.pprof != nil {
			_ = s.pprof.Close()
			_ = os.Remove(PprofSocketPath(s.StateDir))
		}
		if s.ln != nil {
			_ = s.ln.Close()
		}
		releaseDaemonLock(s.lock)
		s.lock = nil
		_ = os.Remove(SocketPath(s.StateDir))
		_ = os.Remove(PIDPath(s.StateDir))
	}()

	for {
		c, err := ln.Accept()
		if err != nil {
			return nil
		}
		s.mu.Lock()
		s.activeIPC++
		s.mu.Unlock()
		go func(conn net.Conn) {
			defer func() {
				s.mu.Lock()
				s.activeIPC--
				s.mu.Unlock()
			}()
			s.handleConn(conn)
		}(c)
	}
}

func (s *Service) handleConn(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(120 * time.Second))
	var req Request
	if err := json.NewDecoder(c).Decode(&req); err != nil {
		writeResp(c, Response{OK: false, Error: err.Error()})
		return
	}
	switch req.Method {
	case "ping":
		writeResp(c, Response{OK: true})
	case "preflight":
		plan, paths, err := s.Node.Preflight()
		if err != nil {
			writeResp(c, Response{OK: false, Error: err.Error()})
			return
		}
		writeResp(c, Response{OK: true, Preflight: &PreflightResponse{Plan: plan, Paths: paths}})
	case "up":
		err := s.Node.Up(context.Background())
		if err != nil {
			writeResp(c, Response{OK: false, Error: err.Error()})
			if !s.Node.VPNActive() {
				s.requestQuit()
			}
			return
		}
		st := s.enrichStatus(s.Node.Status())
		writeResp(c, Response{OK: true, Status: &st})
	case "down":
		err := s.Node.Down()
		st := s.enrichStatus(s.Node.Status())
		if err != nil {
			st.LastLifecycleError = err.Error()
		}
		writeResp(c, Response{OK: err == nil, Error: errString(err), Status: &st})
	case "status":
		st := s.enrichStatus(s.Node.Status())
		writeResp(c, Response{OK: true, Status: &st})
	case "diagnose":
		peer := ""
		if req.Peer != "" {
			peer = req.Peer
		}
		d, err := s.Node.Diagnose(peer)
		if err != nil {
			writeResp(c, Response{OK: false, Error: err.Error()})
			return
		}
		writeResp(c, Response{OK: true, Diagnostics: &d})
	case "peers":
		peers, err := s.Node.PeersList()
		if err != nil {
			writeResp(c, Response{OK: false, Error: err.Error()})
			return
		}
		writeResp(c, Response{OK: true, Peers: peers})
	case "quit":
		writeResp(c, Response{OK: true})
		s.requestQuit()
	case "profile_status":
		writeResp(c, Response{
			OK:      true,
			Profile: profileStatus(s.StateDir),
		})
	case "profile_cpu":
		if !PprofEnabled() {
			writeResp(c, Response{OK: false, Error: "profiling disabled; restart daemon with MYNET_ENABLE_PPROF=1"})
			return
		}
		secs := clampProfileSeconds(req.Seconds)
		_ = c.SetDeadline(time.Now().Add(time.Duration(secs+15) * time.Second))
		b64, err := captureProfileBytes(func(w io.Writer) error {
			return WriteCPUProfile(w, secs)
		})
		if err != nil {
			writeResp(c, Response{OK: false, Error: err.Error()})
			return
		}
		writeResp(c, Response{OK: true, Profile: &ProfileInfo{DataB64: b64, Seconds: secs}})
	case "profile_write":
		if !PprofEnabled() {
			writeResp(c, Response{OK: false, Error: "profiling disabled; restart daemon with MYNET_ENABLE_PPROF=1"})
			return
		}
		kind := req.Profile
		if kind == "" {
			kind = "heap"
		}
		b64, err := captureProfileBytes(func(w io.Writer) error {
			return WriteSnapshotProfile(kind, w)
		})
		if err != nil {
			writeResp(c, Response{OK: false, Error: err.Error()})
			return
		}
		writeResp(c, Response{OK: true, Profile: &ProfileInfo{DataB64: b64, Kind: kind}})
	default:
		writeResp(c, Response{OK: false, Error: "unknown method"})
	}
}

func (s *Service) enrichStatus(st Status) Status {
	st.DaemonPID = os.Getpid()
	if !s.startedAt.IsZero() {
		st.DaemonUptime = formatUptime(time.Since(s.startedAt))
	}
	return st
}

func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func (s *Service) requestQuit() {
	if s.shutdown == nil {
		return
	}
	s.shutdown.trigger(func() {
		if s.Node != nil {
			_ = s.Node.Shutdown()
		}
		if s.pprof != nil {
			_ = s.pprof.Close()
			s.pprof = nil
		}
		s.mu.Lock()
		ln := s.ln
		s.ln = nil
		s.mu.Unlock()
		if ln != nil {
			_ = ln.Close()
		}
	})
}

// RequestQuit asks a running daemon to exit cleanly.
func RequestQuit(stateDir string, wait time.Duration) error {
	if wait <= 0 {
		wait = DefaultQuitWait
	}
	if !DaemonReachable(stateDir) {
		return nil
	}
	pidBefore, _ := readPID(stateDir)
	resp, err := CallIPC(stateDir, "quit")
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if !IsProcessAlive(stateDir) {
			return nil
		}
		if !DaemonReachable(stateDir) && pidBefore > 0 && !processIsAlive(pidBefore) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if pid, ok := readPID(stateDir); ok {
		return fmt.Errorf("daemon (pid %d) did not exit within %s", pid, wait)
	}
	return fmt.Errorf("daemon did not exit within %s", wait)
}

func readPID(stateDir string) (int, bool) {
	data, err := os.ReadFile(PIDPath(stateDir))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func processIsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
