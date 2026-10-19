//go:build !linux

package netprobe

import (
	"fmt"
	"time"
)

// PingIPv4 is only implemented on Linux (staging clients run Linux containers).
func PingIPv4(addr string, timeout time.Duration) (time.Duration, error) {
	return 0, fmt.Errorf("icmp ping not supported on this platform")
}
