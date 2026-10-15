package client

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestICETraceHookConcurrentRegisterAndEmit(t *testing.T) {
	var calls atomic.Int64
	hook := func(stage string, detail map[string]string) {
		calls.Add(1)
		if stage == "" {
			t.Error("empty stage")
		}
	}

	const workers = 24
	const iters = 200
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				SetICETraceHook(hook)
				SetICETraceHook(nil)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				iceTrace("concurrent_emit", map[string]string{"j": "1"})
			}
		}()
	}
	wg.Wait()
	SetICETraceHook(nil)
	if calls.Load() == 0 {
		t.Fatal("expected at least one trace invocation")
	}
}
