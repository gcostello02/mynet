package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/grantcostello/mynet/internal/config"
)

// SocketPath returns the Unix socket for CLI ↔ daemon IPC.
func SocketPath(stateDir string) string {
	return filepath.Join(stateDir, "mynetd.sock")
}

func PIDPath(stateDir string) string {
	return filepath.Join(stateDir, "mynetd.pid")
}

type Request struct {
	Method string `json:"method"`
}

type Response struct {
	OK        bool               `json:"ok"`
	Error     string             `json:"error,omitempty"`
	Status    *Status            `json:"status,omitempty"`
	Preflight *PreflightResponse `json:"preflight,omitempty"`
}

type PreflightResponse struct {
	Plan  interface{} `json:"plan"`
	Paths interface{} `json:"paths"`
}

// ServeIPC listens on a Unix socket and dispatches node commands.
func ServeIPC(stateDir string, node *Node) error {
	path := SocketPath(stateDir)
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	_ = os.Chmod(path, 0o600)
	_ = chownToSudoInvoker(path)
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go handleConn(c, node)
	}
}

func chownToSudoInvoker(path string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	u := strings.TrimSpace(os.Getenv("SUDO_USER"))
	if u == "" || u == "root" {
		return nil
	}
	ou, err := user.Lookup(u)
	if err != nil {
		return err
	}
	uid, _ := strconv.Atoi(ou.Uid)
	gid, _ := strconv.Atoi(ou.Gid)
	return os.Chown(path, uid, gid)
}

func handleConn(c net.Conn, node *Node) {
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
		plan, paths, err := node.Preflight()
		if err != nil {
			writeResp(c, Response{OK: false, Error: err.Error()})
			return
		}
		writeResp(c, Response{OK: true, Preflight: &PreflightResponse{Plan: plan, Paths: paths}})
	case "up":
		err := node.Up(context.Background())
		if err != nil {
			writeResp(c, Response{OK: false, Error: err.Error()})
			return
		}
		st := node.Status()
		writeResp(c, Response{OK: true, Status: &st})
	case "down":
		err := node.Down()
		writeResp(c, Response{OK: err == nil, Error: errString(err)})
	case "status":
		st := node.Status()
		writeResp(c, Response{OK: true, Status: &st})
	default:
		writeResp(c, Response{OK: false, Error: "unknown method"})
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func writeResp(c net.Conn, resp Response) {
	_ = json.NewEncoder(c).Encode(resp)
}

// CallIPC sends a one-shot request to the daemon.
func CallIPC(stateDir string, method string) (Response, error) {
	path := SocketPath(stateDir)
	conn, err := net.Dial("unix", path)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(Request{Method: method}); err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}

// DaemonReachable returns true if the unix socket accepts a ping.
func DaemonReachable(stateDir string) bool {
	resp, err := CallIPC(stateDir, "ping")
	return err == nil && resp.OK
}

// RemoveStaleSocket removes the socket if no live daemon owns it.
func RemoveStaleSocket(stateDir string) error {
	path := SocketPath(stateDir)
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	if DaemonReachable(stateDir) {
		return nil
	}
	return os.Remove(path)
}

// IsProcessAlive checks pid file against a live process.
func IsProcessAlive(stateDir string) bool {
	data, err := os.ReadFile(PIDPath(stateDir))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks existence on Unix.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func DefaultStateDir() string {
	d, err := config.ResolveStateDir()
	if err != nil {
		return config.DefaultStateDir()
	}
	return d
}
