package client

import "time"

// ICETraceHook receives timestamped ICE stage events (tests/diagnostics only).
type ICETraceHook func(stage string, detail map[string]string)

var iceTraceHook ICETraceHook

// SetICETraceHook installs optional stage logging. Pass nil to disable.
func SetICETraceHook(h ICETraceHook) {
	iceTraceHook = h
}

func iceTrace(stage string, detail map[string]string) {
	if iceTraceHook == nil {
		return
	}
	if detail == nil {
		detail = map[string]string{}
	}
	detail["ts"] = time.Now().Format(time.RFC3339Nano)
	iceTraceHook(stage, detail)
}
