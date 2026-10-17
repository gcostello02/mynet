package daemon

import (
	"sync"
	"time"
)

const defaultShutdownWait = 10 * time.Second

// shutdown coordinates daemon exit (quit) without killing the CLI process.
type shutdownCoordinator struct {
	once sync.Once
	done chan struct{}
}

func newShutdownCoordinator() *shutdownCoordinator {
	return &shutdownCoordinator{done: make(chan struct{})}
}

func (c *shutdownCoordinator) trigger(fn func()) {
	c.once.Do(func() {
		go func() {
			fn()
			close(c.done)
		}()
	})
}

func (c *shutdownCoordinator) wait(timeout time.Duration) {
	if c == nil {
		return
	}
	select {
	case <-c.done:
	case <-time.After(timeout):
	}
}
