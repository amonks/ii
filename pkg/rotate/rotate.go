package rotate

import "sync"

type Rotator[T any] struct {
	options []T
	i       int
	mu      sync.Mutex
}

func New[T any](ts ...T) *Rotator[T] {
	return &Rotator[T]{options: ts}
}

func (rot *Rotator[T]) Next() T {
	rot.mu.Lock()
	defer rot.mu.Unlock()
	got := rot.options[rot.i%len(rot.options)]
	rot.i++
	return got
}
