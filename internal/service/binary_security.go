package service

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// SecurityFinding describes a concrete install-path risk or OK state.
type SecurityFinding struct {
	Level  string // ok | warn | critical
	Topic  string
	Detail string
}

// AuditSupervisedBinaryPath inspects the executable launchd/systemd would run (no root required for most checks).
func AuditSupervisedBinaryPath(binaryPath string) ([]SecurityFinding, error) {
	if binaryPath == "" {
		return nil, fmt.Errorf("empty binary path")
	}
	abs, err := filepath.Abs(binaryPath)
	if err != nil {
		return nil, err
	}
	var findings []SecurityFinding
	if !filepath.IsAbs(abs) {
		findings = append(findings, SecurityFinding{
			Level:  "critical",
			Topic:  "binary path",
			Detail: "path must be absolute",
		})
		return findings, nil
	}
	clean := filepath.Clean(abs)
	if err := validateNoSymlinkInPath(clean); err != nil {
		findings = append(findings, SecurityFinding{
			Level:  "critical",
			Topic:  "symlink",
			Detail: err.Error(),
		})
	}
	fi, err := os.Lstat(clean)
	if err != nil {
		findings = append(findings, SecurityFinding{
			Level:  "critical",
			Topic:  "binary",
			Detail: fmt.Sprintf("cannot stat: %v", err),
		})
		return findings, nil
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		findings = append(findings, SecurityFinding{
			Level:  "critical",
			Topic:  "binary",
			Detail: "supervised binary must not be a symlink",
		})
	}
	if !fi.Mode().IsRegular() {
		findings = append(findings, SecurityFinding{
			Level:  "critical",
			Topic:  "binary",
			Detail: "supervised binary must be a regular file",
		})
	}
	if fi.Size() == 0 {
		findings = append(findings, SecurityFinding{
			Level:  "critical",
			Topic:  "binary",
			Detail: "binary file is empty",
		})
	}
	if home, _ := os.UserHomeDir(); home != "" {
		if underPath(clean, home) {
			findings = append(findings, SecurityFinding{
				Level:  "critical",
				Topic:  "writable tree",
				Detail: fmt.Sprintf("binary under user home %q — unprivileged user can replace the launchd executable", home),
			})
		}
	}
	if strings.HasPrefix(clean, DarwinSystemBinaryDir+"/") || clean == DarwinSystemBinaryPath {
		findings = append(findings, SecurityFinding{
			Level:  "ok",
			Topic:  "install location",
			Detail: "binary is under the recommended root-owned libexec path",
		})
	}
	if perms, err := auditDirectoryChain(filepath.Dir(clean)); err == nil {
		findings = append(findings, perms...)
	} else {
		findings = append(findings, SecurityFinding{
			Level:  "warn",
			Topic:  "directory chain",
			Detail: err.Error(),
		})
	}
	mode := fi.Mode().Perm()
	if mode&0o022 != 0 {
		findings = append(findings, SecurityFinding{
			Level:  "warn",
			Topic:  "binary mode",
			Detail: fmt.Sprintf("binary is group/world writable (%#o)", mode),
		})
	}
	return findings, nil
}

func auditDirectoryChain(dir string) ([]SecurityFinding, error) {
	var findings []SecurityFinding
	dir = filepath.Clean(dir)
	for i := 0; i < 4 && dir != "/" && dir != "."; i++ {
		fi, err := os.Lstat(dir)
		if err != nil {
			return findings, fmt.Errorf("stat %s: %w", dir, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return findings, fmt.Errorf("directory chain contains symlink: %s", dir)
		}
		mode := fi.Mode().Perm()
		if mode&0o002 != 0 {
			findings = append(findings, SecurityFinding{
				Level:  "critical",
				Topic:  "directory permissions",
				Detail: fmt.Sprintf("%s is world-writable (%#o)", dir, mode),
			})
		}
		if mode&0o020 != 0 && fi.Sys() != nil {
			findings = append(findings, SecurityFinding{
				Level:  "warn",
				Topic:  "directory permissions",
				Detail: fmt.Sprintf("%s is group-writable (%#o)", dir, mode),
			})
		}
		dir = filepath.Dir(dir)
	}
	return findings, nil
}

// ValidateBinarySource ensures a candidate binary is safe to copy into a system location.
func ValidateBinarySource(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := validateNoSymlinkInPath(abs); err != nil {
		return err
	}
	fi, err := os.Lstat(abs)
	if err != nil {
		return fmt.Errorf("binary %s: %w", abs, err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("candidate binary must not be a symlink: %s", abs)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("candidate binary must be a regular file: %s", abs)
	}
	if fi.Size() == 0 {
		return fmt.Errorf("candidate binary is empty: %s", abs)
	}
	return nil
}

// ValidateSupervisedBinaryTarget requires the installed daemon path to be outside user-writable trees.
func ValidateSupervisedBinaryTarget(path string) error {
	findings, err := AuditSupervisedBinaryPath(path)
	if err != nil {
		return err
	}
	for _, f := range findings {
		if f.Level == "critical" {
			return fmt.Errorf("%s: %s", f.Topic, f.Detail)
		}
	}
	return nil
}

func validateNoSymlinkInPath(path string) error {
	path = filepath.Clean(path)
	for _, p := range []string{path, filepath.Dir(path)} {
		if p == "." || p == "/" {
			continue
		}
		fi, err := os.Lstat(p)
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path component is a symlink: %s", p)
		}
	}
	return nil
}

func hasCritical(findings []SecurityFinding) bool {
	for _, f := range findings {
		if f.Level == "critical" {
			return true
		}
	}
	return false
}

func underPath(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

// FormatSecurityFindings renders audit output for operators.
func FormatSecurityFindings(findings []SecurityFinding) string {
	var b strings.Builder
	for _, f := range findings {
		b.WriteString(fmt.Sprintf("[%s] %s: %s\n", strings.ToUpper(f.Level), f.Topic, f.Detail))
	}
	return b.String()
}

// PlistBinaryFromInstallSpec returns the supervised binary path recorded for this host.
func PlistBinaryFromLocalSpec(localSpecPath string) (string, bool) {
	spec, ok, err := LoadLocalSpec(localSpecPath)
	if err != nil || !ok || spec.BinaryPath == "" {
		return "", false
	}
	return spec.BinaryPath, true
}

// ResolveUpgradeLivePath picks the supervised binary path for upgrade (local spec, else OS default).
func ResolveUpgradeLivePath(localSpecPath string) string {
	if p, ok := PlistBinaryFromLocalSpec(localSpecPath); ok {
		return p
	}
	if p := DefaultSupervisedBinaryPath(); p != "" {
		return p
	}
	return ""
}

// CurrentUserHome returns the named user's home or the process home.
func CurrentUserHome(username string) string {
	if username != "" {
		if u, err := user.Lookup(username); err == nil && u.HomeDir != "" {
			return u.HomeDir
		}
	}
	home, _ := os.UserHomeDir()
	return home
}
