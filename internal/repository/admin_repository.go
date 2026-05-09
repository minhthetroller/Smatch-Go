package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
)

type AdminRepository struct {
	db *pgxpool.Pool
}

func NewAdminRepository(db *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) GetPlatformStats(ctx context.Context) (*domain.PlatformStats, error) {
	stats := &domain.PlatformStats{}

	err := r.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT user_id)
		FROM bookings
		WHERE date >= CURRENT_DATE - INTERVAL '30 days'
		  AND status IN ('confirmed', 'completed')
	`).Scan(&stats.TotalActiveUsers)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT user_id)
		FROM business_profile_applications
		WHERE status = 'approved'
	`).Scan(&stats.TotalCourtOwners)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM courts
	`).Scan(&stats.TotalCourts)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM payments
		WHERE status = 'success'
	`).Scan(&stats.TotalRevenue)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM business_profile_applications WHERE status = 'pending'
	`).Scan(&stats.PendingApplications)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE - INTERVAL '7 days'
	`).Scan(&stats.RecentSignups)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *AdminRepository) CreateAuditLog(ctx context.Context, log *domain.AdminAuditLog) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO admin_audit_logs (admin_user_id, action, target_type, target_id, details)
		VALUES ($1::uuid, $2, $3, $4::uuid, $5)
	`, log.AdminUserID, log.Action, log.TargetType, log.TargetID, log.Details)
	return err
}

func (r *AdminRepository) GetSignupsTimeseries(ctx context.Context, fromHours int, bucket string) ([]dto.TimeseriesPoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT to_char(date_trunc($1, created_at), 'YYYY-MM-DD HH24:00') as label, COUNT(*) as value
		FROM users
		WHERE created_at >= NOW() - INTERVAL '1 hour' * $2
		GROUP BY date_trunc($1, created_at)
		ORDER BY date_trunc($1, created_at)
	`, bucket, fromHours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTimeseries(rows)
}

func (r *AdminRepository) GetBookingUsersTimeseries(ctx context.Context, fromHours int, bucket string) ([]dto.TimeseriesPoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT to_char(date_trunc($1, created_at), 'YYYY-MM-DD HH24:00') as label, COUNT(DISTINCT user_id) as value
		FROM bookings
		WHERE created_at >= NOW() - INTERVAL '1 hour' * $2
		GROUP BY date_trunc($1, created_at)
		ORDER BY date_trunc($1, created_at)
	`, bucket, fromHours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTimeseries(rows)
}

func (r *AdminRepository) GetRevenueTimeseries(ctx context.Context, fromHours int, bucket string) ([]dto.TimeseriesPoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT to_char(date_trunc($1, created_at), 'YYYY-MM-DD HH24:00') as label, COALESCE(SUM(amount), 0) as value
		FROM payments
		WHERE status = 'success' AND created_at >= NOW() - INTERVAL '1 hour' * $2
		GROUP BY date_trunc($1, created_at)
		ORDER BY date_trunc($1, created_at)
	`, bucket, fromHours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTimeseries(rows)
}

func scanTimeseries(rows pgx.Rows) ([]dto.TimeseriesPoint, error) {
	var points []dto.TimeseriesPoint
	for rows.Next() {
		var p dto.TimeseriesPoint
		if err := rows.Scan(&p.Label, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func (r *AdminRepository) ListAuditLogs(ctx context.Context, page, limit int) ([]*domain.AdminAuditLog, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM admin_audit_logs`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, admin_user_id, action, target_type, target_id, details, created_at
		FROM admin_audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*domain.AdminAuditLog
	for rows.Next() {
		l := &domain.AdminAuditLog{}
		if err := rows.Scan(&l.ID, &l.AdminUserID, &l.Action, &l.TargetType, &l.TargetID, &l.Details, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}


