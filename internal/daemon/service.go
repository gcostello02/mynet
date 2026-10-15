package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// Service is the long-lived mynetd IPC server for one state directory.
type Service struct {
	StateDir string
	Node     *Node

	mu   sync.Mutex
	ln   net.Listener
	lock *os.File
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

	defer func() {
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
	case "quit":
		writeResp(c, Response{OK: true})
		s.scheduleStop()
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
