package tundev

import "runtime"

// createTUNName returns the interface name passed to wireguard-go tun.CreateTUN.
//
// Linux uses a persistent name (default mynet0). macOS must use the literal "utun"
// so the kernel allocates the next free utunN (see wireguard-go tun_darwin.go).
func createTUNName(requested string) string {
	switch runtime.GOOS {
	case "darwin":
		return "utun"
	case "linux":
		if requested != "" {
			return requested
		}
		return "mynet0"
	default:
		if requested != "" {
			return requested
		}
		return "mynet0"
	}
}

func plannedTUNDescription() string {
	switch runtime.GOOS {
	case "darwin":
		return "utun (OS-assigned)"
	default:
		return "mynet0"
	}
}
