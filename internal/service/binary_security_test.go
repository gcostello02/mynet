package service

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAuditFlagsHomeBinaryAsCritical(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(home, ".mynet-audit-test-binary")
	t.Cleanup(func() { _ = os.Remove(bin) })
	if err := os.WriteFile(bin, []byte{0xcf, 0xfa}, 0o755); err != nil {
		t.Fatal(err)
	}
	findings, err := AuditSupervisedBinaryPath(bin)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCritical(findings) {
		t.Fatalf("expected critical for home binary: %v", findings)
	}
}

func TestValidateBinarySourceRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "mynet")
	if err := os.WriteFile(real, []byte{1}, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBinarySource(link); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestInstallDryRunDarwinSystemBinaryPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "identity.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "candidate")
	if err := os.WriteFile(bin, []byte{0xcf, 0xfa}, 0o755); err != nil {
		t.Fatal(err)
	}
	spec, err := Install(InstallOptions{
		BinaryPath:   bin,
		StateDir:     dir,
		OwnerUser:    os.Getenv("USER"),
		DryRun:       true,
		SystemBinary: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.BinaryPath != DarwinSystemBinaryPath {
		t.Fatalf("plist binary: got %q want %q", spec.BinaryPath, DarwinSystemBinaryPath)
	}
	if spec.SourceBinary != bin {
		t.Fatalf("source: got %q", spec.SourceBinary)
	}
}

func TestInstallRejectsHomePlistBinaryWithoutSystemFlag(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "identity.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(home, ".mynet-install-refuse-test")
	t.Cleanup(func() { _ = os.Remove(bin) })
	if err := os.WriteFile(bin, []byte{1}, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = Install(InstallOptions{
		BinaryPath:   bin,
		StateDir:     dir,
		OwnerUser:    os.Getenv("USER"),
		DryRun:       true,
		SystemBinary: false,
	})
	if err == nil {
		t.Fatal("expected refusal to point launchd at user-writable binary")
	}
}

func TestLaunchDaemonPlistUsesLibexecPath(t *testing.T) {
	p := LaunchDaemonPlist(DarwinSystemBinaryPath, "/Users/u/.mynet", "u", "/var/log/mynet/daemon.log")
	if !containsAll(p, DarwinSystemBinaryPath, "_daemon") {
		t.Fatalf("plist missing libexec binary path")
	}
}
