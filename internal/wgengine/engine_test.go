package wgengine

import (
	"testing"

	"github.com/gcostello02/mynet/internal/wgbind"
	"golang.zx2c4.com/wireguard/tun/tuntest"
)

func TestEngineNew(t *testing.T) {
	ch := tuntest.NewChannelTUN()
	_, err := New(ch.TUN(), wgbind.NewICEBind())
	if err != nil {
		t.Fatal(err)
	}
}
