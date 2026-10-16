package daemon

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// FetchPprofOverUnix performs an HTTP GET over a Unix socket (localhost-only pprof).
func FetchPprofOverUnix(socketPath, pprofPath string, timeout time.Duration) ([]byte, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("empty pprof socket path")
	}
	if pprofPath == "" {
		pprofPath = "/debug/pprof/"
	}
	if pprofPath[0] != '/' {
		pprofPath = "/" + pprofPath
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: 10 * time.Second}
				return d.DialContext(ctx, "unix", socketPath)
			},
			DisableKeepAlives: true,
		},
	}
	req, err := http.NewRequest(http.MethodGet, "http://localhost"+pprofPath, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pprof http %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
