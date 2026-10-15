package server

func deviceCount(store *Store) int {
	store.mu.RLock()
	n := len(store.devices)
	store.mu.RUnlock()
	return n
}
