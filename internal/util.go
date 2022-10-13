package internal

import (
	"time"
)

var format = "2006-01-02T15-04-05Z"

func FormatTime(t time.Time) string {
	return t.Format(format)
}

func ParseTime(s string) (time.Time, error) {
	return time.Parse(format, s)
}
