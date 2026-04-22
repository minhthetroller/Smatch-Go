package service

import (
	"context"
	"mime/multipart"
	"strings"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/repository"
)

type BusinessProfileService struct {
	bpRepo   *repository.BusinessProfileRepository
	userRepo *repository.UserRepository
	upload   *UploadService
}

func NewBusinessProfileService(bpRepo *repository.BusinessProfileRepository, userRepo *repository.UserRepository, upload *UploadService) *BusinessProfileService {
	return &BusinessProfileService{bpRepo: bpRepo, userRepo: userRepo, upload: upload}
}

func (s *BusinessProfileService) SubmitApplication(ctx context.Context, userID string, req dto.SubmitBusinessProfileRequest, files map[string]*multipart.FileHeader) (*dto.BusinessProfileResponse, error) {
	// Validate bank account holder name matches legal representative name
	if !strings.EqualFold(req.BankAccountHolderName, req.LegalRepresentativeName) {
		return nil, domain.BadRequest("Bank account holder name must match legal representative name")
	}

	existing, err := s.bpRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	bp := &domain.BusinessProfile{
		UserID:                  userID,
		LegalRepresentativeName: req.LegalRepresentativeName,
		PersonalIDNumber:        req.PersonalIDNumber,
		TaxIDNumber:             req.TaxIDNumber,
		BankAccountNumber:       req.BankAccountNumber,
		BankName:                req.BankName,
		BankBranch:              req.BankBranch,
		BankAccountHolderName:   req.BankAccountHolderName,
		OperationalSpecs:        req.OperationalSpecs,
		Status:                  domain.BusinessProfilePending,
	}

	// Upload files
	for field, header := range files {
		file, err := header.Open()
		if err != nil {
			return nil, domain.BadRequest("Failed to read uploaded file")
		}
		url, err := s.upload.UploadDocument(ctx, file, header, "business-profiles/"+userID)
		file.Close()
		if err != nil {
			return nil, err
		}
		switch field {
		case "personalIdFront":
			bp.PersonalIDFrontImageURL = &url
		case "personalIdBack":
			bp.PersonalIDBackImageURL = &url
		case "businessRegistrationCert":
			bp.BusinessRegistrationCertURL = &url
		case "sportsBusinessEligibilityCert":
			bp.SportsBusinessEligibilityCertURL = &url
		case "fireSafetyCert":
			bp.FireSafetyCertURL = &url
		case "proofOfAddress":
			bp.ProofOfAddressURL = &url
		}
	}

	if existing != nil {
		bp.ID = existing.ID
		if err := s.bpRepo.Update(ctx, bp); err != nil {
			return nil, err
		}
	} else {
		if err := s.bpRepo.Create(ctx, bp); err != nil {
			return nil, err
		}
	}

	return s.toResponse(bp), nil
}

func (s *BusinessProfileService) GetApplication(ctx context.Context, userID string) (*dto.BusinessProfileResponse, error) {
	bp, err := s.bpRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if bp == nil {
		return nil, domain.NotFound("Business profile not found")
	}
	return s.toResponse(bp), nil
}

func (s *BusinessProfileService) toResponse(bp *domain.BusinessProfile) *dto.BusinessProfileResponse {
	return &dto.BusinessProfileResponse{
		ID:                               bp.ID,
		UserID:                           bp.UserID,
		LegalRepresentativeName:          bp.LegalRepresentativeName,
		PersonalIDNumber:                 bp.PersonalIDNumber,
		PersonalIDFrontImageURL:          bp.PersonalIDFrontImageURL,
		PersonalIDBackImageURL:           bp.PersonalIDBackImageURL,
		BusinessRegistrationCertURL:      bp.BusinessRegistrationCertURL,
		SportsBusinessEligibilityCertURL: bp.SportsBusinessEligibilityCertURL,
		FireSafetyCertURL:                bp.FireSafetyCertURL,
		TaxIDNumber:                      bp.TaxIDNumber,
		ProofOfAddressURL:                bp.ProofOfAddressURL,
		BankAccountNumber:                bp.BankAccountNumber,
		BankName:                         bp.BankName,
		BankBranch:                       bp.BankBranch,
		BankAccountHolderName:            bp.BankAccountHolderName,
		OperationalSpecs:                 bp.OperationalSpecs,
		Status:                           string(bp.Status),
		AdminNotes:                       bp.AdminNotes,
		SubmittedAt:                      bp.SubmittedAt.Format("2006-01-02T15:04:05Z"),
		ReviewedAt:                       formatTimePtr(bp.ReviewedAt),
		ReviewedBy:                       bp.ReviewedBy,
	}
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02T15:04:05Z")
	return &s
}
