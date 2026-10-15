package tundev

import (
	"net/netip"
	"os"
	"testing"
)

func TestStartupStepOrder(t *testing.T) {
	want := []string{
		"open_tun",
		"assign_address",
		"activate_interface",
		"wireguard_up",
		"peer_routes",
		"wireguard_peers",
		"ice_attach",
	}
	if len(startupSteps) != len(want) {
		t.Fatalf("steps %v", startupSteps)
	}
	for i := range want {
		if startupSteps[i] != want[i] {
			t.Fatalf("step %d: %q want %q", i, startupSteps[i], want[i])
		}
	}
	if StartupStepCount() != len(want) {
		t.Fatal()
	}
}

func TestLinuxTUNIntegration(t *testing.T) {
	if os.Getenv("MYNET_TUN_INTEGRATION") == "" {
		t.Skip("set MYNET_TUN_INTEGRATION=1 and run as root to create a disposable TUN")
	}
	if os.Geteuid() != 0 {
		t.Skip("root required for TUN integration test")
	}
	mgr := NewManager()
	dev, name, err := mgr.Open("mynet0-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = mgr.Cleanup()
	}()
	if dev == nil || name == "" {
		t.Fatal("missing tun")
	}
	pfx := mustPfx("10.77.99.2/32")
	if err := mgr.Configure(pfx); err != nil {
		t.Fatal(err)
	}
	peer := mustAddr("10.77.99.1")
	if err := mgr.AddPeerRoute(peer); err != nil {
		t.Fatal(err)
	}
}

func mustPfx(s string) netip.Prefix {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		panic(err)
	}
	return p
}

func mustAddr(s string) netip.Addr {
	a, err := netip.ParseAddr(s)
	if err != nil {
		panic(err)
	}
	return a
}
