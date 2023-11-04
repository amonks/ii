package system

import (
	"fmt"
	"log"
	"os"
	"strings"

	"monks.co/movietagger/set"
)

type System struct {
	name   string
	logger *log.Logger
}

var (
	logfile *os.File
	systems = set.New[string]()
)

func New(name string) *System {
	return &System{name: name}
}

func Start() (err error) {
	logfile, err = os.OpenFile("movietagger.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("error opening logfile: %w", err)
	}
	return nil
}

func Stop() error {
	if count := systems.Len(); count != 0 {
		return fmt.Errorf("can't stop; %d systems still running: %s", count, strings.Join(systems.Values(), ", "))
	}
	if err := logfile.Close(); err != nil {
		return fmt.Errorf("error closing logfile: %w", err)
	}
	return nil
}

func (s *System) Start() *System {
	systems.Add(s.name)
	s.logger = log.New(logfile, s.name+": ", log.Ldate|log.Ltime)
	s.Println("start")
	return s
}

func (s *System) Stop() *System {
	systems.Remove(s.name)
	s.Println("done")
	return s
}

func (s *System) Println(v ...interface{}) {
	s.logger.Println(v...)
}

func (s *System) Printf(format string, v ...interface{}) {
	s.logger.Printf(format, v...)
}
