package repository

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smatch/badminton-backend/internal/domain"
)

type PaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func scanPayment(row pgx.Row) (*domain.Payment, error) {
	p := &domain.Payment{}
	var callbackData []byte
	err := row.Scan(
		&p.ID, &p.BookingID, &p.MatchPlayerID, &p.PaymentType,
		&p.AppTransID, &p.ZPTransID, &p.ZPTransToken,
		&p.Amount, &p.Status, &p.OrderURL,
		&callbackData, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.CallbackData = json.RawMessage(callbackData)
	return p, nil
}

const paymentCols = `
	id, booking_id, match_player_id, payment_type,
	app_trans_id, zp_trans_id, zp_trans_token,
	amount, status, order_url, callback_data, created_at, updated_at
`

// FindByID returns a payment by its UUID.
func (r *PaymentRepository) FindByID(ctx context.Context, id string) (*domain.Payment, error) {
	return scanPayment(r.db.QueryRow(ctx,
		`SELECT `+paymentCols+` FROM payments WHERE id = $1::uuid`, id))
}

// FindByAppTransID returns a payment by ZaloPay app_trans_id.
func (r *PaymentRepository) FindByAppTransID(ctx context.Context, appTransID string) (*domain.Payment, error) {
	return scanPayment(r.db.QueryRow(ctx,
		`SELECT `+paymentCols+` FROM payments WHERE app_trans_id = $1`, appTransID))
}

// FindLatestPendingByBookingID returns the most recent pending payment for a booking.
func (r *PaymentRepository) FindLatestPendingByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error) {
	return scanPayment(r.db.QueryRow(ctx,
		`SELECT `+paymentCols+`
		 FROM payments WHERE booking_id = $1::uuid AND status = 'pending'
		 ORDER BY created_at DESC LIMIT 1`, bookingID))
}

// HasSuccessfulPayment checks whether a booking already has a successful payment.
func (r *PaymentRepository) HasSuccessfulPayment(ctx context.Context, bookingID string) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE booking_id = $1::uuid AND status = 'success'`,
		bookingID).Scan(&count)
	return count > 0, err
}

// Create inserts a new payment record and returns it.
func (r *PaymentRepository) Create(ctx context.Context, bookingID *string, matchPlayerID *string,
	paymentType domain.PaymentType, appTransID string, amount int) (*domain.Payment, error) {
	return scanPayment(r.db.QueryRow(ctx, `
		INSERT INTO payments (booking_id, match_player_id, payment_type, app_trans_id, amount, status)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, 'pending')
		RETURNING `+paymentCols,
		bookingID, matchPlayerID, string(paymentType), appTransID, amount))
}

// UpdateOrderURL sets the ZaloPay order URL and trans token after order creation.
func (r *PaymentRepository) UpdateOrderURL(ctx context.Context, id, orderURL, zpTransToken string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE payments SET order_url = $2, zp_trans_token = $3, updated_at = NOW()
		WHERE id = $1::uuid
	`, id, orderURL, zpTransToken)
	return err
}

// UpdateStatus sets payment status and optionally the ZaloPay transaction ID.
func (r *PaymentRepository) UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus, zpTransID *string, callbackData json.RawMessage) error {
	_, err := r.db.Exec(ctx, `
		UPDATE payments
		SET status = $2, zp_trans_id = $3, callback_data = $4, updated_at = NOW()
		WHERE id = $1::uuid
	`, id, string(status), zpTransID, []byte(callbackData))
	return err
}

// UpdateStatusByAppTransID updates payment by app_trans_id (for callback processing).
func (r *PaymentRepository) UpdateStatusByAppTransID(ctx context.Context, appTransID string, status domain.PaymentStatus, zpTransID *string, callbackData json.RawMessage) (*domain.Payment, error) {
	return scanPayment(r.db.QueryRow(ctx, `
		UPDATE payments
		SET status = $2, zp_trans_id = $3, callback_data = $4, updated_at = NOW()
		WHERE app_trans_id = $1
		RETURNING `+paymentCols,
		appTransID, string(status), zpTransID, []byte(callbackData)))
}

// MarkExpiredMatchPayments marks old pending MATCH_JOIN payments as expired.
func (r *PaymentRepository) MarkExpiredMatchPayments(ctx context.Context, timeoutSec int) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE payments
		SET status = 'expired', updated_at = NOW()
		WHERE payment_type = 'MATCH_JOIN'
		  AND status = 'pending'
		  AND created_at < NOW() - ($1 || ' seconds')::interval
		RETURNING match_player_id::text
	`, strconv.Itoa(timeoutSec))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playerIDs []string
	for rows.Next() {
		var id *string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id != nil {
			playerIDs = append(playerIDs, *id)
		}
	}
	return playerIDs, rows.Err()
}
