package repository

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RawSubCourt is a minimal row for availability calculations.
type RawSubCourt struct {
	ID          string
	CourtID     string
	Name        string
	Description *string
	IsActive    bool
}

// RawBooking is a minimal row for slot conflict checks.
type RawBooking struct {
	ID          string
	SubCourtID  string
	StartTime   string
	EndTime     string
	Status      string
}

// RawPricingRule is a minimal row for price calculation.
type RawPricingRule struct {
	ID           string
	DayType      string
	StartTime    string
	EndTime      string
	PricePerHour int
	IsActive     bool
}

// RawClosure is a minimal row for slot closure checks.
type RawClosure struct {
	ID          string
	SubCourtID  string
	StartTime   *string // nil = full day
	EndTime     *string
}

// BookingRow is a full booking row with joined names.
type BookingRow struct {
	ID           string
	SubCourtID   string
	SubCourtName string
	CourtID      string
	CourtName    string
	GuestName    string
	GuestPhone   string
	GuestEmail   *string
	Date         string
	StartTime    string
	EndTime      string
	TotalPrice   int
	Status       string
	Notes        *string
	CreatedAt    string
	GroupID      *string
	UserID       *string
}

type AvailabilityRepository struct {
	db *pgxpool.Pool
}

func NewAvailabilityRepository(db *pgxpool.Pool) *AvailabilityRepository {
	return &AvailabilityRepository{db: db}
}

func (r *AvailabilityRepository) GetSubCourtsByCourtID(ctx context.Context, courtID string) ([]*RawSubCourt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, court_id, name, description, is_active
		FROM sub_courts
		WHERE court_id = $1::uuid AND is_active = true
		ORDER BY name
	`, courtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*RawSubCourt
	for rows.Next() {
		sc := &RawSubCourt{}
		if err := rows.Scan(&sc.ID, &sc.CourtID, &sc.Name, &sc.Description, &sc.IsActive); err != nil {
			return nil, err
		}
		result = append(result, sc)
	}
	return result, rows.Err()
}

func (r *AvailabilityRepository) GetBookingsByCourtAndDate(ctx context.Context, courtID, date string) ([]*RawBooking, error) {
	rows, err := r.db.Query(ctx, `
		SELECT b.id, b.sub_court_id,
		       TO_CHAR(b.start_time, 'HH24:MI') as start_time,
		       TO_CHAR(b.end_time, 'HH24:MI') as end_time,
		       b.status
		FROM bookings b
		JOIN sub_courts sc ON b.sub_court_id = sc.id
		WHERE sc.court_id = $1::uuid
		  AND b.date = $2::date
		  AND b.status = 'confirmed'
		ORDER BY b.start_time
	`, courtID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*RawBooking
	for rows.Next() {
		b := &RawBooking{}
		if err := rows.Scan(&b.ID, &b.SubCourtID, &b.StartTime, &b.EndTime, &b.Status); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

func (r *AvailabilityRepository) GetPricingRulesByCourtID(ctx context.Context, courtID string) ([]*RawPricingRule, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, day_type,
		       TO_CHAR(start_time, 'HH24:MI') as start_time,
		       TO_CHAR(end_time, 'HH24:MI') as end_time,
		       price_per_hour, is_active
		FROM pricing_rules
		WHERE court_id = $1::uuid AND is_active = true
		ORDER BY start_time
	`, courtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*RawPricingRule
	for rows.Next() {
		pr := &RawPricingRule{}
		if err := rows.Scan(&pr.ID, &pr.DayType, &pr.StartTime, &pr.EndTime, &pr.PricePerHour, &pr.IsActive); err != nil {
			return nil, err
		}
		result = append(result, pr)
	}
	return result, rows.Err()
}

func (r *AvailabilityRepository) GetClosuresByCourtAndDate(ctx context.Context, courtID, date string) ([]*RawClosure, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.sub_court_id,
		       TO_CHAR(c.start_time, 'HH24:MI') as start_time,
		       TO_CHAR(c.end_time, 'HH24:MI') as end_time
		FROM sub_court_closures c
		JOIN sub_courts sc ON c.sub_court_id = sc.id
		WHERE sc.court_id = $1::uuid AND c.date = $2::date
	`, courtID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*RawClosure
	for rows.Next() {
		cl := &RawClosure{}
		if err := rows.Scan(&cl.ID, &cl.SubCourtID, &cl.StartTime, &cl.EndTime); err != nil {
			return nil, err
		}
		result = append(result, cl)
	}
	return result, rows.Err()
}

func (r *AvailabilityRepository) IsHoliday(ctx context.Context, date string) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM holidays WHERE date = $1::date`, date).Scan(&count)
	return count > 0, err
}

func (r *AvailabilityRepository) GetHolidayMultiplier(ctx context.Context, date string) (float64, error) {
	var multiplier float64
	err := r.db.QueryRow(ctx, `SELECT multiplier FROM holidays WHERE date = $1::date`, date).Scan(&multiplier)
	if err == pgx.ErrNoRows {
		return 1.0, nil
	}
	return multiplier, err
}

func (r *AvailabilityRepository) GetSubCourtWithCourt(ctx context.Context, subCourtID string) (*RawSubCourt, error) {
	sc := &RawSubCourt{}
	err := r.db.QueryRow(ctx, `
		SELECT sc.id, sc.court_id, sc.name, sc.description, sc.is_active
		FROM sub_courts sc
		WHERE sc.id = $1::uuid
	`, subCourtID).Scan(&sc.ID, &sc.CourtID, &sc.Name, &sc.Description, &sc.IsActive)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return sc, err
}

func (r *AvailabilityRepository) HasOverlappingBooking(ctx context.Context, subCourtID, date, startTime, endTime string) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM bookings
		WHERE sub_court_id = $1::uuid
		  AND date = $2::date
		  AND status = 'confirmed'
		  AND (start_time < $4::time AND end_time > $3::time)
	`, subCourtID, date, startTime, endTime).Scan(&count)
	return count > 0, err
}

type CreateBookingItem struct {
	SubCourtID string
	Date       string
	StartTime  string
	EndTime    string
	TotalPrice int
}

type CreateBookingCommon struct {
	GuestName  string
	GuestPhone string
	GuestEmail *string
	UserID     *string
	Notes      *string
	GroupID    string
}

// CreateBookings inserts multiple bookings in a transaction and returns their IDs.
func (r *AvailabilityRepository) CreateBookings(ctx context.Context, items []CreateBookingItem, common CreateBookingCommon) ([]string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var ids []string
	for _, item := range items {
		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO bookings (
				sub_court_id, guest_name, guest_phone, guest_email,
				user_id, date, start_time, end_time, total_price, status, notes, group_id
			) VALUES (
				$1::uuid, $2, $3, $4,
				$5::uuid, $6::date, $7::time, $8::time, $9, 'pending', $10, $11::uuid
			) RETURNING id
		`, item.SubCourtID, common.GuestName, common.GuestPhone, common.GuestEmail,
			common.UserID, item.Date, item.StartTime, item.EndTime, item.TotalPrice,
			common.Notes, common.GroupID,
		).Scan(&id)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

// GetBookingByID returns a booking with joined court/subcourt names.
func (r *AvailabilityRepository) GetBookingByID(ctx context.Context, id string) (*BookingRow, error) {
	b := &BookingRow{}
	err := r.db.QueryRow(ctx, `
		SELECT b.id, b.sub_court_id, sc.name, sc.court_id, c.name,
		       COALESCE(b.guest_name,''), COALESCE(b.guest_phone,''), b.guest_email,
		       TO_CHAR(b.date, 'YYYY-MM-DD'),
		       TO_CHAR(b.start_time, 'HH24:MI'), TO_CHAR(b.end_time, 'HH24:MI'),
		       b.total_price, b.status, b.notes,
		       TO_CHAR(b.created_at, 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
		       b.group_id::text, b.user_id::text
		FROM bookings b
		JOIN sub_courts sc ON b.sub_court_id = sc.id
		JOIN courts c ON sc.court_id = c.id
		WHERE b.id = $1::uuid
	`, id).Scan(
		&b.ID, &b.SubCourtID, &b.SubCourtName, &b.CourtID, &b.CourtName,
		&b.GuestName, &b.GuestPhone, &b.GuestEmail,
		&b.Date, &b.StartTime, &b.EndTime,
		&b.TotalPrice, &b.Status, &b.Notes, &b.CreatedAt,
		&b.GroupID, &b.UserID,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return b, err
}

// GetBookingsByGroupID fetches all bookings in a group.
func (r *AvailabilityRepository) GetBookingsByGroupID(ctx context.Context, groupID string) ([]*RawBooking, error) {
	rows, err := r.db.Query(ctx, `
		SELECT b.id, b.sub_court_id,
		       TO_CHAR(b.start_time, 'HH24:MI'), TO_CHAR(b.end_time, 'HH24:MI'),
		       b.status
		FROM bookings b
		WHERE b.group_id = $1::uuid
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*RawBooking
	for rows.Next() {
		b := &RawBooking{}
		if err := rows.Scan(&b.ID, &b.SubCourtID, &b.StartTime, &b.EndTime, &b.Status); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

// UpdateBookingStatus sets a booking's status.
func (r *AvailabilityRepository) UpdateBookingStatus(ctx context.Context, id, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE bookings SET status = $2, updated_at = NOW() WHERE id = $1::uuid
	`, id, status)
	return err
}

// GetUserBookings returns paginated booking history for a user.
func (r *AvailabilityRepository) GetUserBookings(ctx context.Context, userID string, page, limit int) ([]*BookingRow, int, error) {
	offset := (page - 1) * limit

	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM bookings WHERE user_id = $1::uuid`, userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT b.id, b.sub_court_id, sc.name, sc.court_id, c.name,
		       COALESCE(b.guest_name,''), COALESCE(b.guest_phone,''), b.guest_email,
		       TO_CHAR(b.date, 'YYYY-MM-DD'),
		       TO_CHAR(b.start_time, 'HH24:MI'), TO_CHAR(b.end_time, 'HH24:MI'),
		       b.total_price, b.status, b.notes,
		       TO_CHAR(b.created_at, 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
		       b.group_id::text, b.user_id::text
		FROM bookings b
		JOIN sub_courts sc ON b.sub_court_id = sc.id
		JOIN courts c ON sc.court_id = c.id
		WHERE b.user_id = $1::uuid
		ORDER BY b.date DESC, b.start_time DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []*BookingRow
	for rows.Next() {
		b := &BookingRow{}
		if err := rows.Scan(
			&b.ID, &b.SubCourtID, &b.SubCourtName, &b.CourtID, &b.CourtName,
			&b.GuestName, &b.GuestPhone, &b.GuestEmail,
			&b.Date, &b.StartTime, &b.EndTime,
			&b.TotalPrice, &b.Status, &b.Notes, &b.CreatedAt,
			&b.GroupID, &b.UserID,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, b)
	}
	return result, total, rows.Err()
}

// MarkCompletedBookings marks past confirmed bookings as completed (scheduler).
func (r *AvailabilityRepository) MarkCompletedBookings(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE bookings
		SET status = 'completed', updated_at = NOW()
		WHERE status = 'confirmed'
		  AND (date + end_time) < (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')
	`)
	return tag.RowsAffected(), err
}

// MarkExpiredPendingBookings marks old pending bookings as cancelled (scheduler).
func (r *AvailabilityRepository) MarkExpiredPendingBookings(ctx context.Context, timeoutSec int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		UPDATE payments SET status = 'failed', updated_at = NOW()
		WHERE status = 'pending'
		  AND created_at < NOW() - ($1 || ' seconds')::interval
	`, strconv.Itoa(timeoutSec))
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE bookings SET status = 'cancelled', updated_at = NOW()
		WHERE status = 'pending'
		  AND created_at < NOW() - ($1 || ' seconds')::interval
	`, strconv.Itoa(timeoutSec))
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// LinkGuestBookings links guest bookings (by phone) to a user account.
func (r *AvailabilityRepository) LinkGuestBookings(ctx context.Context, phone, userID string) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE bookings SET user_id = $2::uuid, updated_at = NOW()
		WHERE guest_phone = $1 AND user_id IS NULL
	`, phone, userID)
	return tag.RowsAffected(), err
}
