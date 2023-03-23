package system

import (
	"log"
	"os"
)

type System struct {
	name   string
	logger *log.Logger
}

func New(name string) *System {
	return &System{name: name}
}

func (s *System) Start() func() {
	logfile, err := os.OpenFile(s.name+".log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		os.Exit(1)
	}
	
	s.logger = log.New(logfile, s.name+": ", log.Ldate|log.Ltime)

	return func() {
		logfile.Close()
	}
}

func (s *System) Println(v ...interface{}) {
	s.logger.Println(v...)
}

func (s *System) Printf(format string, v ...interface{}) {
	s.logger.Printf(format, v...)
}

