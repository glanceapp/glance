package glance

import "sync"

type SingleflightCall[T any] struct {
	done chan struct{}
	val  T
	err  error
}

type Singleflight[T any] struct {
	fn      func() (T, error)
	mu      sync.Mutex
	current *SingleflightCall[T]
}

func (s *Singleflight[T]) Do() (T, error) {
	s.mu.Lock()
	if s.current != nil {
		// Wait for the current call to finish and return its result
		c := s.current
		s.mu.Unlock()
		<-c.done
		return c.val, c.err
	}

	// Initiate a new call
	c := &SingleflightCall[T]{done: make(chan struct{})}
	s.current = c
	s.mu.Unlock()

	c.val, c.err = s.fn()

	s.mu.Lock()
	s.current = nil
	s.mu.Unlock()
	close(c.done)
	return c.val, c.err
}

func NewSingleflight[T any](fn func() (T, error)) func() (T, error) {
	sf := &Singleflight[T]{fn: fn}
	return sf.Do
}
