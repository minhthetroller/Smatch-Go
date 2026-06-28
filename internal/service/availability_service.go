package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/repository"
)

// OpeningHours matches the JSON stored in the DB:
// {"mon":"06:00-22:00","tue":"06:00-22:00",...,"sun":"06:00-23:00"}
// Keys are lowercase 3-letter day abbreviations; values are "HH:MM-HH:MM" range strings.
type OpeningHours map[string]string

var dayNames = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

type AvailabilityService struct {
	availRepo availabilityRepository
	courtRepo courtRepository
}

type availabilityRepository interface {
	GetSubCourtsByCourtID(ctx context.Context, courtID string) ([]*repository.RawSubCourt, error)
	GetBookingsByCourtAndDate(ctx context.Context, courtID, date string) ([]*repository.RawBooking, error)
	GetPricingRulesByCourtID(ctx context.Context, courtID string) ([]*repository.RawPricingRule, error)
	GetClosuresByCourtAndDate(ctx context.Context, courtID, date string) ([]*repository.RawClosure, error)
	IsHoliday(ctx context.Context, date string) (bool, error)
	GetHolidayMultiplier(ctx context.Context, date string) (float64, error)
	GetSubCourtWithCourt(ctx context.Context, subCourtID string) (*repository.RawSubCourt, error)
	HasOverlappingBooking(ctx context.Context, subCourtID, date, startTime, endTime string) (bool, error)
	CreateBookings(ctx context.Context, items []repository.CreateBookingItem, common repository.CreateBookingCommon) ([]string, error)
	GetBookingByID(ctx context.Context, id string) (*repository.BookingRow, error)
	UpdateBookingStatus(ctx context.Context, id, status string) error
	GetUserBookings(ctx context.Context, userID string, page, limit int) ([]*repository.BookingRow, int, error)
}

type courtRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Court, error)
}

func NewAvailabilityService(ar *repository.AvailabilityRepository, cr *repository.CourtRepository) *AvailabilityService {
	return &AvailabilityService{availRepo: ar, courtRepo: cr}
}

// GetCourtAvailability returns time slot availability for a court on a date.
func (s *AvailabilityService) GetCourtAvailability(ctx context.Context, courtID, date string) (*dto.CourtAvailabilityResponse, error) {
	if !isValidDate(date) {
		return nil, domain.BadRequest("Invalid date format. Use YYYY-MM-DD")
	}

	court, err := s.courtRepo.FindByID(ctx, courtID)
	if err != nil {
		return nil, err
	}
	if court == nil {
		return nil, domain.NotFound("Court not found")
	}

	// Parse opening hours
	var oh OpeningHours
	if err := json.Unmarshal(court.OpeningHours, &oh); err != nil {
		return nil, fmt.Errorf("availability: parse opening hours: %w", err)
	}

	dateObj, _ := parseDate(date)
	dayKey := dayNames[dateObj.Weekday()]
	dayHours := ohForDay(&oh, dayKey)
	if dayHours == nil {
		return nil, domain.BadRequest(fmt.Sprintf("Court is closed on %s", dayKey))
	}

	openTime, closeTime := splitHours(*dayHours)

	// Day type
	isHoliday, _ := s.availRepo.IsHoliday(ctx, date)
	isWeekend := dateObj.Weekday() == 0 || dateObj.Weekday() == 6
	dayType := "weekday"
	if isHoliday {
		dayType = "holiday"
	} else if isWeekend {
		dayType = "weekend"
	}

	// Fetch data in parallel (sequential for simplicity, parallel in prod)
	subCourts, _ := s.availRepo.GetSubCourtsByCourtID(ctx, courtID)
	bookings, _ := s.availRepo.GetBookingsByCourtAndDate(ctx, courtID, date)
	pricingRules, _ := s.availRepo.GetPricingRulesByCourtID(ctx, courtID)
	closures, _ := s.availRepo.GetClosuresByCourtAndDate(ctx, courtID, date)
	holidayMultiplier, _ := s.availRepo.GetHolidayMultiplier(ctx, date)

	// Group by sub-court
	bookingsBySubCourt := groupBookingsBySubCourt(bookings)
	closuresBySubCourt := groupClosuresBySubCourt(closures)

	resp := &dto.CourtAvailabilityResponse{
		CourtID:     court.ID,
		CourtName:   court.Name,
		Date:        date,
		DayType:     dayType,
		OpeningTime: openTime,
		ClosingTime: closeTime,
	}

	for _, sc := range subCourts {
		scBookings := bookingsBySubCourt[sc.ID]
		scClosures := closuresBySubCourt[sc.ID]
		slots := generateTimeSlots(openTime, closeTime, scBookings, scClosures, pricingRules, dayType, holidayMultiplier)
		resp.SubCourts = append(resp.SubCourts, dto.SubCourtAvailabilityResponse{
			ID:          sc.ID,
			Name:        sc.Name,
			Description: sc.Description,
			IsActive:    sc.IsActive,
			Slots:       slots,
		})
	}

	return resp, nil
}

// CreateBooking creates one or more bookings for a user.
func (s *AvailabilityService) CreateBooking(ctx context.Context, req *dto.CreateBookingRequest, userID string) ([]*dto.BookingResponse, error) {
	items := req.Bookings
	if len(items) == 0 && req.SubCourtID != "" {
		items = []dto.SingleBookingItem{{
			SubCourtID: req.SubCourtID,
			Date:       req.Date,
			StartTime:  req.StartTime,
			EndTime:    req.EndTime,
		}}
	}
	if len(items) == 0 {
		return nil, domain.BadRequest("Invalid booking data")
	}

	groupID := uuid.New().String()
	var repoItems []repository.CreateBookingItem

	for _, item := range items {
		if _, err := uuid.Parse(item.SubCourtID); err != nil {
			return nil, domain.BadRequest("Invalid subCourtId: must be a UUID")
		}
		if !isValidDate(item.Date) {
			return nil, domain.BadRequest(fmt.Sprintf("Invalid date format: %s", item.Date))
		}
		if !isValidTime(item.StartTime) || !isValidTime(item.EndTime) {
			return nil, domain.BadRequest(fmt.Sprintf("Invalid time: %s-%s", item.StartTime, item.EndTime))
		}
		if item.StartTime >= item.EndTime {
			return nil, domain.BadRequest("Start time must be before end time")
		}
		dur := minutesBetween(item.StartTime, item.EndTime)
		if dur < 60 {
			return nil, domain.BadRequest("Minimum booking duration is 1 hour")
		}
		if dur%30 != 0 {
			return nil, domain.BadRequest("Duration must be in 30-minute increments")
		}

		sc, err := s.availRepo.GetSubCourtWithCourt(ctx, item.SubCourtID)
		if err != nil {
			return nil, err
		}
		if sc == nil {
			return nil, domain.NotFound(fmt.Sprintf("Sub-court %s not found", item.SubCourtID))
		}
		if !sc.IsActive {
			return nil, domain.BadRequest(fmt.Sprintf("Sub-court %s is not active", sc.Name))
		}

		overlap, err := s.availRepo.HasOverlappingBooking(ctx, item.SubCourtID, item.Date, item.StartTime, item.EndTime)
		if err != nil {
			return nil, err
		}
		if overlap {
			return nil, domain.Conflict(fmt.Sprintf("Time slot %s-%s is already booked", item.StartTime, item.EndTime))
		}

		// Pricing
		isHol, _ := s.availRepo.IsHoliday(ctx, item.Date)
		mult, _ := s.availRepo.GetHolidayMultiplier(ctx, item.Date)
		d, _ := parseDate(item.Date)
		isWknd := d.Weekday() == 0 || d.Weekday() == 6
		dayType := "weekday"
		if isHol {
			dayType = "holiday"
		} else if isWknd {
			dayType = "weekend"
		}
		rules, _ := s.availRepo.GetPricingRulesByCourtID(ctx, sc.CourtID)
		totalPrice := calculateTotalPrice(item.StartTime, item.EndTime, rules, dayType, mult)

		repoItems = append(repoItems, repository.CreateBookingItem{
			SubCourtID: item.SubCourtID,
			Date:       item.Date,
			StartTime:  item.StartTime,
			EndTime:    item.EndTime,
			TotalPrice: totalPrice,
		})
	}

	uid := userID
	common := repository.CreateBookingCommon{
		GuestName:  req.GuestName,
		GuestPhone: req.GuestPhone,
		GuestEmail: req.GuestEmail,
		UserID:     &uid,
		Notes:      req.Notes,
		GroupID:    groupID,
	}
	if userID == "" {
		common.UserID = nil
	}

	ids, err := s.availRepo.CreateBookings(ctx, repoItems, common)
	if err != nil {
		return nil, err
	}

	var responses []*dto.BookingResponse
	for _, id := range ids {
		b, err := s.availRepo.GetBookingByID(ctx, id)
		if err != nil {
			return nil, err
		}
		responses = append(responses, mapBookingRow(b))
	}
	return responses, nil
}

// GetBookingByID returns a single booking.
func (s *AvailabilityService) GetBookingByID(ctx context.Context, id string) (*dto.BookingResponse, error) {
	b, err := s.availRepo.GetBookingByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, domain.NotFound("Booking not found")
	}
	return mapBookingRow(b), nil
}

// CancelBooking marks a booking as cancelled.
func (s *AvailabilityService) CancelBooking(ctx context.Context, id string) (*dto.BookingResponse, error) {
	b, err := s.availRepo.GetBookingByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, domain.NotFound("Booking not found")
	}
	if b.Status == "cancelled" {
		return nil, domain.BadRequest("Booking is already cancelled")
	}
	if b.Status == "completed" {
		return nil, domain.BadRequest("Cannot cancel a completed booking")
	}
	if err := s.availRepo.UpdateBookingStatus(ctx, id, "cancelled"); err != nil {
		return nil, err
	}
	updated, _ := s.availRepo.GetBookingByID(ctx, id)
	return mapBookingRow(updated), nil
}

// GetUserBookings returns paginated bookings for a user.
func (s *AvailabilityService) GetUserBookings(ctx context.Context, userID string, page, limit int) ([]*dto.BookingResponse, int, error) {
	rows, total, err := s.availRepo.GetUserBookings(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, err
	}
	var resp []*dto.BookingResponse
	for _, b := range rows {
		resp = append(resp, mapBookingRow(b))
	}
	return resp, total, nil
}

// ==================== Private Helpers ====================

func generateTimeSlots(
	openTime, closeTime string,
	bookings []*repository.RawBooking,
	closures []*repository.RawClosure,
	rules []*repository.RawPricingRule,
	dayType string,
	holidayMultiplier float64,
) []dto.TimeSlot {
	var slots []dto.TimeSlot
	current := openTime
	for current < closeTime {
		next := addMinutes(current, 30)
		booked := false
		for _, b := range bookings {
			if timesOverlap(current, next, b.StartTime, b.EndTime) {
				booked = true
				break
			}
		}
		closed := false
		for _, cl := range closures {
			if cl.StartTime == nil || cl.EndTime == nil {
				closed = true
				break
			}
			if timesOverlap(current, next, *cl.StartTime, *cl.EndTime) {
				closed = true
				break
			}
		}
		price := getPriceForSlot(current, rules, dayType, holidayMultiplier)
		slots = append(slots, dto.TimeSlot{
			StartTime:   current,
			EndTime:     next,
			IsAvailable: !booked && !closed,
			Price:       price,
		})
		current = next
	}
	return slots
}

func getPriceForSlot(time string, rules []*repository.RawPricingRule, dayType string, mult float64) int {
	for _, r := range rules {
		if r.DayType == dayType && r.IsActive && time >= r.StartTime && time < r.EndTime {
			base := r.PricePerHour / 2 // 30-min slot
			return int(math.Round(float64(base) * mult))
		}
	}
	return 0
}

func calculateTotalPrice(start, end string, rules []*repository.RawPricingRule, dayType string, mult float64) int {
	total := 0
	cur := start
	for cur < end {
		total += getPriceForSlot(cur, rules, dayType, mult)
		cur = addMinutes(cur, 30)
	}
	return total
}

func groupBookingsBySubCourt(bookings []*repository.RawBooking) map[string][]*repository.RawBooking {
	m := make(map[string][]*repository.RawBooking)
	for _, b := range bookings {
		m[b.SubCourtID] = append(m[b.SubCourtID], b)
	}
	return m
}

func groupClosuresBySubCourt(closures []*repository.RawClosure) map[string][]*repository.RawClosure {
	m := make(map[string][]*repository.RawClosure)
	for _, c := range closures {
		m[c.SubCourtID] = append(m[c.SubCourtID], c)
	}
	return m
}

func timesOverlap(s1, e1, s2, e2 string) bool {
	return s1 < e2 && e1 > s2
}

func addMinutes(t string, mins int) string {
	h, m := parseTime(t)
	total := h*60 + m + mins
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func parseTime(t string) (int, int) {
	var h, m int
	_, _ = fmt.Sscanf(t, "%d:%d", &h, &m)
	return h, m
}

func minutesBetween(start, end string) int {
	sh, sm := parseTime(start)
	eh, em := parseTime(end)
	return (eh*60 + em) - (sh*60 + sm)
}

func ohForDay(oh *OpeningHours, day string) *string {
	if oh == nil {
		return nil
	}
	hours, ok := (*oh)[day]
	if !ok || hours == "" {
		return nil
	}
	return &hours
}

func splitHours(s string) (string, string) {
	for i, c := range s {
		if c == '-' {
			return s[:i], s[i+1:]
		}
	}
	return "00:00", "23:59"
}

func mapBookingRow(b *repository.BookingRow) *dto.BookingResponse {
	return &dto.BookingResponse{
		ID:           b.ID,
		SubCourtID:   b.SubCourtID,
		SubCourtName: b.SubCourtName,
		CourtID:      b.CourtID,
		CourtName:    b.CourtName,
		GuestName:    b.GuestName,
		GuestPhone:   b.GuestPhone,
		GuestEmail:   b.GuestEmail,
		Date:         b.Date,
		StartTime:    b.StartTime,
		EndTime:      b.EndTime,
		TotalPrice:   b.TotalPrice,
		Status:       b.Status,
		Notes:        b.Notes,
		CreatedAt:    b.CreatedAt,
	}
}
