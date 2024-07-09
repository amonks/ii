package periodically

import (
	"log"
	"sync"
	"time"
)

type Periodic[T any] struct {
	stopped bool
	mu      sync.Mutex

	v   T
	err error
}

func Do[T any](dur time.Duration, f func() (T, error)) *Periodic[T] {
	val, err := f()
	p := &Periodic[T]{
		err: err,
		v:   val,
	}
	go func() {
		for {
			time.Sleep(dur)

			log.Println("reload")
			if p.isStopped() {
				return
			}
			val, err := f()
			p.set(val, err)
		}
	}()
	return p
}

func (p *Periodic[T]) isStopped() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopped
}

func (p *Periodic[T]) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
}

func (p *Periodic[T]) get() (T, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.v, p.err
}

func (p *Periodic[T]) set(v T, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.v, p.err = v, err
}
