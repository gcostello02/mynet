package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gcostello02/mynet/internal/config"
)

// InstallSpec records a service installation (also mirrored locally for CLI status).
type InstallSpec struct {
	Supervisor   string    `json:"supervisor"` // launchd | systemd
	BinaryPath   string    `json:"binary_path"`
	SourceBinary string    `json:"source_binary,omitempty"` // candidate copied from (macOS libexec install)
	StateDir     string    `json:"state_dir"`
	OwnerUser    string    `json:"owner_user"`
	InstalledAt  time.Time `json:"installed_at"`
	LaunchdPlist string    `json:"launchd_plist,omitempty"`
	SystemdUnit  string    `json:"systemd_unit,omitempty"`
	EnvFile      string    `json:"env_file"`
}

func SaveLocalSpec(path string, spec InstallSpec) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadLocalSpec(path string) (InstallSpec, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return InstallSpec{}, false, nil
		}
		return InstallSpec{}, false, err
	}
	var spec InstallSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return InstallSpec{}, false, err
	}
	return spec, true, nil
}

// EnvFileContents builds the environment file shared by launchd/systemd.
func EnvFileContents(stateDir, ownerUser, binaryPath string) string {
	return fmt.Sprintf("%s=%s\n%s=1\n%s=1\n%s=%s\n%s=%s\n",
		config.EnvStateDir, stateDir,
		config.EnvSupervised,
		config.EnvRunVPNAtStart,
		config.EnvOwnerUser, ownerUser,
		config.EnvServiceBinary, binaryPath,
	)
}
