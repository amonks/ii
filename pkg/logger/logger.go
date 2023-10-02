package logger

import "log"

type Logger interface {
  Logf(msg string, args ...interface{})
}

type namedLogger struct {
  name string
}

func (l *namedLogger) Logf(msg string, args ...interface{}) {
  log.Printf(l.name+": "+msg+"\n", args...)
}

func New(name string) Logger {
  return &namedLogger{
    name: name,
  }
}

