package config

import (
	"os"
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

func TestValidateStateDirRejectsSymlink(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("unix")
	}
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := ValidateStateDirPath(link); err == nil {
		t.Fatal("expected error for symlink state dir")
	}
}
