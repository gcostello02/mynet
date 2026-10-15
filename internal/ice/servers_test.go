package icenet

import (
	"strings"
	"testing"

	"github.com/gcostello02/mynet/internal/control"
)

func TestPionURLsFromICEServersTURNCreds(t *testing.T) {
	urls, err := pionURLsFromICEServers([]control.ICEServer{{
		URLs:       []string{"turn:example.com:3478?transport=udp"},
		Username:   "user:1",
		Credential: "pass+/=",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 1 {
		t.Fatalf("got %d urls", len(urls))
	}
	if urls[0].Username != "user:1" || urls[0].Password != "pass+/=" {
		t.Fatalf("creds not set: %#v", urls[0])
	}
	if strings.Contains(urls[0].String(), "password=") {
		t.Fatalf("credentials should not be in query: %s", urls[0].String())
	}
}
