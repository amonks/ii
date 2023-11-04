package loggingwaitgroup

import (
	"fmt"
	"sync"
)

const log = false

type WaitGroup struct {
	sync.WaitGroup
	name string
}

func (wg *WaitGroup) Add(name string) {
	if log {
		fmt.Println("add", name)
	}
	wg.WaitGroup.Add(1)
}

func (wg *WaitGroup) Done(name string) {
	if log {
		fmt.Println("done", name)
	}
	wg.WaitGroup.Done()
}
