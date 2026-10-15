package tundev

import (
	"runtime"
	"testing"
)

func TestCreateTUNNameDarwinUsesAutoUtun(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin only")
	}
	if createTUNName("mynet0") != "utun" {
		t.Fatal("darwin must ignore mynet0 and use utun for auto allocation")
	}
	if createTUNName("") != "utun" {
		t.Fatal("darwin empty request must use utun")
	}
	if createTUNName("utun7") != "utun" {
		t.Fatal("darwin must not hardcode utun index")
	}
}

func TestCreateTUNNameLinuxUsesMynet0(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	if createTUNName("") != "mynet0" {
		t.Fatal("linux default")
	}
	if createTUNName("mynet0") != "mynet0" {
		t.Fatal("linux respects requested name")
	}
}

func TestPlannedTUNDescription(t *testing.T) {
	desc := plannedTUNDescription()
	switch runtime.GOOS {
	case "darwin":
		if desc != "utun (OS-assigned)" {
			t.Fatalf("got %q", desc)
		}
	default:
		if desc != "mynet0" {
			t.Fatalf("got %q", desc)
		}
	}
}
