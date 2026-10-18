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

	// Prove hook + emit work on this goroutine before racing register/emit workers.
	SetICETraceHook(hook)
	iceTrace("preflight", map[string]string{"phase": "sync"})
	SetICETraceHook(nil)
	baseline := calls.Load()
	if baseline != 1 {
		t.Fatalf("preflight: expected 1 hook call, got %d", baseline)
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

	if calls.Load() < baseline {
		t.Fatalf("calls regressed during stress: %d < %d", calls.Load(), baseline)
	}

	// Registration must still work after concurrent churn (stress may have invoked hook too).
	afterStress := calls.Load()
	SetICETraceHook(hook)
	iceTrace("postflight", map[string]string{"phase": "sync"})
	SetICETraceHook(nil)
	if calls.Load() != afterStress+1 {
		t.Fatalf("postflight: expected %d hook calls, got %d", afterStress+1, calls.Load())
	}
}
