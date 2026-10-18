package daemon

import (
	"sync"
	"time"
)

var activeService struct {
	sync.Mutex
	s *Service
}

func registerActiveService(s *Service) {
	activeService.Lock()
	activeService.s = s
	activeService.Unlock()
}

func unregisterActiveService() {
	activeService.Lock()
	activeService.s = nil
	activeService.Unlock()
}

// GracefulShutdownFromSignal stops the in-process IPC server (SIGTERM/SIGINT).
func GracefulShutdownFromSignal(wait time.Duration) bool {
	activeService.Lock()
	s := activeService.s
	activeService.Unlock()
	if s == nil {
		return false
	}
	s.requestQuit()
	if s.shutdown != nil {
		s.shutdown.wait(wait)
	}
	return true
}
