package service

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/grantcostello/mynet/internal/config"
)

func TestLaunchDaemonTemplate(t *testing.T) {
	plist := LaunchDaemonPlist("/usr/local/bin/mynet", "/Users/u/.mynet", "u", "/var/log/mynet/daemon.log")
	label, ok := ParseLaunchDaemonPlistLabel(plist)
	if !ok || label != Label {
		t.Fatalf("label parse: %v %q", ok, label)
	}
	if !containsAll(plist, config.EnvStateDir, "/Users/u/.mynet", config.EnvSupervised, config.EnvRunVPNAtStart) {
		t.Fatalf("missing env keys in plist")
	}
}

func TestSystemdTemplate(t *testing.T) {
	unit := SystemdUnit("/opt/mynet/mynet", "/etc/mynet/client.env")
	exec, ok := ParseSystemdExecStart(unit)
	if !ok || exec != "/opt/mynet/mynet _daemon" {
		t.Fatalf("exec start: %q ok=%v", exec, ok)
	}
}

func TestInstallDryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix only")
	}
	dir := t.TempDir()
	state := filepath.Join(dir, ".mynet")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	id := filepath.Join(state, "identity.json")
	if err := os.WriteFile(id, []byte(`{"device_id":"x","name":"t","public_key":"k"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	spec, err := Install(InstallOptions{
		BinaryPath: filepath.Join(dir, "mynet"),
		StateDir:   state,
		OwnerUser:  os.Getenv("USER"),
		DryRun:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.StateDir != state {
		t.Fatalf("state dir: %s", spec.StateDir)
	}
}

func TestUpgradeStage(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, "mynet")
	cand := filepath.Join(dir, "cand")
	if err := os.WriteFile(live, []byte("live"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cand, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanUpgrade(live, cand, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := StageCandidate(plan); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(plan.StagedPath)
	if string(b) != "new" {
		t.Fatalf("staged: %q", b)
	}
}

func TestEnvFileContents(t *testing.T) {
	s := EnvFileContents("/home/u/.mynet", "u", "/bin/mynet")
	if !containsAll(s, "MYNET_STATE_DIR=/home/u/.mynet", "MYNET_OWNER_USER=u", "MYNET_SUPERVISED=1") {
		t.Fatalf("env: %s", s)
	}
}

func containsAll(hay string, parts ...string) bool {
	for _, p := range parts {
		if !contains(hay, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
