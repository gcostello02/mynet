package service

import (
	"fmt"
	"strings"
)

// SystemdUnit renders a system unit for the foreground VPN daemon.
func SystemdUnit(binaryPath, envFile string) string {
	return fmt.Sprintf(`[Unit]
Description=MyNet VPN daemon (%s)
Documentation=https://github.com/grantcostello/mynet
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=300
StartLimitBurst=5

[Service]
Type=simple
EnvironmentFile=-%s
ExecStart=%s _daemon
Restart=on-failure
RestartSec=5
TimeoutStopSec=30
KillSignal=SIGTERM
# TUN/routing on Linux typically requires elevated privileges.
User=root
AmbientCapabilities=CAP_NET_ADMIN
# Root must read/write the identity owner's ~/.mynet (often mode 0700 under a 0750 home)
# without broadening filesystem permissions. Bounded caps drop DAC bypass by default.
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_SETUID CAP_SETGID CAP_CHOWN CAP_DAC_READ_SEARCH CAP_DAC_OVERRIDE
NoNewPrivileges=false

[Install]
WantedBy=multi-user.target
`, Label, envFile, binaryPath)
}

func ParseSystemdExecStart(unit string) (string, bool) {
	for _, line := range strings.Split(unit, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ExecStart=") {
			return strings.TrimPrefix(line, "ExecStart="), true
		}
	}
	return "", false
}

// systemdSection returns trimmed non-empty lines for a unit section (excluding header).
func systemdSection(unit, name string) []string {
	header := "[" + name + "]"
	start := strings.Index(unit, header)
	if start < 0 {
		return nil
	}
	rest := unit[start+len(header):]
	end := strings.Index(rest, "\n[")
	if end >= 0 {
		rest = rest[:end]
	}
	var lines []string
	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines
}

// StartLimitKeysInUnitSection reports whether burst/interval appear only under [Unit].
func StartLimitKeysInUnitSection(unit string) (ok bool, detail string) {
	unitLines := systemdSection(unit, "Unit")
	serviceLines := systemdSection(unit, "Service")
	hasInterval := false
	hasBurst := false
	for _, l := range unitLines {
		if strings.HasPrefix(l, "StartLimitIntervalSec=") {
			hasInterval = true
		}
		if strings.HasPrefix(l, "StartLimitBurst=") {
			hasBurst = true
		}
	}
	for _, l := range serviceLines {
		if strings.HasPrefix(l, "StartLimitIntervalSec=") || strings.HasPrefix(l, "StartLimitBurst=") {
			return false, "StartLimit* must not appear in [Service]"
		}
	}
	if !hasInterval || !hasBurst {
		return false, "StartLimitIntervalSec and StartLimitBurst required in [Unit]"
	}
	return true, ""
}
