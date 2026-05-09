package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smatch/badminton-backend/internal/domain"
)

type CourtOwnerRepository struct {
	db *pgxpool.Pool
}

func NewCourtOwnerRepository(db *pgxpool.Pool) *CourtOwnerRepository {
	return &CourtOwnerRepository{db: db}
}

func (r *CourtOwnerRepository) ListCourtsByOwner(ctx context.Context, ownerID string) ([]*domain.Court, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, description, phone_numbers,
		       address_street, address_ward, address_district, address_city,
		       details, opening_hours,
		       ST_Y(location) AS lat,
		       ST_X(location) AS lng,
		       created_at, updated_at
		FROM courts
		WHERE owner_user_id = $1::uuid
		ORDER BY name
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courts []*domain.Court
	for rows.Next() {
		c := &domain.Court{}
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Description, &c.PhoneNumbers,
			&c.AddressStreet, &c.AddressWard, &c.AddressDistrict, &c.AddressCity,
			&c.Details, &c.OpeningHours, &c.Lat, &c.Lng,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		courts = append(courts, c)
	}
	return courts, rows.Err()
}

func (r *CourtOwnerRepository) GetCourtStats(ctx context.Context, courtID string, from, to time.Time) (*domain.CourtStats, error) {
	stats := &domain.CourtStats{}

	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(total_price), 0)
		FROM bookings b
		JOIN sub_courts sc ON b.sub_court_id = sc.id
		WHERE sc.court_id = $1::uuid
		  AND b.date BETWEEN $2::date AND $3::date
		  AND b.status IN ('confirmed', 'completed')
	`, courtID, from, to).Scan(&stats.TotalBookings, &stats.TotalRevenue)
	if err != nil {
		return nil, err
	}

	var cancelled int64
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM bookings b
		JOIN sub_courts sc ON b.sub_court_id = sc.id
		WHERE sc.court_id = $1::uuid
		  AND b.date BETWEEN $2::date AND $3::date
		  AND b.status = 'cancelled'
	`, courtID, from, to).Scan(&cancelled)
	if err != nil {
		return nil, err
	}

	var totalSlots int64
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) * 24
		FROM sub_courts
		WHERE court_id = $1::uuid AND is_active = true
	`, courtID).Scan(&totalSlots)
	if err != nil {
		return nil, err
	}

	if totalSlots > 0 {
		stats.OccupancyRate = float64(stats.TotalBookings) / float64(totalSlots)
	}
	if stats.TotalBookings+cancelled > 0 {
		stats.CancellationRate = float64(cancelled) / float64(stats.TotalBookings+cancelled)
	}
	return stats, nil
}

func (r *CourtOwnerRepository) CloseSubCourt(ctx context.Context, subCourtID, date, startTime, endTime, reason string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO sub_court_closures (sub_court_id, date, start_time, end_time, reason)
		VALUES ($1::uuid, $2::date, $3::time, $4::time, $5)
	`, subCourtID, date, startTime, endTime, reason)
	return err
}

func (r *CourtOwnerRepository) OpenSubCourt(ctx context.Context, subCourtID, date string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM sub_court_closures
		WHERE sub_court_id = $1::uuid AND date = $2::date
	`, subCourtID, date)
	return err
}

func (r *CourtOwnerRepository) SetSubCourtActive(ctx context.Context, subCourtID string, active bool) error {
	_, err := r.db.Exec(ctx, `
		UPDATE sub_courts SET is_active = $2, updated_at = NOW()
		WHERE id = $1::uuid
	`, subCourtID, active)
	return err
}

func (r *CourtOwnerRepository) CloseAllSubCourts(ctx context.Context, courtID, date, startTime, endTime, reason string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO sub_court_closures (sub_court_id, date, start_time, end_time, reason)
		SELECT id, $2::date, $3::time, $4::time, $5
		FROM sub_courts
		WHERE court_id = $1::uuid
	`, courtID, date, startTime, endTime, reason)
	return err
}

func (r *CourtOwnerRepository) OpenAllSubCourts(ctx context.Context, courtID, date string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM sub_court_closures
		WHERE sub_court_id IN (SELECT id FROM sub_courts WHERE court_id = $1::uuid)
		  AND date = $2::date
	`, courtID, date)
	return err
}

func (r *CourtOwnerRepository) GetCourtByOwner(ctx context.Context, ownerID, courtID string) (*domain.Court, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, name, description, phone_numbers,
		       address_street, address_ward, address_district, address_city,
		       details, opening_hours,
		       ST_Y(location) AS lat,
		       ST_X(location) AS lng,
		       created_at, updated_at
		FROM courts
		WHERE id = $1::uuid AND owner_user_id = $2::uuid
	`, courtID, ownerID)

	c := &domain.Court{}
	err := row.Scan(
		&c.ID, &c.Name, &c.Description, &c.PhoneNumbers,
		&c.AddressStreet, &c.AddressWard, &c.AddressDistrict, &c.AddressCity,
		&c.Details, &c.OpeningHours, &c.Lat, &c.Lng,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func (r *CourtOwnerRepository) ListSubCourtsByCourt(ctx context.Context, courtID string) ([]*domain.SubCourt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, court_id, name, description, is_active, created_at, updated_at
		FROM sub_courts
		WHERE court_id = $1::uuid
		ORDER BY name
	`, courtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.SubCourt
	for rows.Next() {
		sc := &domain.SubCourt{}
		if err := rows.Scan(&sc.ID, &sc.CourtID, &sc.Name, &sc.Description, &sc.IsActive, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, sc)
	}
	return items, rows.Err()
}

func (r *CourtOwnerRepository) ListPricingRulesByCourt(ctx context.Context, courtID string) ([]*domain.PricingRule, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, court_id, name, day_type, start_time, end_time, price_per_hour, is_active, created_at, updated_at
		FROM pricing_rules
		WHERE court_id = $1::uuid AND is_active = true
		ORDER BY day_type, start_time
	`, courtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.PricingRule
	for rows.Next() {
		pr := &domain.PricingRule{}
		if err := rows.Scan(&pr.ID, &pr.CourtID, &pr.Name, &pr.DayType, &pr.StartTime, &pr.EndTime, &pr.PricePerHour, &pr.IsActive, &pr.CreatedAt, &pr.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, pr)
	}
	return items, rows.Err()
}

func (r *CourtOwnerRepository) ListUpcomingClosuresByCourt(ctx context.Context, courtID string, from time.Time) ([]*domain.SubCourtClosure, error) {
	rows, err := r.db.Query(ctx, `
		SELECT sc.id, sc.sub_court_id, sc.date, sc.start_time, sc.end_time, sc.reason, sc.created_at
		FROM sub_court_closures sc
		JOIN sub_courts s ON sc.sub_court_id = s.id
		WHERE s.court_id = $1::uuid AND sc.date >= $2::date
		ORDER BY sc.date, sc.start_time
		LIMIT 100
	`, courtID, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.SubCourtClosure
	for rows.Next() {
		cl := &domain.SubCourtClosure{}
		if err := rows.Scan(&cl.ID, &cl.SubCourtID, &cl.Date, &cl.StartTime, &cl.EndTime, &cl.Reason, &cl.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, cl)
	}
	return items, rows.Err()
}

func (r *CourtOwnerRepository) GetCourtStatsDaily(ctx context.Context, courtID string, from, to time.Time) ([]*domain.CourtStatsDaily, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DATE(b.date) AS day,
		       COUNT(*) FILTER (WHERE b.status IN ('confirmed', 'completed')) AS bookings,
		       COALESCE(SUM(b.total_price) FILTER (WHERE b.status IN ('confirmed', 'completed')), 0) AS revenue,
		       COUNT(*) FILTER (WHERE b.status = 'cancelled') AS cancellations
		FROM bookings b
		JOIN sub_courts sc ON b.sub_court_id = sc.id
		WHERE sc.court_id = $1::uuid
		  AND b.date BETWEEN $2::date AND $3::date
		GROUP BY DATE(b.date)
		ORDER BY day
	`, courtID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.CourtStatsDaily
	for rows.Next() {
		d := &domain.CourtStatsDaily{}
		if err := rows.Scan(&d.Date, &d.Bookings, &d.Revenue, &d.Cancellations); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (r *CourtOwnerRepository) CreateCourtFromSpecs(ctx context.Context, ownerID string, specs domain.OperationalSpecs) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	openingHours, _ := json.Marshal(specs.OperatingHours)

	var courtID string
	err = tx.QueryRow(ctx, `
		INSERT INTO courts (name, phone_numbers, details, opening_hours, owner_user_id)
		VALUES ($1, '{}', '{}', $2, $3::uuid)
		RETURNING id
	`, specs.SurfaceType+" Court", openingHours, ownerID).Scan(&courtID)
	if err != nil {
		return "", err
	}

	for i := 0; i < specs.SubcourtCount; i++ {
		_, err = tx.Exec(ctx, `
			INSERT INTO sub_courts (court_id, name)
			VALUES ($1::uuid, $2)
		`, courtID, fmt.Sprintf("Court %d", i+1))
		if err != nil {
			return "", err
		}
	}

	for _, pr := range specs.BasePricing {
		_, err = tx.Exec(ctx, `
			INSERT INTO pricing_rules (court_id, name, day_type, start_time, end_time, price_per_hour)
			VALUES ($1::uuid, $2, $3, $4::time, $5::time, $6)
		`, courtID, "Base", pr.DayType, pr.StartTime, pr.EndTime, pr.PricePerHour)
		if err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return courtID, nil
}
