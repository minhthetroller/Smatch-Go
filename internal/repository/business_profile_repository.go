package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smatch/badminton-backend/internal/domain"
)

type BusinessProfileRepository struct {
	db *pgxpool.Pool
}

func NewBusinessProfileRepository(db *pgxpool.Pool) *BusinessProfileRepository {
	return &BusinessProfileRepository{db: db}
}

const businessProfileCols = `
	id, user_id,
	legal_representative_name, personal_id_number,
	personal_id_front_image_url, personal_id_back_image_url,
	business_registration_cert_url, sports_business_eligibility_cert_url,
	fire_safety_cert_url, tax_id_number,
	proof_of_address_url,
	bank_account_number, bank_name, bank_branch, bank_account_holder_name,
	operational_specs, status, admin_notes,
	submitted_at, reviewed_at, reviewed_by
`

func scanBusinessProfile(row pgx.Row) (*domain.BusinessProfile, error) {
	bp := &domain.BusinessProfile{}
	err := row.Scan(
		&bp.ID, &bp.UserID,
		&bp.LegalRepresentativeName, &bp.PersonalIDNumber,
		&bp.PersonalIDFrontImageURL, &bp.PersonalIDBackImageURL,
		&bp.BusinessRegistrationCertURL, &bp.SportsBusinessEligibilityCertURL,
		&bp.FireSafetyCertURL, &bp.TaxIDNumber,
		&bp.ProofOfAddressURL,
		&bp.BankAccountNumber, &bp.BankName, &bp.BankBranch, &bp.BankAccountHolderName,
		&bp.OperationalSpecs,
		&bp.Status, &bp.AdminNotes,
		&bp.SubmittedAt, &bp.ReviewedAt, &bp.ReviewedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return bp, err
}

func (r *BusinessProfileRepository) Create(ctx context.Context, bp *domain.BusinessProfile) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO business_profile_applications (
			user_id, legal_representative_name, personal_id_number,
			personal_id_front_image_url, personal_id_back_image_url,
			business_registration_cert_url, sports_business_eligibility_cert_url,
			fire_safety_cert_url, tax_id_number,
			proof_of_address_url,
			bank_account_number, bank_name, bank_branch, bank_account_holder_name,
			operational_specs, status, admin_notes
		) VALUES (
			$1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		)
		RETURNING id, submitted_at, reviewed_at, reviewed_by
	`, bp.UserID, bp.LegalRepresentativeName, bp.PersonalIDNumber,
		bp.PersonalIDFrontImageURL, bp.PersonalIDBackImageURL,
		bp.BusinessRegistrationCertURL, bp.SportsBusinessEligibilityCertURL,
		bp.FireSafetyCertURL, bp.TaxIDNumber,
		bp.ProofOfAddressURL,
		bp.BankAccountNumber, bp.BankName, bp.BankBranch, bp.BankAccountHolderName,
		bp.OperationalSpecs, bp.Status, bp.AdminNotes,
	).Scan(&bp.ID, &bp.SubmittedAt, &bp.ReviewedAt, &bp.ReviewedBy)
}

func (r *BusinessProfileRepository) FindByUserID(ctx context.Context, userID string) (*domain.BusinessProfile, error) {
	row := r.db.QueryRow(ctx, `SELECT `+businessProfileCols+` FROM business_profile_applications WHERE user_id = $1::uuid`, userID)
	return scanBusinessProfile(row)
}

func (r *BusinessProfileRepository) FindByID(ctx context.Context, id string) (*domain.BusinessProfile, error) {
	row := r.db.QueryRow(ctx, `SELECT `+businessProfileCols+` FROM business_profile_applications WHERE id = $1::uuid`, id)
	return scanBusinessProfile(row)
}

func (r *BusinessProfileRepository) Update(ctx context.Context, bp *domain.BusinessProfile) error {
	_, err := r.db.Exec(ctx, `
		UPDATE business_profile_applications
		SET legal_representative_name = $2,
		    personal_id_number = $3,
		    personal_id_front_image_url = $4,
		    personal_id_back_image_url = $5,
		    business_registration_cert_url = $6,
		    sports_business_eligibility_cert_url = $7,
		    fire_safety_cert_url = $8,
		    tax_id_number = $9,
		    proof_of_address_url = $10,
		    bank_account_number = $11,
		    bank_name = $12,
		    bank_branch = $13,
		    bank_account_holder_name = $14,
		    operational_specs = $15,
		    status = $16,
		    admin_notes = $17,
		    submitted_at = NOW()
		WHERE id = $1::uuid
	`, bp.ID, bp.LegalRepresentativeName, bp.PersonalIDNumber,
		bp.PersonalIDFrontImageURL, bp.PersonalIDBackImageURL,
		bp.BusinessRegistrationCertURL, bp.SportsBusinessEligibilityCertURL,
		bp.FireSafetyCertURL, bp.TaxIDNumber,
		bp.ProofOfAddressURL,
		bp.BankAccountNumber, bp.BankName, bp.BankBranch, bp.BankAccountHolderName,
		bp.OperationalSpecs, bp.Status, bp.AdminNotes,
	)
	return err
}

func (r *BusinessProfileRepository) UpdateStatus(ctx context.Context, id string, status domain.BusinessProfileStatus, adminNotes string, reviewedBy string) error {
	args := []interface{}{id, status, adminNotes}
	reviewedByArg := interface{}(nil)
	if reviewedBy != "" {
		reviewedByArg = reviewedBy
	}
	args = append(args, reviewedByArg)

	_, err := r.db.Exec(ctx, `
		UPDATE business_profile_applications
		SET status = $2,
		    admin_notes = $3,
		    reviewed_at = NOW(),
		    reviewed_by = $4::uuid
		WHERE id = $1::uuid
	`, args...)
	return err
}

func (r *BusinessProfileRepository) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM business_profile_applications WHERE user_id = $1::uuid`, userID)
	return err
}

func (r *BusinessProfileRepository) List(ctx context.Context, status string, page, limit int) ([]*domain.BusinessProfile, int, error) {
	var where string
	var args []interface{}
	if status != "" {
		where = "WHERE status = $1"
		args = append(args, status)
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM business_profile_applications %s`, where)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT `+businessProfileCols+`
		FROM business_profile_applications
		%s
		ORDER BY submitted_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)
	args = append(args, limit, (page-1)*limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var profiles []*domain.BusinessProfile
	for rows.Next() {
		bp := &domain.BusinessProfile{}
		if err := rows.Scan(
			&bp.ID, &bp.UserID,
			&bp.LegalRepresentativeName, &bp.PersonalIDNumber,
			&bp.PersonalIDFrontImageURL, &bp.PersonalIDBackImageURL,
			&bp.BusinessRegistrationCertURL, &bp.SportsBusinessEligibilityCertURL,
			&bp.FireSafetyCertURL, &bp.TaxIDNumber,
			&bp.ProofOfAddressURL,
			&bp.BankAccountNumber, &bp.BankName, &bp.BankBranch, &bp.BankAccountHolderName,
			&bp.OperationalSpecs,
			&bp.Status, &bp.AdminNotes,
			&bp.SubmittedAt, &bp.ReviewedAt, &bp.ReviewedBy,
		); err != nil {
			return nil, 0, err
		}
		profiles = append(profiles, bp)
	}
	return profiles, total, rows.Err()
}
