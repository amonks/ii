package weblog

import (
	"co.monks.monks.co/logger"
	"crawshaw.io/sqlite"
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
