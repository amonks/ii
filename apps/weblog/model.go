package weblog

import (
	"crawshaw.io/sqlite"
	"monks.co/pkg/logger"
)

type model struct {
	logger.Logger
}

func NewModel() *model {
	return &model{
		Logger: logger.New("weblog model"),
	}
}

func (m *model) migrate(conn *sqlite.Conn) error {
	return nil
}
