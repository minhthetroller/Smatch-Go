package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smatch/badminton-backend/internal/domain"
)

type CourtRepository struct {
	db *pgxpool.Pool
}

func NewCourtRepository(db *pgxpool.Pool) *CourtRepository {
	return &CourtRepository{db: db}
}

const courtSelectCols = `
	id, name, description, phone_numbers,
	address_street, address_ward, address_district, address_city,
	details, opening_hours,
	ST_Y(location) AS lat,
	ST_X(location) AS lng,
	created_at, updated_at
`

func scanCourt(row pgx.Row) (*domain.Court, error) {
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

func scanCourtWithDistance(rows pgx.Rows) ([]*domain.Court, []float64, error) {
	var courts []*domain.Court
	var distances []float64
	for rows.Next() {
		c := &domain.Court{}
		var dist float64
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Description, &c.PhoneNumbers,
			&c.AddressStreet, &c.AddressWard, &c.AddressDistrict, &c.AddressCity,
			&c.Details, &c.OpeningHours, &c.Lat, &c.Lng,
			&c.CreatedAt, &c.UpdatedAt, &dist,
		); err != nil {
			return nil, nil, err
		}
		courts = append(courts, c)
		distances = append(distances, dist)
	}
	return courts, distances, rows.Err()
}

// FindAll returns all courts (no geo).
func (r *CourtRepository) FindAll(ctx context.Context) ([]*domain.Court, error) {
	rows, err := r.db.Query(ctx, `SELECT `+courtSelectCols+` FROM courts ORDER BY name`)
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

// FindByID returns a single court.
func (r *CourtRepository) FindByID(ctx context.Context, id string) (*domain.Court, error) {
	return scanCourt(r.db.QueryRow(ctx,
		`SELECT `+courtSelectCols+` FROM courts WHERE id = $1::uuid`, id))
}

// FindNearby returns courts within radiusMeters of (lat, lng), sorted by distance.
func (r *CourtRepository) FindNearby(ctx context.Context, lat, lng, radiusMeters float64) ([]*domain.Court, []float64, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+courtSelectCols+`,
		       ST_Distance(location::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) AS distance
		FROM courts
		WHERE ST_DWithin(location::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		ORDER BY distance
	`, lng, lat, radiusMeters)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	return scanCourtWithDistance(rows)
}

// Create inserts a new court.
func (r *CourtRepository) Create(ctx context.Context, c *domain.Court) (*domain.Court, error) {
	var locationSQL string
	args := []interface{}{
		c.Name, c.Description, c.PhoneNumbers,
		c.AddressStreet, c.AddressWard, c.AddressDistrict, c.AddressCity,
		c.Details, c.OpeningHours,
	}

	if c.Lat != nil && c.Lng != nil {
		locationSQL = `ST_SetSRID(ST_MakePoint($10, $11), 4326)`
		args = append(args, *c.Lng, *c.Lat) // PostGIS: MakePoint(lng, lat)
	} else {
		locationSQL = `NULL`
	}

	query := fmt.Sprintf(`
		INSERT INTO courts (name, description, phone_numbers,
		                    address_street, address_ward, address_district, address_city,
		                    details, opening_hours, location)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, %s)
		RETURNING `+courtSelectCols, locationSQL)

	return scanCourt(r.db.QueryRow(ctx, query, args...))
}

// Update applies partial updates to a court.
func (r *CourtRepository) Update(ctx context.Context, id string, fields map[string]interface{}) (*domain.Court, error) {
	if len(fields) == 0 {
		return r.FindByID(ctx, id)
	}

	sets := make([]string, 0, len(fields))
	args := make([]interface{}, 0, len(fields)+1)
	i := 1
	for col, val := range fields {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE courts SET %s, updated_at = NOW()
		WHERE id = $%d::uuid
		RETURNING `+courtSelectCols, strings.Join(sets, ", "), i)

	return scanCourt(r.db.QueryRow(ctx, query, args...))
}

// Delete removes a court by ID.
func (r *CourtRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM courts WHERE id = $1::uuid`, id)
	return err
}
