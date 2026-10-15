package control

import "testing"

func TestWebSocketURLScheme(t *testing.T) {
	// Explicit URL construction check without dialing.
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"https://mesh.example.com", "wss"},
		{"http://10.0.0.1:8787", "ws"},
	} {
		c := NewAPIClient(tc.in)
		c.Token = "t"
		u, err := c.webSocketURL()
		if err != nil {
			t.Fatal(err)
		}
		if u.Scheme != tc.want {
			t.Fatalf("%s: scheme %q want %q", tc.in, u.Scheme, tc.want)
		}
		if u.Path != "/v1/ws" {
			t.Fatalf("path %q", u.Path)
		}
		if u.Query().Get("token") != "t" {
			t.Fatal("token query missing")
		}
	}
}
