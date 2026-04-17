package service

import (
	"regexp"
	"time"
)

var (
	reDateYMD = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reTimeHM  = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

func isValidDate(s string) bool {
	if !reDateYMD.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func isValidTime(s string) bool {
	return reTimeHM.MatchString(s)
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
