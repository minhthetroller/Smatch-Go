package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/repository"
)

type fakeAvailabilityRepository struct {
	subCourt *repository.RawSubCourt
}

func (f fakeAvailabilityRepository) GetSubCourtsByCourtID(context.Context, string) ([]*repository.RawSubCourt, error) {
	return nil, nil
}

func (f fakeAvailabilityRepository) GetBookingsByCourtAndDate(context.Context, string, string) ([]*repository.RawBooking, error) {
	return nil, nil
}

func (f fakeAvailabilityRepository) GetPricingRulesByCourtID(context.Context, string) ([]*repository.RawPricingRule, error) {
	return nil, nil
}

func (f fakeAvailabilityRepository) GetClosuresByCourtAndDate(context.Context, string, string) ([]*repository.RawClosure, error) {
	return nil, nil
}

func (f fakeAvailabilityRepository) IsHoliday(context.Context, string) (bool, error) {
	return false, nil
}

func (f fakeAvailabilityRepository) GetHolidayMultiplier(context.Context, string) (float64, error) {
	return 1.0, nil
}

func (f fakeAvailabilityRepository) GetSubCourtWithCourt(context.Context, string) (*repository.RawSubCourt, error) {
	return f.subCourt, nil
}

func (f fakeAvailabilityRepository) HasOverlappingBooking(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}

func (f fakeAvailabilityRepository) CreateBookings(context.Context, []repository.CreateBookingItem, repository.CreateBookingCommon) ([]string, error) {
	return nil, nil
}

func (f fakeAvailabilityRepository) GetBookingByID(context.Context, string) (*repository.BookingRow, error) {
	return nil, nil
}

func (f fakeAvailabilityRepository) UpdateBookingStatus(context.Context, string, string) error {
	return nil
}

func (f fakeAvailabilityRepository) GetUserBookings(context.Context, string, int, int) ([]*repository.BookingRow, int, error) {
	return nil, 0, nil
}

func TestGenerateTimeSlots_Empty(t *testing.T) {
	slots := generateTimeSlots("06:00", "08:00", nil, nil, nil, "weekday", 1.0)

	// 06:00-08:00 = 4 x 30-min slots
	if len(slots) != 4 {
		t.Fatalf("expected 4 slots, got %d", len(slots))
	}

	expected := []struct{ start, end string }{
		{"06:00", "06:30"},
		{"06:30", "07:00"},
		{"07:00", "07:30"},
		{"07:30", "08:00"},
	}

	for i, e := range expected {
		if slots[i].StartTime != e.start || slots[i].EndTime != e.end {
			t.Errorf("slot %d: got %s-%s, want %s-%s", i, slots[i].StartTime, slots[i].EndTime, e.start, e.end)
		}
		if !slots[i].IsAvailable {
			t.Errorf("slot %d: expected available, got unavailable", i)
		}
		// No pricing rules → price should be 0
		if slots[i].Price != 0 {
			t.Errorf("slot %d: price = %d, want 0", i, slots[i].Price)
		}
	}
}

func TestCreateBooking_InvalidSingleSubCourtIDReturnsBadRequest(t *testing.T) {
	svc := &AvailabilityService{}
	req := &dto.CreateBookingRequest{
		SubCourtID: "subcourt-1",
		Date:       "2026-05-20",
		StartTime:  "09:00",
		EndTime:    "10:00",
		GuestName:  "Guest",
		GuestPhone: "0900000000",
	}

	_, err := svc.CreateBooking(context.Background(), req, "user-1")

	appErr, ok := err.(*domain.AppError)
	if !ok {
		t.Fatalf("err = %T, want *domain.AppError", err)
	}
	if appErr.Code != "BAD_REQUEST" || appErr.Status != 400 {
		t.Fatalf("appErr = %+v, want BAD_REQUEST 400", appErr)
	}
	if appErr.Message != "Invalid subCourtId: must be a UUID" {
		t.Fatalf("message = %q, want invalid subCourtId message", appErr.Message)
	}
}

func TestCreateBooking_InvalidMultiSubCourtIDReturnsBadRequest(t *testing.T) {
	svc := &AvailabilityService{}
	req := &dto.CreateBookingRequest{
		Bookings: []dto.SingleBookingItem{{
			SubCourtID: "subcourt-1",
			Date:       "2026-05-20",
			StartTime:  "09:00",
			EndTime:    "10:00",
		}},
		GuestName:  "Guest",
		GuestPhone: "0900000000",
	}

	_, err := svc.CreateBooking(context.Background(), req, "")

	appErr, ok := err.(*domain.AppError)
	if !ok {
		t.Fatalf("err = %T, want *domain.AppError", err)
	}
	if appErr.Code != "BAD_REQUEST" || appErr.Status != 400 {
		t.Fatalf("appErr = %+v, want BAD_REQUEST 400", appErr)
	}
}

func TestCreateBooking_ValidMissingSubCourtReturnsNotFound(t *testing.T) {
	svc := &AvailabilityService{availRepo: fakeAvailabilityRepository{}}
	req := &dto.CreateBookingRequest{
		SubCourtID: "11111111-1111-1111-1111-111111111111",
		Date:       "2026-05-20",
		StartTime:  "09:00",
		EndTime:    "10:00",
		GuestName:  "Guest",
		GuestPhone: "0900000000",
	}

	_, err := svc.CreateBooking(context.Background(), req, "")

	appErr, ok := err.(*domain.AppError)
	if !ok {
		t.Fatalf("err = %T, want *domain.AppError", err)
	}
	if appErr.Code != "NOT_FOUND" || appErr.Status != 404 {
		t.Fatalf("appErr = %+v, want NOT_FOUND 404", appErr)
	}
}

func TestGenerateTimeSlots_WithBooking(t *testing.T) {
	bookings := []*repository.RawBooking{
		{ID: "b1", SubCourtID: "sc1", StartTime: "07:00", EndTime: "08:00", Status: "confirmed"},
	}

	slots := generateTimeSlots("06:00", "09:00", bookings, nil, nil, "weekday", 1.0)

	// 6 slots total: 06:00, 06:30, 07:00, 07:30, 08:00, 08:30
	if len(slots) != 6 {
		t.Fatalf("expected 6 slots, got %d", len(slots))
	}

	// 06:00 and 06:30 should be available
	if !slots[0].IsAvailable || !slots[1].IsAvailable {
		t.Error("06:00-07:00 should be available")
	}
	// 07:00 and 07:30 should be booked
	if slots[2].IsAvailable || slots[3].IsAvailable {
		t.Error("07:00-08:00 should be unavailable (booked)")
	}
	// 08:00 and 08:30 should be available
	if !slots[4].IsAvailable || !slots[5].IsAvailable {
		t.Error("08:00-09:00 should be available")
	}
}

func TestGenerateTimeSlots_WithClosure(t *testing.T) {
	startT := "10:00"
	endT := "11:00"
	closures := []*repository.RawClosure{
		{ID: "c1", SubCourtID: "sc1", StartTime: &startT, EndTime: &endT},
	}

	slots := generateTimeSlots("09:00", "12:00", nil, closures, nil, "weekday", 1.0)

	// 09:00, 09:30 → available
	if !slots[0].IsAvailable || !slots[1].IsAvailable {
		t.Error("09:00-10:00 should be available")
	}
	// 10:00, 10:30 → closed
	if slots[2].IsAvailable || slots[3].IsAvailable {
		t.Error("10:00-11:00 should be unavailable (closed)")
	}
	// 11:00, 11:30 → available
	if !slots[4].IsAvailable || !slots[5].IsAvailable {
		t.Error("11:00-12:00 should be available")
	}
}

func TestGenerateTimeSlots_FullDayClosure(t *testing.T) {
	closures := []*repository.RawClosure{
		{ID: "c1", SubCourtID: "sc1", StartTime: nil, EndTime: nil},
	}

	slots := generateTimeSlots("06:00", "08:00", nil, closures, nil, "weekday", 1.0)

	for i, s := range slots {
		if s.IsAvailable {
			t.Errorf("slot %d (%s-%s): expected unavailable for full-day closure", i, s.StartTime, s.EndTime)
		}
	}
}

func TestGenerateTimeSlots_WithPricingRules(t *testing.T) {
	rules := []*repository.RawPricingRule{
		{ID: "r1", DayType: "weekday", StartTime: "06:00", EndTime: "17:00", PricePerHour: 200000, IsActive: true},
		{ID: "r2", DayType: "weekday", StartTime: "17:00", EndTime: "22:00", PricePerHour: 300000, IsActive: true},
	}

	slots := generateTimeSlots("16:00", "18:00", nil, nil, rules, "weekday", 1.0)

	if len(slots) != 4 {
		t.Fatalf("expected 4 slots, got %d", len(slots))
	}

	// 16:00 and 16:30 should use first rule (200000/2 = 100000 per 30-min)
	if slots[0].Price != 100000 {
		t.Errorf("slot 16:00 price = %d, want 100000", slots[0].Price)
	}
	if slots[1].Price != 100000 {
		t.Errorf("slot 16:30 price = %d, want 100000", slots[1].Price)
	}
	// 17:00 and 17:30 should use second rule (300000/2 = 150000 per 30-min)
	if slots[2].Price != 150000 {
		t.Errorf("slot 17:00 price = %d, want 150000", slots[2].Price)
	}
	if slots[3].Price != 150000 {
		t.Errorf("slot 17:30 price = %d, want 150000", slots[3].Price)
	}
}

func TestGenerateTimeSlots_HolidayMultiplier(t *testing.T) {
	rules := []*repository.RawPricingRule{
		{ID: "r1", DayType: "holiday", StartTime: "06:00", EndTime: "22:00", PricePerHour: 200000, IsActive: true},
	}

	slots := generateTimeSlots("06:00", "07:00", nil, nil, rules, "holiday", 1.5)

	// 200000/2 * 1.5 = 150000 per 30-min slot
	if slots[0].Price != 150000 {
		t.Errorf("slot 06:00 price = %d, want 150000", slots[0].Price)
	}
}

func TestCalculateTotalPrice(t *testing.T) {
	rules := []*repository.RawPricingRule{
		{ID: "r1", DayType: "weekday", StartTime: "06:00", EndTime: "17:00", PricePerHour: 200000, IsActive: true},
		{ID: "r2", DayType: "weekday", StartTime: "17:00", EndTime: "22:00", PricePerHour: 300000, IsActive: true},
	}

	tests := []struct {
		name  string
		start string
		end   string
		want  int
	}{
		{
			name:  "1 hour within first rule",
			start: "06:00",
			end:   "07:00",
			want:  200000, // 100000 * 2 slots
		},
		{
			name:  "crossing boundary",
			start: "16:00",
			end:   "18:00",
			want:  500000, // 100000 + 100000 + 150000 + 150000
		},
		{
			name:  "1 hour within second rule",
			start: "18:00",
			end:   "19:00",
			want:  300000, // 150000 * 2 slots
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateTotalPrice(tt.start, tt.end, rules, "weekday", 1.0)
			if got != tt.want {
				t.Errorf("calculateTotalPrice(%q, %q) = %d, want %d", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

func TestGetPriceForSlot_InactiveRule(t *testing.T) {
	rules := []*repository.RawPricingRule{
		{ID: "r1", DayType: "weekday", StartTime: "06:00", EndTime: "22:00", PricePerHour: 200000, IsActive: false},
	}

	price := getPriceForSlot("10:00", rules, "weekday", 1.0)
	if price != 0 {
		t.Errorf("expected 0 for inactive rule, got %d", price)
	}
}

func TestGetPriceForSlot_WrongDayType(t *testing.T) {
	rules := []*repository.RawPricingRule{
		{ID: "r1", DayType: "weekend", StartTime: "06:00", EndTime: "22:00", PricePerHour: 200000, IsActive: true},
	}

	price := getPriceForSlot("10:00", rules, "weekday", 1.0)
	if price != 0 {
		t.Errorf("expected 0 for wrong day type, got %d", price)
	}
}

func TestGroupBookingsBySubCourt(t *testing.T) {
	bookings := []*repository.RawBooking{
		{ID: "b1", SubCourtID: "sc1"},
		{ID: "b2", SubCourtID: "sc1"},
		{ID: "b3", SubCourtID: "sc2"},
	}

	grouped := groupBookingsBySubCourt(bookings)

	if len(grouped["sc1"]) != 2 {
		t.Errorf("sc1: expected 2 bookings, got %d", len(grouped["sc1"]))
	}
	if len(grouped["sc2"]) != 1 {
		t.Errorf("sc2: expected 1 booking, got %d", len(grouped["sc2"]))
	}
}

func TestGroupClosuresBySubCourt(t *testing.T) {
	closures := []*repository.RawClosure{
		{ID: "c1", SubCourtID: "sc1"},
		{ID: "c2", SubCourtID: "sc2"},
		{ID: "c3", SubCourtID: "sc2"},
	}

	grouped := groupClosuresBySubCourt(closures)

	if len(grouped["sc1"]) != 1 {
		t.Errorf("sc1: expected 1 closure, got %d", len(grouped["sc1"]))
	}
	if len(grouped["sc2"]) != 2 {
		t.Errorf("sc2: expected 2 closures, got %d", len(grouped["sc2"]))
	}
}

func TestMapBookingRow(t *testing.T) {
	notes := "test notes"
	b := &repository.BookingRow{
		ID:           "booking-1",
		SubCourtID:   "sc-1",
		SubCourtName: "Court A",
		CourtID:      "court-1",
		CourtName:    "Test Complex",
		GuestName:    "John",
		GuestPhone:   "0123456789",
		Date:         "2026-04-14",
		StartTime:    "10:00",
		EndTime:      "11:00",
		TotalPrice:   200000,
		Status:       "confirmed",
		Notes:        &notes,
		CreatedAt:    "2026-04-14T10:00:00.000Z",
	}

	resp := mapBookingRow(b)

	if resp.ID != b.ID || resp.SubCourtID != b.SubCourtID || resp.CourtName != b.CourtName {
		t.Error("mapBookingRow did not correctly copy fields")
	}
	if resp.TotalPrice != 200000 {
		t.Errorf("total_price = %d, want 200000", resp.TotalPrice)
	}
	if resp.Notes == nil || *resp.Notes != notes {
		t.Errorf("notes = %v, want %q", resp.Notes, notes)
	}
}

// Ensure dto.TimeSlot fields match what we expect.
func TestTimeSlotStruct(t *testing.T) {
	slot := dto.TimeSlot{
		StartTime:   "06:00",
		EndTime:     "06:30",
		IsAvailable: true,
		Price:       100000,
	}
	if slot.StartTime != "06:00" || slot.EndTime != "06:30" || !slot.IsAvailable || slot.Price != 100000 {
		t.Error("TimeSlot fields not set correctly")
	}
}

// TestOpeningHours_ParseFromDBShape is a regression test for the
// "court is closed on weekdays" bug. The DB stores opening_hours as
// per-day lowercase keys: {"mon":"06:00-22:00",...,"sun":"06:00-23:00"}.
// The parser must unmarshal this correctly and ohForDay must return
// non-nil hours for every day that has a value.
func TestOpeningHours_ParseFromDBShape(t *testing.T) {
	raw := []byte(`{"mon":"06:00-22:00","tue":"06:00-22:00","wed":"06:00-22:00","thu":"06:00-22:00","fri":"06:00-22:00","sat":"06:00-23:00","sun":"06:00-23:00"}`)

	var oh OpeningHours
	if err := json.Unmarshal(raw, &oh); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	days := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	for _, day := range days {
		got := ohForDay(&oh, day)
		if got == nil {
			t.Errorf("ohForDay(oh, %q) = nil, expected non-nil hours (court should be open)", day)
		}
	}
}

// TestOpeningHours_EmptyDayReturnsNil ensures a day with no hours
// (empty string or missing key) correctly returns nil (closed).
func TestOpeningHours_EmptyDayReturnsNil(t *testing.T) {
	raw := []byte(`{"mon":"06:00-22:00","tue":"06:00-22:00"}`)
	var oh OpeningHours
	if err := json.Unmarshal(raw, &oh); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if got := ohForDay(&oh, "wed"); got != nil {
		t.Errorf("ohForDay(oh, \"wed\") = %q, want nil (no hours for wed)", *got)
	}
}
