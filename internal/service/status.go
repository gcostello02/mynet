package service

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gcostello02/mynet/internal/config"
	"github.com/gcostello02/mynet/internal/daemon"
)

// HealthReport aggregates supervisor, daemon, and VPN readiness.
type HealthReport struct {
	Supervisor          string
	Installed           bool // supervised install: local metadata and/or system unit
	LocalSpecPresent    bool
	LocalSpecReadable   bool
	SystemUnitPresent   bool
	SupervisorActive    bool
	SupervisorActiveKnown bool
	BootAutostartEnabled bool
	BootAutostartKnown  bool
	EnabledHint         string
	RunningHint         string
	StateDir            string
	DaemonReachable     bool
	DaemonStatus        *daemon.Status
	VPNDataReady        bool
	LastError           string
}

func CollectHealth(stateDir, localSpecPath string) HealthReport {
	layout := SystemLayoutForOS()
	specPresent, specReadable, _ := localSpecPresence(localSpecPath)
	var spec InstallSpec
	if specReadable {
		spec, _, _ = LoadLocalSpec(localSpecPath)
	}
	systemUnit := SystemUnitInstalled(layout)
	hr := HealthReport{
		StateDir:            stateDir,
		LocalSpecPresent:    specPresent,
		LocalSpecReadable:   specReadable,
		SystemUnitPresent:   systemUnit,
		Installed:           specPresent || systemUnit,
		DaemonReachable:     daemon.DaemonReachable(stateDir),
	}
	if specReadable && spec.Supervisor != "" {
		hr.Supervisor = spec.Supervisor
	}
	active, bootEnabled, activeKnown, bootKnown := probeSupervisor(layout)
	hr.SupervisorActive = active
	hr.SupervisorActiveKnown = activeKnown
	hr.BootAutostartEnabled = bootEnabled
	hr.BootAutostartKnown = bootKnown
	switch runtime.GOOS {
	case "darwin":
		hr.Supervisor = "launchd"
	case "linux":
		hr.Supervisor = "systemd"
	}
	if hr.DaemonReachable {
		resp, err := daemon.CallIPC(stateDir, "status")
		if err == nil && resp.OK && resp.Status != nil {
			hr.DaemonStatus = resp.Status
			hr.VPNDataReady = resp.Status.WireGuard == "Active" && resp.Status.VPNIP != ""
		} else if err != nil {
			hr.LastError = err.Error()
		}
	} else {
		hr.LastError = "daemon IPC not reachable"
	}
	hr.EnabledHint = platformEnabledHint(layout)
	hr.RunningHint = platformRunningHint(layout)
	return hr
}

func platformEnabledHint(layout SystemLayout) string {
	switch runtime.GOOS {
	case "linux":
		return "check: systemctl is-enabled mynet-daemon"
	case "darwin":
		return "check: launchctl print system/" + Label
	default:
		return ""
	}
}

func platformRunningHint(layout SystemLayout) string {
	switch runtime.GOOS {
	case "linux":
		return "check: systemctl is-active mynet-daemon"
	case "darwin":
		return "check: launchctl print system/" + Label
	default:
		return ""
	}
}

// FormatHealth prints a human-readable service status block.
func FormatHealth(hr HealthReport) string {
	var b string
	b += fmt.Sprintf("Supervisor:      %s\n", hr.Supervisor)
	b += fmt.Sprintf("Service installed: %v\n", hr.Installed)
	b += fmt.Sprintf("  local metadata:  present=%v readable=%v\n", hr.LocalSpecPresent, hr.LocalSpecReadable)
	b += fmt.Sprintf("  system unit:     present=%v\n", hr.SystemUnitPresent)
	if hr.SupervisorActiveKnown {
		b += fmt.Sprintf("  supervisor run:  %v\n", hr.SupervisorActive)
	}
	if hr.BootAutostartKnown {
		b += fmt.Sprintf("  boot autostart:  %v\n", hr.BootAutostartEnabled)
	}
	if hr.EnabledHint != "" && !hr.BootAutostartKnown {
		b += fmt.Sprintf("Boot autostart hint: %s\n", hr.EnabledHint)
	}
	if hr.RunningHint != "" && !hr.SupervisorActiveKnown {
		b += fmt.Sprintf("Supervisor run hint: %s\n", hr.RunningHint)
	}
	b += fmt.Sprintf("State directory: %s\n", hr.StateDir)
	b += fmt.Sprintf("Daemon IPC:      %v\n", hr.DaemonReachable)
	if hr.DaemonStatus != nil {
		st := hr.DaemonStatus
		b += fmt.Sprintf("Coordination:    %s\n", st.Signaling)
		b += fmt.Sprintf("WireGuard engine:%s\n", st.WireGuard)
		if st.DaemonPID > 0 {
			b += fmt.Sprintf("Daemon PID:      %d (uptime %s)\n", st.DaemonPID, st.DaemonUptime)
		}
		if st.LastLifecycleError != "" {
			b += fmt.Sprintf("Lifecycle note:  %s\n", st.LastLifecycleError)
		}
	}
	b += fmt.Sprintf("VPN data path:   %v\n", hr.VPNDataReady)
	if hr.LastError != "" && !hr.DaemonReachable {
		b += fmt.Sprintf("Note:            %s\n", hr.LastError)
	}
	return b
}

func UptimeSince(start time.Time) string {
	if start.IsZero() {
		return ""
	}
	d := time.Since(start).Round(time.Second)
	return d.String()
}

// ResolveCLIStateDir mirrors CLI state resolution.
func ResolveCLIStateDir() (string, error) {
	return config.ResolveStateDir()
}

// LocalSpecPath returns the per-user service metadata path.
func LocalSpecPath() string {
	layout := SystemLayoutForOS()
	if layout.LocalSpec != "" {
		return layout.LocalSpec
	}
	return filepath.Join(config.DefaultStateDir(), "service.json")
}
