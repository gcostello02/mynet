package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveStateDirExplicitEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvStateDir, dir)
	got, err := ResolveStateDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("got %q want %q", got, dir)
	}
}

func TestResolveStateDirDefault(t *testing.T) {
	t.Setenv(EnvStateDir, "")
	home, _ := os.UserHomeDir()
	got, err := ResolveStateDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".mynet")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
