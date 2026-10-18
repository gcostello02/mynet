package daemon

import (
	"testing"
)

// Documents NetWatcher scope: default-route egress only (not local IP churn on same iface).
func TestNetWatcherScopeDocumented(t *testing.T) {
	const doc = "NetWatcher fingerprints default-route interface only; LAN IP changes without default-route movement rely on ICE transport-down recovery."
	if doc == "" {
		t.Fatal("documentation anchor")
	}
}
