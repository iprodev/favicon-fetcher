// Package cache provides request deduplication using the singleflight pattern.
// This prevents "thundering herd" problems where multiple concurrent requests
// for the same resource cause multiple fetches.
package cache

import (
	"sync"
)

// Group represents a group of duplicate-suppressed function calls.
// It coordinates multiple simultaneous requests for the same key,
// ensuring only one execution happens while others wait for the result.
// This prevents "thundering herd" problem where multiple concurrent
// requests for the same resource cause multiple fetches.
type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

type call struct {
	wg  sync.WaitGroup
	val []byte
	err error
}

// NewGroup creates a new Group instance for request deduplication.
func NewGroup() *Group {
	return &Group{
		m: make(map[string]*call),
	}
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a time.
// If a duplicate comes in, the duplicate caller waits for the original
// to complete and receives the same results.
func (g *Group) Do(key string, fn func() ([]byte, error)) ([]byte, error) {
	g.mu.Lock()
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := &call{}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
