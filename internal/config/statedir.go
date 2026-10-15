package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// EnvStateDir overrides the configuration directory (absolute path recommended).
const EnvStateDir = "MYNET_STATE_DIR"

// ResolveStateDir returns the MyNet state directory for the invoking user.
//
// When run under sudo, root's home (/var/root) is NOT used; the original user's
// ~/.mynet is selected via SUDO_USER (or SUDO_UID).
func ResolveStateDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv(EnvStateDir)); v != "" {
		return filepath.Abs(v)
	}
	if home, err := sudoInvokerHome(); err == nil && home != "" {
		return filepath.Join(home, ".mynet"), nil
	}
	return DefaultStateDir(), nil
}

// MustResolveStateDir is ResolveStateDir that panics on error (tests only).
func MustResolveStateDir() string {
	d, err := ResolveStateDir()
	if err != nil {
		return DefaultStateDir()
	}
	return d
}

func sudoInvokerHome() (string, error) {
	if os.Geteuid() != 0 {
		return "", fmt.Errorf("not root")
	}
	if u := strings.TrimSpace(os.Getenv("SUDO_USER")); u != "" && u != "root" {
		ou, err := user.Lookup(u)
		if err != nil {
			return "", err
		}
		return ou.HomeDir, nil
	}
	return "", fmt.Errorf("no sudo invoker")
}

// IdentityMustExist returns a clear error if identity.json is missing.
func IdentityMustExist(stateDir string) error {
	path := IdentityPath(stateDir)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("device identity not found at %s (do not run mynet init under sudo; set %s or run without sudo)", path, EnvStateDir)
		}
		return err
	}
	return nil
}

// PreflightPaths describes where files will be read from.
type PreflightPaths struct {
	StateDir      string
	IdentityPath  string
	WGKeyPath     string
	ClientConfig  string
	SessionPath   string
	DaemonSocket  string
	EffectiveUser string
	EffectiveEUID int
	SudoInvoker   string
	HomeFromGo    string
}

// CollectPreflightPaths gathers path resolution diagnostics.
func CollectPreflightPaths() (PreflightPaths, error) {
	stateDir, err := ResolveStateDir()
	if err != nil {
		return PreflightPaths{}, err
	}
	home, _ := os.UserHomeDir()
	p := PreflightPaths{
		StateDir:      stateDir,
		IdentityPath:  IdentityPath(stateDir),
		WGKeyPath:     filepath.Join(stateDir, "wg.json"),
		ClientConfig:  ClientConfigPath(stateDir),
		SessionPath:   SessionPath(stateDir),
		DaemonSocket:  filepath.Join(stateDir, "mynetd.sock"),
		EffectiveUser: os.Getenv("USER"),
		EffectiveEUID: os.Geteuid(),
		SudoInvoker:   os.Getenv("SUDO_USER"),
		HomeFromGo:    home,
	}
	return p, nil
}
