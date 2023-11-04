package db

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
)

func NewStringList(s []string) *StringList {
	l := StringList(s)
	return &l
}

type StringList []string

// Scan scan value into Jsonb, implements sql.Scanner interface
func (l *StringList) Scan(value interface{}) error {
	list, ok := value.(string)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal StringList value:", value))
	}

	*l = split(list)
	return nil
}

// Value return json value, implement driver.Valuer interface
func (l StringList) Value() (driver.Value, error) {
	if len(l) == 0 {
		return nil, nil
	}
	return string(join(l)), nil
}

func join(ss StringList) string {
	return ":" + strings.Join(ss, ":") + ":"
}

func split(s string) StringList {
	return strings.Split(s[1:len(s)-1], ":")
}
