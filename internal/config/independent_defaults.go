package config

import "strings"

// DefaultIndependentExcludeInterfaces are applied in independent mode (merged with user list).
var DefaultIndependentExcludeInterfaces = []string{
	"tailscale0",
	"utun*",
	"mynet*",
	"lo",
	"lo0",
	"docker*",
	"br-*",
	"veth*",
	"virbr*",
	"cni*",
	"flannel*",
	"tunl*",
	"kube-*",
}

// DefaultIndependentExcludeCIDRs block virtual overlay host addresses in independent mode.
var DefaultIndependentExcludeCIDRs = []string{
	DefaultExcludeTailscaleCIDR,
	"127.0.0.0/8",
	"172.16.0.0/12", // Docker default bridge pools (172.16–172.31)
}

// EffectiveExcludeInterfaces returns interface name patterns skipped for ICE gathering.
func (c ClientConfig) EffectiveExcludeInterfaces() []string {
	if !c.Network.IndependentMode {
		return append([]string(nil), c.Network.ExcludeInterfaces...)
	}
	return mergeStrings(DefaultIndependentExcludeInterfaces, c.Network.ExcludeInterfaces)
}

func mergeStrings(a, b []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range append(a, b...) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
