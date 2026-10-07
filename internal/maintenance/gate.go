// Package maintenance coordinates short application-wide maintenance windows.
package maintenance

import "sync"

type Gate struct {
	mu sync.RWMutex
}

func (g *Gate) ReadLock() func() {
	g.mu.RLock()
	return g.mu.RUnlock
}

func (g *Gate) Lock() func() {
	g.mu.Lock()
	return g.mu.Unlock
}
