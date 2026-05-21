package service

import (
	"testing"
)

func TestIsValidDate(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"2026-01-15", true},
		{"2026-12-31", true},
		{"2026-02-28", true},
		{"2026-02-29", false}, // 2026 is not a leap year
		{"2024-02-29", true},  // 2024 is a leap year
		{"2026-13-01", false}, // invalid month
		{"2026-00-01", false}, // invalid month
		{"2026-01-32", false}, // invalid day
		{"26-01-15", false},   // wrong format
		{"2026/01/15", false}, // wrong separator
		{"", false},
		{"not-a-date", false},
		{"2026-1-5", false}, // missing leading zeros
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isValidDate(tt.input)
			if got != tt.want {
				t.Errorf("isValidDate(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidTime(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"00:00", true},
		{"23:59", true},
		{"12:30", true},
		{"06:00", true},
		{"24:00", false},
		{"12:60", false},
		{"1:30", false}, // missing leading zero
		{"12:5", false}, // missing leading zero
		{"", false},
		{"noon", false},
		{"12:30:00", false}, // has seconds
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isValidTime(tt.input)
			if got != tt.want {
				t.Errorf("isValidTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	t.Run("valid date", func(t *testing.T) {
		d, err := parseDate("2026-04-14")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Year() != 2026 || d.Month() != 4 || d.Day() != 14 {
			t.Errorf("parseDate(\"2026-04-14\") = %v, want 2026-04-14", d)
		}
	})

	t.Run("invalid date", func(t *testing.T) {
		_, err := parseDate("not-a-date")
		if err == nil {
			t.Error("expected error for invalid date, got nil")
		}
	})
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input string
		wantH int
		wantM int
	}{
		{"06:30", 6, 30},
		{"00:00", 0, 0},
		{"23:59", 23, 59},
		{"12:00", 12, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			h, m := parseTime(tt.input)
			if h != tt.wantH || m != tt.wantM {
				t.Errorf("parseTime(%q) = (%d, %d), want (%d, %d)", tt.input, h, m, tt.wantH, tt.wantM)
			}
		})
	}
}

func TestAddMinutes(t *testing.T) {
	tests := []struct {
		input string
		mins  int
		want  string
	}{
		{"06:00", 30, "06:30"},
		{"06:30", 30, "07:00"},
		{"23:30", 30, "24:00"},
		{"00:00", 60, "01:00"},
		{"10:00", 90, "11:30"},
		{"10:45", 15, "11:00"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := addMinutes(tt.input, tt.mins)
			if got != tt.want {
				t.Errorf("addMinutes(%q, %d) = %q, want %q", tt.input, tt.mins, got, tt.want)
			}
		})
	}
}

func TestMinutesBetween(t *testing.T) {
	tests := []struct {
		start string
		end   string
		want  int
	}{
		{"06:00", "07:00", 60},
		{"06:00", "06:30", 30},
		{"00:00", "23:59", 1439},
		{"10:30", "12:00", 90},
	}

	for _, tt := range tests {
		t.Run(tt.start+"-"+tt.end, func(t *testing.T) {
			got := minutesBetween(tt.start, tt.end)
			if got != tt.want {
				t.Errorf("minutesBetween(%q, %q) = %d, want %d", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

func TestSplitHours(t *testing.T) {
	tests := []struct {
		input     string
		wantOpen  string
		wantClose string
	}{
		{"06:00-22:00", "06:00", "22:00"},
		{"08:00-21:00", "08:00", "21:00"},
		{"00:00-23:59", "00:00", "23:59"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			open, close := splitHours(tt.input)
			if open != tt.wantOpen || close != tt.wantClose {
				t.Errorf("splitHours(%q) = (%q, %q), want (%q, %q)", tt.input, open, close, tt.wantOpen, tt.wantClose)
			}
		})
	}

	t.Run("no dash", func(t *testing.T) {
		open, close := splitHours("invalid")
		if open != "00:00" || close != "23:59" {
			t.Errorf("splitHours(\"invalid\") = (%q, %q), want (\"00:00\", \"23:59\")", open, close)
		}
	})
}

func TestOhForDay(t *testing.T) {
	weekdayStr := "06:00-22:00"
	weekendStr := "08:00-20:00"
	oh := &OpeningHours{
		Weekdays: &DayRange{Open: "06:00", Close: "22:00"},
		Weekends: &DayRange{Open: "08:00", Close: "20:00"},
	}

	tests := []struct {
		day  string
		want *string
	}{
		{"mon", &weekdayStr},
		{"tue", &weekdayStr},
		{"wed", &weekdayStr},
		{"thu", &weekdayStr},
		{"fri", &weekdayStr},
		{"sat", &weekendStr},
		{"sun", &weekendStr},
		{"invalid", nil},
	}

	for _, tt := range tests {
		t.Run(tt.day, func(t *testing.T) {
			got := ohForDay(oh, tt.day)
			if tt.want == nil {
				if got != nil {
					t.Errorf("ohForDay(oh, %q) = %v, want nil", tt.day, *got)
				}
			} else {
				if got == nil {
					t.Errorf("ohForDay(oh, %q) = nil, want %q", tt.day, *tt.want)
				} else if *got != *tt.want {
					t.Errorf("ohForDay(oh, %q) = %q, want %q", tt.day, *got, *tt.want)
				}
			}
		})
	}
}

func TestTimesOverlap(t *testing.T) {
	tests := []struct {
		name string
		s1   string
		e1   string
		s2   string
		e2   string
		want bool
	}{
		{"full overlap", "06:00", "08:00", "06:00", "08:00", true},
		{"partial overlap start", "06:00", "08:00", "07:00", "09:00", true},
		{"partial overlap end", "07:00", "09:00", "06:00", "08:00", true},
		{"contained", "07:00", "08:00", "06:00", "09:00", true},
		{"contains", "06:00", "09:00", "07:00", "08:00", true},
		{"adjacent no overlap", "06:00", "07:00", "07:00", "08:00", false},
		{"no overlap gap", "06:00", "07:00", "08:00", "09:00", false},
		{"reverse adjacent", "07:00", "08:00", "06:00", "07:00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := timesOverlap(tt.s1, tt.e1, tt.s2, tt.e2)
			if got != tt.want {
				t.Errorf("timesOverlap(%q, %q, %q, %q) = %v, want %v", tt.s1, tt.e1, tt.s2, tt.e2, got, tt.want)
			}
		})
	}
}
