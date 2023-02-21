package db

import "strings"

func join(ss []string) string {
	return ":" + strings.Join(ss, ":") + ":"
}

func split(s string) []string {
	return strings.Split(s[1:len(s)-1], ":")
}

