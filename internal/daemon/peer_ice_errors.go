package daemon

import (
	"context"
	"errors"
	"strings"
)

// peerICEErrorCountsAsFailure reports whether a failed tryPeerICE result should increment recovery failure metrics.
func peerICEErrorCountsAsFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "ice credentials") ||
		strings.Contains(msg, "ice gather") ||
		strings.Contains(msg, "ice connect")
}
