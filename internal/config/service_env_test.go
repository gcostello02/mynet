package config

import (
	"path/filepath"
	"testing"
)

func TestValidateStateDirPath(t *testing.T) {
	dir := t.TempDir()
	if err := ValidateStateDirPath(dir); err != nil {
		t.Fatal(err)
	}
	if err := ValidateStateDirPath(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected missing error")
	}
}

func TestOwnerUsernameFallback(t *testing.T) {
	t.Setenv(EnvOwnerUser, "alice")
	if OwnerUsername() != "alice" {
		t.Fatalf("got %q", OwnerUsername())
	}
}
