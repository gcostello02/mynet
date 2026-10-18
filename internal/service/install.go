package service

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"time"

	"github.com/grantcostello/mynet/internal/config"
)

// InstallOptions configures service registration.
type InstallOptions struct {
	BinaryPath string
	StateDir   string
	OwnerUser  string
	DryRun     bool
}

// Install writes service templates and local metadata. Requires root on real hosts.
func Install(opts InstallOptions) (InstallSpec, error) {
	if opts.BinaryPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return InstallSpec{}, err
		}
		opts.BinaryPath = exe
	}
	if opts.StateDir == "" {
		sd, err := config.ResolveStateDir()
		if err != nil {
			return InstallSpec{}, err
		}
		opts.StateDir = sd
	}
	if err := config.ValidateStateDirPath(opts.StateDir); err != nil {
		return InstallSpec{}, err
	}
	if err := config.IdentityMustExist(opts.StateDir); err != nil {
		return InstallSpec{}, err
	}
	if opts.OwnerUser == "" {
		opts.OwnerUser = os.Getenv("USER")
	}
	if opts.OwnerUser == "" {
		return InstallSpec{}, fmt.Errorf("owner user required (set --user or run as a normal user)")
	}
	if _, err := user.Lookup(opts.OwnerUser); err != nil {
		return InstallSpec{}, fmt.Errorf("owner user %q: %w", opts.OwnerUser, err)
	}

	layout := SystemLayoutForOS()
	spec := InstallSpec{
		BinaryPath:  opts.BinaryPath,
		StateDir:    opts.StateDir,
		OwnerUser:   opts.OwnerUser,
		InstalledAt: time.Now().UTC(),
	}

	switch runtime.GOOS {
	case "darwin":
		spec.Supervisor = "launchd"
		spec.LaunchdPlist = layout.LaunchDaemonPlist
		spec.EnvFile = layout.LaunchDaemonEnv
	case "linux":
		spec.Supervisor = "systemd"
		spec.SystemdUnit = layout.SystemdUnit
		spec.EnvFile = layout.SystemdEnvFile
	default:
		return InstallSpec{}, fmt.Errorf("service install unsupported on %s", runtime.GOOS)
	}

	if opts.DryRun {
		return spec, nil
	}
	if os.Geteuid() != 0 {
		return InstallSpec{}, fmt.Errorf("service install requires root (re-run with sudo)")
	}

	if err := os.MkdirAll(layout.LogDir, 0o755); err != nil {
		return InstallSpec{}, err
	}
	envBody := EnvFileContents(opts.StateDir, opts.OwnerUser, opts.BinaryPath)
	if runtime.GOOS == "linux" {
		if err := os.MkdirAll(layout.SystemdEnvDir, 0o755); err != nil {
			return InstallSpec{}, err
		}
		if err := writeFileAtomic(layout.SystemdEnvFile, envBody, 0o600); err != nil {
			return InstallSpec{}, err
		}
		unit := SystemdUnit(opts.BinaryPath, layout.SystemdEnvFile)
		if err := writeFileAtomic(layout.SystemdUnit, unit, 0o644); err != nil {
			return InstallSpec{}, err
		}
		_ = exec.Command("systemctl", "daemon-reload").Run()
	} else if runtime.GOOS == "darwin" {
		logPath := filepath.Join(layout.LogDir, "daemon.log")
		plist := LaunchDaemonPlist(opts.BinaryPath, opts.StateDir, opts.OwnerUser, logPath)
		if err := writeFileAtomic(layout.LaunchDaemonPlist, plist, 0o644); err != nil {
			return InstallSpec{}, err
		}
		if err := writeFileAtomic(layout.LaunchDaemonEnv, envBody, 0o600); err != nil {
			return InstallSpec{}, err
		}
	}

	// Local spec is written as invoking user when possible.
	localPath := LocalSpecPathForOwner(opts.OwnerUser)
	if err := SaveLocalSpec(localPath, spec); err != nil {
		return InstallSpec{}, err
	}
	return spec, nil
}

func LocalSpecPathForOwner(owner string) string {
	if u, err := user.Lookup(owner); err == nil && u.HomeDir != "" {
		return filepath.Join(u.HomeDir, ".mynet", "service.json")
	}
	return LocalSpecPath()
}

func writeFileAtomic(path, body string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Uninstall removes system unit/plist. Preserves state dir and identity.
func Uninstall(dryRun bool) error {
	if os.Geteuid() != 0 && !dryRun {
		return fmt.Errorf("service uninstall requires root (re-run with sudo)")
	}
	layout := SystemLayoutForOS()
	if dryRun {
		return nil
	}
	switch runtime.GOOS {
	case "linux":
		_ = exec.Command("systemctl", "disable", "--now", "mynet-daemon").Run()
		_ = os.Remove(layout.SystemdUnit)
		_ = os.Remove(layout.SystemdEnvFile)
		_ = exec.Command("systemctl", "daemon-reload").Run()
	case "darwin":
		_ = exec.Command("launchctl", "bootout", "system", layout.LaunchDaemonPlist).Run()
		_ = os.Remove(layout.LaunchDaemonPlist)
	}
	_ = os.Remove(LocalSpecPath())
	return nil
}
