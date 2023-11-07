package loggingwaitgroup

import (
	"log"
	"sync"
)

const shouldLog = false

type WaitGroup struct {
	sync.WaitGroup
	name string
}

func (wg *WaitGroup) Add(name string) {
	if shouldLog {
		log.Println("add", name)
	}
	wg.WaitGroup.Add(1)
}

func (wg *WaitGroup) Done(name string) {
	if shouldLog {
		log.Println("done", name)
	}
	wg.WaitGroup.Done()
}
