package daemon

import (
	"os/exec"
	"runtime"
	"strings"
)

// DefaultNetSnapshot returns a stable fingerprint of the default IPv4 egress path.
func DefaultNetSnapshot() NetFingerprint {
	switch runtime.GOOS {
	case "darwin":
		return darwinNetSnapshot()
	case "linux":
		return linuxNetSnapshot()
	default:
		return ""
	}
}

func darwinNetSnapshot() NetFingerprint {
	out, err := exec.Command("netstat", "-rn", "-f", "inet").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if fields[0] != "default" {
			continue
		}
		// default gateway iface
		return NetFingerprint("darwin:" + fields[len(fields)-1])
	}
	return ""
}

func linuxNetSnapshot() NetFingerprint {
	out, err := exec.Command("ip", "-4", "route", "show", "default").Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return ""
	}
	fields := strings.Fields(line)
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return NetFingerprint("linux:" + fields[i+1])
		}
	}
	return NetFingerprint("linux:" + line)
}
