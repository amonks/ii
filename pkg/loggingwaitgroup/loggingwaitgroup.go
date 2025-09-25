package loggingwaitgroup

import (
	"fmt"
	"log"
	"sync"
)

const shouldLog = false

type WaitGroup struct {
	sync.WaitGroup
	name string
}

func (wg *WaitGroup) Add(name string) {
	wg.println("add", name)
	wg.WaitGroup.Add(1)
}

func (wg *WaitGroup) Done(name string) {
	wg.println("done", name)
	wg.WaitGroup.Done()
}

func (wg *WaitGroup) println(vs ...any) {
	if !shouldLog {
		return
	}

	if wg.name != "" {
		vs = append([]any{fmt.Sprintf("[%s]", wg.name)}, vs...)
	}
	log.Println(vs...)
}
