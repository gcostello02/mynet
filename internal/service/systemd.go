package service

import (
	"fmt"
	"strings"
)

// SystemdUnit renders a system unit for the foreground VPN daemon.
func SystemdUnit(binaryPath, envFile string) string {
	return fmt.Sprintf(`[Unit]
Description=MyNet VPN daemon (%s)
Documentation=https://github.com/gcostello02/mynet
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-%s
ExecStart=%s _daemon
Restart=on-failure
RestartSec=5
StartLimitIntervalSec=300
StartLimitBurst=5
TimeoutStopSec=30
KillSignal=SIGTERM
# TUN/routing on Linux typically requires elevated privileges.
User=root
AmbientCapabilities=CAP_NET_ADMIN
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_SETUID CAP_SETGID
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
