package handler

import "testing"

func TestImageURLResolver_Match_PrependBase(t *testing.T) {
	r := NewImageURLResolver("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")
	got := r.Match("matches/abc-123.jpg")
	want := "http://localhost:4566/smatch-matches/matches/abc-123.jpg"
	if got != want {
		t.Errorf("Match() = %q, want %q", got, want)
	}
}

func TestImageURLResolver_Profile_PrependBase(t *testing.T) {
	r := NewImageURLResolver("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")
	got := r.Profile("profile/user-1/abc.jpg")
	want := "http://localhost:4566/smatch-profiles/profile/user-1/abc.jpg"
	if got != want {
		t.Errorf("Profile() = %q, want %q", got, want)
	}
}

func TestImageURLResolver_TolerantFullURL(t *testing.T) {
	r := NewImageURLResolver("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")
	got := r.Match("https://cdn.example.com/already-a-url.jpg")
	if got != "https://cdn.example.com/already-a-url.jpg" {
		t.Errorf("full URL should pass through, got %q", got)
	}
}

func TestImageURLResolver_EmptyKey(t *testing.T) {
	r := NewImageURLResolver("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")
	if r.Match("") != "" {
		t.Error("empty key should return empty string")
	}
	if r.Profile("") != "" {
		t.Error("empty key should return empty string")
	}
}

func TestImageURLResolver_TrailingSlashNormalization(t *testing.T) {
	r := NewImageURLResolver("http://localhost:4566/smatch-matches/", "http://localhost:4566/smatch-profiles/")
	got := r.Match("matches/abc.jpg")
	want := "http://localhost:4566/smatch-matches/matches/abc.jpg"
	if got != want {
		t.Errorf("with trailing slash, Match() = %q, want %q", got, want)
	}
}
