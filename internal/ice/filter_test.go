package icenet

import (
	"net"
	"testing"
)

func TestIPAllowFilterTailscaleCIDR(t *testing.T) {
	_, n, _ := net.ParseCIDR("100.64.0.0/10")
	allow := IPAllowFilter([]*net.IPNet{n})
	if allow(net.ParseIP("100.125.1.1")) {
		t.Fatal("expected 100.x blocked")
	}
	if !allow(net.ParseIP("10.0.0.1")) {
		t.Fatal("expected 10.x allowed")
	}
}

func TestInterfaceAllowFilter(t *testing.T) {
	allow := InterfaceAllowFilter([]string{"tailscale0", "utun*"})
	if allow("tailscale0") {
		t.Fatal("tailscale0 should be excluded")
	}
	if allow("utun99") {
		t.Fatal("utun* should be excluded")
	}
	if !allow("en0") {
		t.Fatal("en0 should be allowed")
	}
}

func TestInterfaceAllowFilterDockerAndMynet(t *testing.T) {
	allow := InterfaceAllowFilter(DefaultIndependentExcludePatterns())
	if allow("docker0") || allow("br-abc123") || allow("mynet0") || allow("utun16") {
		t.Fatal("virtual interfaces should be excluded")
	}
	if !allow("en0") && !allow("enp1s0") {
		t.Fatal("physical ethernet should be allowed")
	}
}

// DefaultIndependentExcludePatterns mirrors config defaults for unit tests in this package.
func DefaultIndependentExcludePatterns() []string {
	return []string{
		"tailscale0", "utun*", "mynet*", "lo", "lo0",
		"docker*", "br-*", "veth*", "virbr*",
	}
}
