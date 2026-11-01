//go:build linux

package staging

import (
	"os"
	"path/filepath"
	"testing"
)

// Optional post-run check after operator executes deploy/staging/scripts/run-acceptance.sh.
func TestArtifactsAfterRealStagingRun(t *testing.T) {
	root := os.Getenv("MYNET_STAGING_ROOT")
	if root == "" {
		t.Skip("set MYNET_STAGING_ROOT to deploy/staging after a real staging run")
	}
	dp := filepath.Join(root, "artifacts", "data-plane-results.txt")
	if _, err := os.Stat(dp); err != nil {
		t.Skip("data-plane-results.txt missing; run scripts/data-plane.sh first")
	}
	b, err := os.ReadFile(dp)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("empty data-plane results")
	}
}
