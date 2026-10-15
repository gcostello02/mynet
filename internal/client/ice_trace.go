package client

import (
	"sync"
	"time"
)

// ICETraceHook receives timestamped ICE stage events (tests/diagnostics only).
type ICETraceHook func(stage string, detail map[string]string)

var (
	iceTraceMu   sync.RWMutex
	iceTraceHook ICETraceHook
)

// SetICETraceHook installs optional stage logging. Pass nil to disable.
func SetICETraceHook(h ICETraceHook) {
	iceTraceMu.Lock()
	iceTraceHook = h
	iceTraceMu.Unlock()
}

func iceTrace(stage string, detail map[string]string) {
	iceTraceMu.RLock()
	h := iceTraceHook
	iceTraceMu.RUnlock()
	if h == nil {
		return
	}
	if detail == nil {
		detail = map[string]string{}
	} else {
		detail = cloneStringMap(detail)
	}
	detail["ts"] = time.Now().Format(time.RFC3339Nano)
	h(stage, detail)
}

func cloneStringMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
