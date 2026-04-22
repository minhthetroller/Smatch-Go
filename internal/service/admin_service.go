package service

import (
	"context"
	"encoding/json"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/repository"
)

type AdminService struct {
	bpRepo   *repository.BusinessProfileRepository
	userRepo *repository.UserRepository
	coRepo   *repository.CourtOwnerRepository
	adminRepo *repository.AdminRepository
}

func NewAdminService(bpRepo *repository.BusinessProfileRepository, userRepo *repository.UserRepository, coRepo *repository.CourtOwnerRepository, adminRepo *repository.AdminRepository) *AdminService {
	return &AdminService{bpRepo: bpRepo, userRepo: userRepo, coRepo: coRepo, adminRepo: adminRepo}
}

func (s *AdminService) ListApplications(ctx context.Context, status string, page, limit int) ([]dto.BusinessProfileResponse, int, error) {
	profiles, total, err := s.bpRepo.List(ctx, status, page, limit)
	if err != nil {
		return nil, 0, err
	}

	var items []dto.BusinessProfileResponse
	for _, p := range profiles {
		items = append(items, dto.BusinessProfileResponse{
			ID:                               p.ID,
			UserID:                           p.UserID,
			LegalRepresentativeName:          p.LegalRepresentativeName,
			PersonalIDNumber:                 p.PersonalIDNumber,
			PersonalIDFrontImageURL:          p.PersonalIDFrontImageURL,
			PersonalIDBackImageURL:           p.PersonalIDBackImageURL,
			BusinessRegistrationCertURL:      p.BusinessRegistrationCertURL,
			SportsBusinessEligibilityCertURL: p.SportsBusinessEligibilityCertURL,
			FireSafetyCertURL:                p.FireSafetyCertURL,
			TaxIDNumber:                      p.TaxIDNumber,
			ProofOfAddressURL:                p.ProofOfAddressURL,
			BankAccountNumber:                p.BankAccountNumber,
			BankName:                         p.BankName,
			BankBranch:                       p.BankBranch,
			BankAccountHolderName:            p.BankAccountHolderName,
			OperationalSpecs:                 p.OperationalSpecs,
			Status:                           string(p.Status),
			AdminNotes:                       p.AdminNotes,
			SubmittedAt:                      p.SubmittedAt.Format("2006-01-02T15:04:05Z"),
			ReviewedAt:                       formatTimePtr(p.ReviewedAt),
			ReviewedBy:                       p.ReviewedBy,
		})
	}

	return items, total, nil
}

func (s *AdminService) ReviewApplication(ctx context.Context, adminID, appID string, req dto.ReviewBusinessProfileRequest) error {
	bp, err := s.bpRepo.FindByID(ctx, appID)
	if err != nil {
		return err
	}
	if bp == nil {
		return domain.NotFound("Application not found")
	}

	var status domain.BusinessProfileStatus
	switch req.Action {
	case "approve":
		status = domain.BusinessProfileApproved
	case "reject":
		status = domain.BusinessProfileRejected
	case "request_resubmit":
		status = domain.BusinessProfileResubmitReq
	default:
		return domain.BadRequest("Invalid action")
	}

	if err := s.bpRepo.UpdateStatus(ctx, appID, status, req.AdminNotes, adminID); err != nil {
		return err
	}

	// If approved, add court_owner role and auto-create court
	if status == domain.BusinessProfileApproved {
		roles, err := s.userRepo.GetRoles(ctx, bp.UserID)
		if err != nil {
			return err
		}
		if !contains(roles, "court_owner") {
			roles = append(roles, "court_owner")
			if err := s.userRepo.UpdateRoles(ctx, bp.UserID, roles); err != nil {
				return err
			}
		}

		var specs domain.OperationalSpecs
		if err := json.Unmarshal(bp.OperationalSpecs, &specs); err == nil {
			_, _ = s.coRepo.CreateCourtFromSpecs(ctx, bp.UserID, specs)
		}
	}

	details, _ := json.Marshal(map[string]string{
		"action":      req.Action,
		"admin_notes": req.AdminNotes,
	})
	_ = s.adminRepo.CreateAuditLog(ctx, &domain.AdminAuditLog{
		AdminUserID: adminID,
		Action:      "review_business_profile:" + req.Action,
		TargetType:  strPtr("business_profile"),
		TargetID:    &appID,
		Details:     details,
	})

	return nil
}

func (s *AdminService) GetPlatformStats(ctx context.Context) (*dto.AdminStatsResponse, error) {
	stats, err := s.adminRepo.GetPlatformStats(ctx)
	if err != nil {
		return nil, err
	}
	return &dto.AdminStatsResponse{
		TotalActiveUsers:    stats.TotalActiveUsers,
		TotalCourtOwners:    stats.TotalCourtOwners,
		TotalCourts:         stats.TotalCourts,
		TotalRevenue:        stats.TotalRevenue,
		PendingApplications: stats.PendingApplications,
		RecentSignups:       stats.RecentSignups,
	}, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func strPtr(s string) *string { return &s }
