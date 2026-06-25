package handler

import (
	"testing"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/repository"
)

func newTestMatchHandler() *MatchHandler {
	return &MatchHandler{images: testResolver}
}

func TestMapMatchRowToDTO(t *testing.T) {
	firstName := "John"
	lastName := "Doe"
	username := "johndoe"
	title := "Weekend game"
	desc := "Casual play"

	mr := &repository.MatchRow{
		Match: domain.Match{
			ID:           "match-1",
			CourtID:      "court-1",
			HostUserID:   "user-1",
			Title:        &title,
			Description:  &desc,
			Images:       []string{"img1.jpg", "img2.jpg"},
			SkillLevel:   domain.SkillLevelTB,
			ShuttleType:  domain.ShuttleTC77,
			PlayerFormat: domain.PlayerFormatDoubleMale,
			Date:         time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
			StartTime:    "18:00",
			EndTime:      "20:00",
			IsPrivate:    false,
			Price:        50000,
			SlotsNeeded:  4,
			Status:       domain.MatchStatusOpen,
			CreatedAt:    time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
		},
		CourtName:       "Test Court",
		CourtAddressStr: "123 Main St, Ba Dinh, Ha Noi",
		HostFirstName:   &firstName,
		HostLastName:    &lastName,
		HostUsername:    &username,
		SlotsAccepted:   2,
	}

	h := newTestMatchHandler()
	resp := h.mapMatchRowToDTO(mr)

	if resp.ID != "match-1" {
		t.Errorf("ID = %q, want match-1", resp.ID)
	}
	if resp.CourtName != "Test Court" {
		t.Errorf("CourtName = %q, want Test Court", resp.CourtName)
	}
	if resp.HostName != "John Doe" {
		t.Errorf("HostName = %q, want 'John Doe'", resp.HostName)
	}
	if resp.Date != "2026-04-15" {
		t.Errorf("Date = %q, want 2026-04-15", resp.Date)
	}
	if resp.SkillLevel != "TB" {
		t.Errorf("SkillLevel = %q, want TB", resp.SkillLevel)
	}
	if resp.SlotsAccepted != 2 {
		t.Errorf("SlotsAccepted = %d, want 2", resp.SlotsAccepted)
	}
	if resp.Status != "OPEN" {
		t.Errorf("Status = %q, want OPEN", resp.Status)
	}
	if len(resp.Images) != 2 {
		t.Errorf("Images length = %d, want 2", len(resp.Images))
	}
	if resp.Images[0] != "http://localhost:4566/smatch-matches/img1.jpg" {
		t.Errorf("Images[0] = %q, want resolved URL", resp.Images[0])
	}
}

func TestMapMatchRowToDTO_NilImages(t *testing.T) {
	mr := &repository.MatchRow{
		Match: domain.Match{
			ID:        "match-1",
			Images:    nil,
			Date:      time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	h := newTestMatchHandler()
	resp := h.mapMatchRowToDTO(mr)

	// Nil images should become empty array
	if resp.Images == nil {
		t.Error("Images should be empty array, not nil")
	}
	if len(resp.Images) != 0 {
		t.Errorf("Images length = %d, want 0", len(resp.Images))
	}
}

func TestMapPlayerRowToDTO(t *testing.T) {
	firstName := "Jane"
	lastName := "Smith"
	username := "janesmith"
	photoURL := "https://example.com/photo.jpg"
	message := "I want to play!"
	position := 3
	respondedAt := time.Date(2026, 4, 14, 11, 0, 0, 0, time.UTC)

	p := &repository.MatchPlayerRow{
		MatchPlayer: domain.MatchPlayer{
			ID:          "mp-1",
			MatchID:     "match-1",
			UserID:      "user-2",
			Status:      domain.MatchPlayerStatusAccepted,
			Message:     &message,
			Position:    &position,
			RequestedAt: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
			RespondedAt: &respondedAt,
		},
		UserFirstName: &firstName,
		UserLastName:  &lastName,
		UserUsername:  &username,
		UserPhotoURL:  &photoURL,
	}

	h := newTestMatchHandler()
	resp := h.mapPlayerRowToDTO(p)

	if resp.ID != "mp-1" {
		t.Errorf("ID = %q, want mp-1", resp.ID)
	}
	if resp.UserName != "Jane Smith" {
		t.Errorf("UserName = %q, want 'Jane Smith'", resp.UserName)
	}
	if resp.Status != "ACCEPTED" {
		t.Errorf("Status = %q, want ACCEPTED", resp.Status)
	}
	if resp.Position == nil || *resp.Position != 3 {
		t.Errorf("Position = %v, want 3", resp.Position)
	}
	if resp.RespondedAt == nil {
		t.Error("RespondedAt should not be nil")
	}
	if resp.UserPhotoURL == nil || *resp.UserPhotoURL != photoURL {
		t.Errorf("UserPhotoURL = %v, want %q", resp.UserPhotoURL, photoURL)
	}
}

func TestMapPlayerRowToDTO_NilRespondedAt(t *testing.T) {
	p := &repository.MatchPlayerRow{
		MatchPlayer: domain.MatchPlayer{
			ID:          "mp-1",
			Status:      domain.MatchPlayerStatusPending,
			RequestedAt: time.Now(),
		},
	}

	h := newTestMatchHandler()
	resp := h.mapPlayerRowToDTO(p)

	if resp.RespondedAt != nil {
		t.Errorf("RespondedAt should be nil for pending player, got %v", resp.RespondedAt)
	}
}

func TestMapPlayersToDTO(t *testing.T) {
	players := []*repository.MatchPlayerRow{
		{
			MatchPlayer: domain.MatchPlayer{
				ID: "mp-1", Status: domain.MatchPlayerStatusAccepted, RequestedAt: time.Now(),
			},
		},
		{
			MatchPlayer: domain.MatchPlayer{
				ID: "mp-2", Status: domain.MatchPlayerStatusPending, RequestedAt: time.Now(),
			},
		},
	}

	h := newTestMatchHandler()
	resp := h.mapPlayersToDTO(players)

	if len(resp) != 2 {
		t.Errorf("length = %d, want 2", len(resp))
	}
	if resp[0].ID != "mp-1" || resp[1].ID != "mp-2" {
		t.Error("players not mapped in order")
	}
}

func TestMapCurrentUserStatus(t *testing.T) {
	position := 2
	respondedAt := time.Date(2026, 4, 14, 11, 0, 0, 0, time.UTC)

	p := &repository.MatchPlayerRow{
		MatchPlayer: domain.MatchPlayer{
			ID:          "mp-1",
			Status:      domain.MatchPlayerStatusAccepted,
			Position:    &position,
			RequestedAt: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
			RespondedAt: &respondedAt,
		},
	}

	resp := mapCurrentUserStatus(p)

	if resp.ID != "mp-1" {
		t.Errorf("ID = %q, want mp-1", resp.ID)
	}
	if resp.Status != "ACCEPTED" {
		t.Errorf("Status = %q, want ACCEPTED", resp.Status)
	}
	if resp.Position == nil || *resp.Position != 2 {
		t.Errorf("Position = %v, want 2", resp.Position)
	}
	if resp.RespondedAt == nil {
		t.Error("RespondedAt should not be nil")
	}
}

func TestBuildDisplayName(t *testing.T) {
	first := "John"
	last := "Doe"
	user := "johndoe"

	tests := []struct {
		name      string
		firstName *string
		lastName  *string
		username  *string
		want      string
	}{
		{"both names", &first, &last, &user, "John Doe"},
		{"first only", &first, nil, &user, "John"},
		{"username only", nil, nil, &user, "johndoe"},
		{"all nil", nil, nil, nil, "Anonymous"},
		{"first and last, no username", &first, &last, nil, "John Doe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDisplayName(tt.firstName, tt.lastName, tt.username)
			if got != tt.want {
				t.Errorf("buildDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
