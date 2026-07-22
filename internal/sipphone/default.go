package sipphone

import "sync"

var defaultManager struct {
	sync.RWMutex
	manager *Manager
}

func SetDefaultManager(manager *Manager) {
	defaultManager.Lock()
	defaultManager.manager = manager
	defaultManager.Unlock()
}

func DefaultManager() *Manager {
	defaultManager.RLock()
	defer defaultManager.RUnlock()
	return defaultManager.manager
}
