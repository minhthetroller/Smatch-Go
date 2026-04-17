package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smatch/badminton-backend/internal/domain"
)

type SearchRepository struct {
	db *pgxpool.Pool
}

func NewSearchRepository(db *pgxpool.Pool) *SearchRepository {
	return &SearchRepository{db: db}
}

// SearchCourts performs full-text + trigram search on courts.
func (r *SearchRepository) SearchCourts(ctx context.Context, query string, page, limit int) ([]*domain.Court, int, error) {
	offset := (page - 1) * limit

	var total int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM courts
		WHERE name ILIKE $1 OR address_district ILIKE $1 OR address_ward ILIKE $1
	`, "%"+query+"%").Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT `+courtSelectCols+`
		FROM courts
		WHERE name ILIKE $1 OR address_district ILIKE $1 OR address_ward ILIKE $1
		ORDER BY name
		LIMIT $2 OFFSET $3
	`, "%"+query+"%", limit, offset)
	if err != nil {
		return nil, 0, err
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
			return nil, 0, err
		}
		courts = append(courts, c)
	}
	return courts, total, rows.Err()
}

// GetAllCourtNames returns all court IDs and names (for autocomplete reindex).
func (r *SearchRepository) GetAllCourtNames(ctx context.Context) ([]struct{ ID, Name, District string }, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name, COALESCE(address_district,'') FROM courts ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type CourtName struct {
		ID       string
		Name     string
		District string
	}
	var result []struct{ ID, Name, District string }
	for rows.Next() {
		var cn struct{ ID, Name, District string }
		if err := rows.Scan(&cn.ID, &cn.Name, &cn.District); err != nil {
			return nil, err
		}
		result = append(result, cn)
	}
	return result, rows.Err()
}
