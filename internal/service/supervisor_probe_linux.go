//go:build linux

package service

import (
	"os"
	"os/exec"
	"strings"
)

// probeSupervisor reads systemd active/enabled state (no root required for read).
func probeSupervisor(layout SystemLayout) (active, enabledAtBoot bool, activeKnown, enabledKnown bool) {
	if layout.SystemdUnit == "" {
		return false, false, false, false
	}
	if _, err := os.Stat(layout.SystemdUnit); err != nil {
		return false, false, false, false
	}
	if out, err := exec.Command("systemctl", "is-active", Label).Output(); err == nil {
		activeKnown = true
		active = strings.TrimSpace(string(out)) == "active"
	}
	if out, err := exec.Command("systemctl", "is-enabled", Label).Output(); err == nil {
		enabledKnown = true
		enabledAtBoot = strings.TrimSpace(string(out)) == "enabled"
	}
	return active, enabledAtBoot, activeKnown, enabledKnown
}
