package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

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

	mu      sync.Mutex
	ln      net.Listener
	lock    *os.File
	pprof   *ProfileServer
}

// Run serves IPC until the listener is closed (quit or shutdown).
func (s *Service) Run() error {
	lock, err := acquireDaemonLock(s.StateDir)
	if err != nil {
		return err
	}
	s.lock = lock

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
		_ = ln.Close()
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
		go s.handleConn(c)
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
				s.scheduleStop()
			}
			return
		}
		st := s.Node.Status()
		writeResp(c, Response{OK: true, Status: &st})
	case "down":
		err := s.Node.Down()
		writeResp(c, Response{OK: err == nil, Error: errString(err)})
	case "status":
		st := s.Node.Status()
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
		s.scheduleStop()
	case "profile_status":
		writeResp(c, Response{
			OK:          true,
			Profile:     profileStatus(s.StateDir),
		})
	case "profile_cpu":
		if !PprofEnabled() {
			writeResp(c, Response{OK: false, Error: "profiling disabled; restart daemon with MYNET_ENABLE_PPROF=1"})
			return
		}
		secs := clampProfileSeconds(req.Seconds)
		_ = c.SetDeadline(time.Now().Add(time.Duration(secs+15) * time.Second))
		out, err := captureProfileFile(s.StateDir, "cpu", func(w io.Writer) error {
			return WriteCPUProfile(w, secs)
		})
		if err != nil {
			writeResp(c, Response{OK: false, Error: err.Error()})
			return
		}
		writeResp(c, Response{OK: true, Profile: &ProfileInfo{OutputPath: out, Seconds: secs}})
	case "profile_write":
		if !PprofEnabled() {
			writeResp(c, Response{OK: false, Error: "profiling disabled; restart daemon with MYNET_ENABLE_PPROF=1"})
			return
		}
		kind := req.Profile
		if kind == "" {
			kind = "heap"
		}
		out, err := captureProfileFile(s.StateDir, kind, func(w io.Writer) error {
			return WriteSnapshotProfile(kind, w)
		})
		if err != nil {
			writeResp(c, Response{OK: false, Error: err.Error()})
			return
		}
		writeResp(c, Response{OK: true, Profile: &ProfileInfo{OutputPath: out, Kind: kind}})
	default:
		writeResp(c, Response{OK: false, Error: "unknown method"})
	}
}

func (s *Service) scheduleStop() {
	go func() {
		s.mu.Lock()
		ln := s.ln
		s.mu.Unlock()
		if ln != nil {
			_ = ln.Close()
		}
	}()
}

// RequestQuit asks a running daemon to exit cleanly.
func RequestQuit(stateDir string) error {
	if !DaemonReachable(stateDir) {
		return nil
	}
	resp, err := CallIPC(stateDir, "quit")
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !DaemonReachable(stateDir) && !IsProcessAlive(stateDir) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not exit after quit")
}
