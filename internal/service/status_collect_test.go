package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectHealthInstalledWithUnreadableLocalSpec(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "service.json")
	if err := os.WriteFile(spec, []byte(`{"supervisor":"systemd"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(spec, 0o000)
	defer os.Chmod(spec, 0o600)

	present, readable, err := localSpecPresence(spec)
	if err != nil || !present || readable {
		t.Fatalf("present=%v readable=%v err=%v", present, readable, err)
	}

	hr := CollectHealth(dir, spec)
	if !hr.Installed {
		t.Fatal("expected installed when metadata file exists even if unreadable")
	}
	if !hr.LocalSpecPresent || hr.LocalSpecReadable {
		t.Fatalf("local spec flags: present=%v readable=%v", hr.LocalSpecPresent, hr.LocalSpecReadable)
	}
}
