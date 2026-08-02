package syncex

import "sync"

// Slice is a mutex-protected slice safe for concurrent use.
type Slice[T any] struct {
	s  []T
	mu sync.Mutex
}

// Do locks the slice, passes its current contents to fn, and stores the
// slice returned by fn as the new contents.
func (s *Slice[T]) Do(fn func(v []T) []T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.s = fn(s.s)
}
