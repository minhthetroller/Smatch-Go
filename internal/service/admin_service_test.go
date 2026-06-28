package service

import (
	"testing"
	"time"
)

func TestFormatTimePtr(t *testing.T) {
	now := time.Now()
	result := formatTimePtr(&now)
	if result == nil {
		t.Fatal("expected non-nil string")
	}
	if *result == "" {
		t.Error("expected non-empty formatted time")
	}

	result = formatTimePtr(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestContains(t *testing.T) {
	if !contains([]string{"user", "court_owner"}, "court_owner") {
		t.Error("expected true for existing item")
	}
	if contains([]string{"user"}, "admin") {
		t.Error("expected false for missing item")
	}
}

func TestParsePeriod(t *testing.T) {
	from, to := parsePeriod("week")
	if !to.After(from) {
		t.Error("expected to > from for week period")
	}

	from, to = parsePeriod("month")
	if !to.After(from) {
		t.Error("expected to > from for month period")
	}

	from, to = parsePeriod("")
	if !to.After(from) {
		t.Error("expected to > from for default period")
	}
}
