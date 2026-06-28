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

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// FindByFirebaseUID fetches a user by their Firebase UID.
func (r *UserRepository) FindByFirebaseUID(ctx context.Context, uid string) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, firebase_uid, email, username, provider, is_anonymous,
		       first_name, last_name, gender, phone_number, photo_url,
		       address_street, address_ward, address_district, address_city,
		       fcm_tokens, created_at, updated_at
		FROM users WHERE firebase_uid = $1
	`, uid)
	return scanUser(row)
}

// FindByID fetches a user by their internal UUID.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, firebase_uid, email, username, provider, is_anonymous,
		       first_name, last_name, gender, phone_number, photo_url,
		       address_street, address_ward, address_district, address_city,
		       fcm_tokens, created_at, updated_at
		FROM users WHERE id = $1::uuid
	`, id)
	return scanUser(row)
}

// FindByUsername returns a user by username, or nil if not found.
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, firebase_uid, email, username, provider, is_anonymous,
		       first_name, last_name, gender, phone_number, photo_url,
		       address_street, address_ward, address_district, address_city,
		       fcm_tokens, created_at, updated_at
		FROM users WHERE username = $1
	`, username)
	return scanUser(row)
}

// Upsert creates or updates a user on Firebase login.
// Returns the user and isNewUser bool.
func (r *UserRepository) Upsert(ctx context.Context, u *domain.User) (*domain.User, bool, error) {
	var isNew bool
	row := r.db.QueryRow(ctx, `
		INSERT INTO users (firebase_uid, email, username, provider, is_anonymous,
		                   first_name, last_name, gender, phone_number, photo_url,
		                   address_city)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'Hà Nội')
		ON CONFLICT (firebase_uid) DO UPDATE
		  SET updated_at = NOW()
		RETURNING id, firebase_uid, email, username, provider, is_anonymous,
		          first_name, last_name, gender, phone_number, photo_url,
		          address_street, address_ward, address_district, address_city,
		          fcm_tokens, created_at, updated_at,
		          (xmax = 0) AS is_new
	`, u.FirebaseUID, u.Email, u.Username, u.Provider, u.IsAnonymous,
		u.FirstName, u.LastName, u.Gender, u.PhoneNumber, u.PhotoURL)

	user := &domain.User{}
	err := row.Scan(
		&user.ID, &user.FirebaseUID, &user.Email, &user.Username, &user.Provider, &user.IsAnonymous,
		&user.FirstName, &user.LastName, &user.Gender, &user.PhoneNumber, &user.PhotoURL,
		&user.AddressStreet, &user.AddressWard, &user.AddressDistrict, &user.AddressCity,
		&user.FCMTokens, &user.CreatedAt, &user.UpdatedAt, &isNew,
	)
	if err != nil {
		return nil, false, err
	}
	return user, isNew, nil
}

// UpdateProfile updates editable profile fields for a user.
func (r *UserRepository) UpdateProfile(ctx context.Context, id string, fields map[string]interface{}) (*domain.User, error) {
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
		UPDATE users SET %s, updated_at = NOW()
		WHERE id = $%d::uuid
		RETURNING id, firebase_uid, email, username, provider, is_anonymous,
		          first_name, last_name, gender, phone_number, photo_url,
		          address_street, address_ward, address_district, address_city,
		          fcm_tokens, created_at, updated_at
	`, strings.Join(sets, ", "), i)

	return scanUser(r.db.QueryRow(ctx, query, args...))
}

// AddFCMToken appends a unique FCM token to the user's array.
func (r *UserRepository) AddFCMToken(ctx context.Context, userID, token string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET fcm_tokens = array_append(array_remove(fcm_tokens, $2), $2),
		    updated_at = NOW()
		WHERE id = $1::uuid
	`, userID, token)
	return err
}

// RemoveFCMToken removes a specific FCM token.
func (r *UserRepository) RemoveFCMToken(ctx context.Context, userID, token string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET fcm_tokens = array_remove(fcm_tokens, $2), updated_at = NOW()
		WHERE id = $1::uuid
	`, userID, token)
	return err
}

// Delete permanently deletes a user account.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM users WHERE id = $1::uuid`, id)
	return err
}

// UpdateRoles updates the roles array for a user.
func (r *UserRepository) UpdateRoles(ctx context.Context, id string, roles []string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET roles = $2, updated_at = NOW()
		WHERE id = $1::uuid
	`, id, roles)
	return err
}

// HasRole checks if a user has a specific role.
func (r *UserRepository) HasRole(ctx context.Context, id string, role string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users
			WHERE id = $1::uuid AND $2 = ANY(roles)
		)
	`, id, role).Scan(&exists)
	return exists, err
}

// GetRoles returns the roles array for a user.
func (r *UserRepository) GetRoles(ctx context.Context, id string) ([]string, error) {
	var roles []string
	rows, err := r.db.Query(ctx, `SELECT unnest(roles) FROM users WHERE id = $1::uuid`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

func scanUser(row pgx.Row) (*domain.User, error) {
	u := &domain.User{}
	err := row.Scan(
		&u.ID, &u.FirebaseUID, &u.Email, &u.Username, &u.Provider, &u.IsAnonymous,
		&u.FirstName, &u.LastName, &u.Gender, &u.PhoneNumber, &u.PhotoURL,
		&u.AddressStreet, &u.AddressWard, &u.AddressDistrict, &u.AddressCity,
		&u.FCMTokens, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}
