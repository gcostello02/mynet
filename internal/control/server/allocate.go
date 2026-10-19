package server

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

// AllocateVPNIPv4 assigns a stable address in pool or updates WG key for device.
func (s *Store) AllocateVPNIPv4(deviceID string, wgPublicKeyHex string, pool *net.IPNet) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dev, ok := s.devices[deviceID]
	if !ok || dev.Revoked {
		return "", fmt.Errorf("unknown device")
	}
	wgPublicKeyHex = strings.ToLower(wgPublicKeyHex)
	if dev.VPNIPv4 != "" && dev.WGPublicKey != "" && dev.WGPublicKey != wgPublicKeyHex {
		return "", fmt.Errorf("wg public key already registered for this device")
	}
	for _, d := range s.devices {
		if d.Revoked || d.DeviceID == dev.DeviceID {
			continue
		}
		if d.WGPublicKey == wgPublicKeyHex {
			return "", fmt.Errorf("wg public key already used by another device")
		}
	}

	if dev.VPNIPv4 != "" {
		if !IPInPool(net.ParseIP(dev.VPNIPv4), pool) {
			dev.VPNIPv4 = ""
		}
	}
	if dev.VPNIPv4 == "" {
		ip, err := s.nextFreeHost(pool)
		if err != nil {
			return "", err
		}
		dev.VPNIPv4 = ip
	}
	dev.WGPublicKey = wgPublicKeyHex
	if s.dbPath != "" {
		_ = PersistDevice(s.dbPath, dev)
	}
	return dev.VPNIPv4, nil
}

func (s *Store) nextFreeHost(pool *net.IPNet) (string, error) {
	used := make(map[string]bool)
	for _, d := range s.devices {
		if d.VPNIPv4 != "" {
			used[d.VPNIPv4] = true
		}
	}
	mask := pool.Mask
	network := pool.IP.To4().Mask(mask)
	if network == nil {
		return "", fmt.Errorf("vpn pool must be ipv4")
	}
	for ip := incIPv4(network); pool.Contains(ip); ip = incIPv4(ip) {
		if isNetworkOrBroadcast(ip, mask) {
			continue
		}
		s := ip.String()
		if !used[s] {
			return s, nil
		}
	}
	return "", fmt.Errorf("no free addresses in pool")
}

func isNetworkOrBroadcast(ip net.IP, mask net.IPMask) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return true
	}
	network := ip4.Mask(mask)
	if ip4.Equal(network) {
		return true
	}
	broadcast := make(net.IP, 4)
	for i := range ip4 {
		broadcast[i] = ip4[i] | ^mask[i]
	}
	return ip4.Equal(broadcast)
}

// HostIndexForTests exposes deterministic ordering for unit tests.
func HostIndex(ip net.IP) uint16 {
	ip4 := ip.To4()
	if ip4 == nil {
		return 0
	}
	return binary.BigEndian.Uint16(ip4[2:4])
}
