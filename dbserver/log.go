package dbserver

import (
	"fmt"
)

func (s *DBServer) Logf(msg string, args ...interface{}) {
	fmt.Printf(s.name+": "+msg+"\n", args...)
}
