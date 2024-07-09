package flock

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type Flock struct {
	f *os.File
}

func Lock(filename string) (*Flock, error) {
	f, _ := os.Create(filename)
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		return nil, fmt.Errorf("error locking %s: %w", filename, err)
	}
	return &Flock{f}, nil
}

func (flock *Flock) Unlock() {
	if err := unix.Flock(int(flock.f.Fd()), unix.LOCK_UN); err != nil {
		panic(fmt.Errorf("error unlocking %s: %w", flock.f.Name(), err))
	}
	if err := flock.f.Close(); err != nil {
		panic(err)
	}
}
