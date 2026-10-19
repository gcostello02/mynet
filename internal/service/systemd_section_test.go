package service

import (
	"strings"
	"testing"
)

func TestSystemdStartLimitInUnitSection(t *testing.T) {
	u := SystemdUnit("/bin/mynet", "/etc/mynet/client.env")
	ok, detail := StartLimitKeysInUnitSection(u)
	if !ok {
		t.Fatalf("start limit placement: %s", detail)
	}
	if !strings.Contains(u, "Restart=on-failure") || !strings.Contains(u, "RestartSec=5") {
		t.Fatalf("missing service restart policy")
	}
	if strings.Contains(systemdSectionText(u, "Service"), "StartLimitIntervalSec") {
		t.Fatal("StartLimitIntervalSec leaked into [Service]")
	}
}

func TestSystemdCapabilityDACForUserState(t *testing.T) {
	u := SystemdUnit("/bin/mynet", "/etc/mynet/client.env")
	if !strings.Contains(u, "CAP_DAC_OVERRIDE") || !strings.Contains(u, "CAP_DAC_READ_SEARCH") {
		t.Fatalf("unit must include DAC capabilities for root+user state dir access")
	}
}

func systemdSectionText(unit, name string) string {
	return strings.Join(systemdSection(unit, name), "\n")
}
