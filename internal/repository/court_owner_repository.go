package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

func (r *CourtOwnerRepository) CreateCourtFromSpecs(ctx context.Context, ownerID string, specs domain.OperationalSpecs) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

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
