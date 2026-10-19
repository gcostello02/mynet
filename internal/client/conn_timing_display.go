package client

import "time"

// maxDisplayConnStage is the longest plausible single-stage duration for diagnostics.
const maxDisplayConnStage = 24 * time.Hour

// ConnStageDurationOK reports whether d is safe to show (not zero-time artifact or overflow).
func ConnStageDurationOK(d time.Duration) bool {
	if d < 0 {
		return false
	}
	return d <= maxDisplayConnStage
}

// SanitizeConnStageDuration returns d or -1 when not displayable.
func SanitizeConnStageDuration(d time.Duration) time.Duration {
	if !ConnStageDurationOK(d) {
		return -1
	}
	return d
}
