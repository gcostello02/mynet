//go:build darwin

package service

import (
	"os"
	"os/exec"
	"strings"
)

// probeSupervisor reports whether com.mynet.daemon is loaded in launchd and whether
// boot autostart is configured (plist present under /Library/LaunchDaemons).
func probeSupervisor(layout SystemLayout) (active, enabledAtBoot bool, activeKnown, enabledKnown bool) {
	if layout.LaunchDaemonPlist == "" {
		return false, false, false, false
	}
	if _, err := os.Stat(layout.LaunchDaemonPlist); err != nil {
		return false, false, false, false
	}
	enabledKnown = true
	enabledAtBoot = true

	out, err := exec.Command("launchctl", "print", "system/"+Label).CombinedOutput()
	if err != nil {
		activeKnown = true
		active = false
		return active, enabledAtBoot, activeKnown, enabledKnown
	}
	activeKnown = true
	text := string(out)
	// launchctl print includes "state = running" when the job is up.
	active = strings.Contains(text, "state = running") || strings.Contains(text, "pid = ")
	return active, enabledAtBoot, activeKnown, enabledKnown
}
