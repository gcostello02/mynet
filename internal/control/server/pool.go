package server

import (
	"fmt"
	"net"
)

// IPInPool reports whether ip is contained in pool (IPv4).
func IPInPool(ip net.IP, pool *net.IPNet) bool {
	if ip == nil || pool == nil {
		return false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return pool.Contains(ip4)
}

// AssignVPNIPv4 sets a device's VPN address if it is in pool and not used by another device.
func (s *Store) AssignVPNIPv4(deviceID string, want string, pool *net.IPNet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ip := net.ParseIP(want)
	if ip == nil || !IPInPool(ip, pool) {
		return fmt.Errorf("address %q is not inside VPN pool %s", want, pool.String())
	}
	dev, ok := s.devices[deviceID]
	if !ok || dev.Revoked {
		return fmt.Errorf("unknown device")
	}
	for _, d := range s.devices {
		if d.Revoked || d.DeviceID == deviceID {
			continue
		}
		if d.VPNIPv4 == want {
			return fmt.Errorf("address %s already assigned to %s", want, d.Name)
		}
	}
	dev.VPNIPv4 = want
	if s.dbPath != "" {
		_ = PersistDevice(s.dbPath, dev)
	}
	return nil
}

func incIPv4(ip net.IP) net.IP {
	ip4 := ip.To4()
	if ip4 == nil {
		return ip
	}
	out := make(net.IP, 4)
	copy(out, ip4)
	for i := 3; i >= 0; i-- {
		out[i]++
		if out[i] != 0 {
			break
		}
	}
	return out
}
