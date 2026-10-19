package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/grantcostello/mynet/internal/config"
)

func TestShouldSpawnDetachedDaemonNoUnit(t *testing.T) {
	dir := t.TempDir()
	ok, _ := ShouldSpawnDetachedDaemon(dir, filepath.Join(dir, "service.json"))
	if !ok {
		t.Fatal("expected spawn allowed without unit")
	}
}

func TestShouldSpawnDetachedDaemonUnitWithoutDaemon(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MYNET_SERVICE_ROOT", root)
	layout := SystemLayoutForOS()
	if runtime.GOOS == "darwin" {
		_ = os.MkdirAll(filepath.Dir(layout.LaunchDaemonPlist), 0o755)
		_ = writeFileAtomic(layout.LaunchDaemonPlist, "<plist/>", 0o644)
	} else {
		_ = os.MkdirAll(filepath.Dir(layout.SystemdUnit), 0o755)
		_ = writeFileAtomic(layout.SystemdUnit, "[Service]\n", 0o644)
	}
	state := filepath.Join(root, "state")
	ok, msg := ShouldSpawnDetachedDaemon(state, filepath.Join(state, "service.json"))
	if ok {
		t.Fatalf("expected spawn blocked when unit present")
	}
	if msg == "" {
		t.Fatal("expected message")
	}
}

func TestPreflightInstallRejectsSymlinkState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	base := t.TempDir()
	target := filepath.Join(base, "real")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "identity.json"), []byte(`{"device_id":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateStateDirPath(link); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestPreflightInstallRequiresExistingIdentity(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(t.TempDir(), "mynet")
	if err := os.WriteFile(bin, []byte{0x7f, 'E', 'L', 'F'}, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := PreflightInstall(dir, bin); err == nil {
		t.Fatal("expected error without identity.json")
	}
}

func TestPreflightInstallBinaryChecks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "identity.json"), []byte(`x`), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "mynet")
	if err := os.WriteFile(bin, []byte{0}, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := PreflightInstall(dir, bin); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestEnsureDaemonStoppedCleanDir(t *testing.T) {
	if err := EnsureDaemonStopped(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchdThrottleAndKeepAlive(t *testing.T) {
	p := LaunchDaemonPlist("/bin/mynet", "/Users/u/.mynet", "u", "/var/log/mynet/daemon.log")
	if !containsAll(p, "ThrottleInterval", "SuccessfulExit", "_daemon") {
		t.Fatalf("missing launchd controls")
	}
}

func TestLaunchDaemonPlistPassesPlutil(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS plutil")
	}
	p := LaunchDaemonPlist("/usr/local/bin/mynet", "/Users/u/.mynet", "u", "/var/log/mynet/daemon.log")
	path := filepath.Join(t.TempDir(), "com.mynet.daemon.plist")
	if err := os.WriteFile(path, []byte(p), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("plutil", "-lint", path).CombinedOutput()
	if err != nil {
		t.Fatalf("plutil: %v %s", err, out)
	}
}

func TestSystemdRestartPolicy(t *testing.T) {
	u := SystemdUnit("/bin/mynet", "/etc/mynet/client.env")
	if !containsAll(u, "Restart=on-failure", "StartLimitBurst=5", "TimeoutStopSec=30") {
		t.Fatalf("missing systemd policy")
	}
	ok, detail := StartLimitKeysInUnitSection(u)
	if !ok {
		t.Fatalf("start limits: %s", detail)
	}
}

func TestSystemdUnitPassesAnalyzeVerify(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd-analyze on Linux")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "mynet")
	if err := os.WriteFile(bin, []byte{0x7f, 'E', 'L', 'F'}, 0o755); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(dir, "client.env")
	if err := os.WriteFile(env, []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	u := SystemdUnit(bin, env)
	path := filepath.Join(dir, "mynet-daemon.service")
	if err := os.WriteFile(path, []byte(u), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("systemd-analyze", "verify", path).CombinedOutput()
	if err != nil {
		t.Fatalf("systemd-analyze verify: %v %s", err, out)
	}
}
