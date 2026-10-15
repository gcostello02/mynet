package wgkeys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateStable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	a, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a.PublicKey != b.PublicKey {
		t.Fatal("expected stable wg key")
	}
	st, err := os.Stat(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perms %o", st.Mode().Perm())
	}
}
