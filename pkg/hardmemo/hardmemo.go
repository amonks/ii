package hardmemo

import (
	"encoding/gob"
	"errors"
	"log"
	"os"
	"time"

	"monks.co/pkg/env"
	"monks.co/pkg/flock"
)

type FuncOf[T any] func() (T, error)
type Cached[T any] struct {
	Value T
	Error string
}

func Memoize[T any](name string, dur time.Duration, fn FuncOf[T]) FuncOf[T] {
	var zero T

	var (
		filename  = env.InMonksData(name)
		lockfile  = filename + ".lock"
		cachefile = filename + ".gob"
	)

	mustHaveFile(lockfile)
	mustHaveFile(cachefile)

	return func() (T, error) {
		lock, err := flock.Lock(name + ".lock")
		if err != nil {
			return zero, err
		}
		defer lock.Unlock()

		if fileinfo, err := os.Stat(cachefile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return zero, err
		} else if err == nil && fileinfo.ModTime().After(time.Now().Add(-dur)) && fileinfo.Size() > 0 {
			log.Printf("returning %s from cache", name)

			cachefile, err := os.Open(cachefile)
			if err != nil {
				return zero, err
			}
			dec := gob.NewDecoder(cachefile)
			var cached Cached[T]
			if err := dec.Decode(&cached); err != nil {
				return zero, err
			}
			if cached.Error != "" {
				return zero, errors.New(cached.Error)
			}
			return cached.Value, nil
		} else {
			log.Printf("calculating %s", name)

			value, err := fn()
			var cache Cached[T]
			if err != nil {
				cache = Cached[T]{Error: err.Error()}
			} else {
				cache = Cached[T]{Value: value}
			}

			cachefile, err := os.OpenFile(cachefile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				return zero, err
			}
			enc := gob.NewEncoder(cachefile)
			if err := enc.Encode(cache); err != nil {
				return zero, err
			}

			if cache.Error != "" {
				return zero, errors.New(cache.Error)
			} else {
				return cache.Value, nil
			}
		}
	}
}

func mustHaveFile(filename string) {
	if _, err := os.Stat(filename); err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	} else if err != nil {
		f, err := os.Create(filename)
		if err != nil {
			panic(err)
		}
		f.Close()
	}
}
