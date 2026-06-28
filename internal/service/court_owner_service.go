package service

import (
	"context"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/repository"
)

type CourtOwnerService struct {
	coRepo    *repository.CourtOwnerRepository
	courtRepo *repository.CourtRepository
}

func NewCourtOwnerService(coRepo *repository.CourtOwnerRepository, courtRepo *repository.CourtRepository) *CourtOwnerService {
	return &CourtOwnerService{coRepo: coRepo, courtRepo: courtRepo}
}

func (s *CourtOwnerService) ListMyCourts(ctx context.Context, ownerID string) ([]dto.CourtOwnerCourtResponse, error) {
	courts, err := s.coRepo.ListCourtsByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	var resp []dto.CourtOwnerCourtResponse
	for _, c := range courts {
		resp = append(resp, dto.CourtOwnerCourtResponse{
			ID:              c.ID,
			Name:            c.Name,
			Description:     c.Description,
			PhoneNumbers:    c.PhoneNumbers,
			AddressStreet:   c.AddressStreet,
			AddressWard:     c.AddressWard,
			AddressDistrict: c.AddressDistrict,
			AddressCity:     c.AddressCity,
			Details:         c.Details,
			OpeningHours:    c.OpeningHours,
			Lat:             c.Lat,
			Lng:             c.Lng,
			CreatedAt:       c.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:       c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	return resp, nil
}

func (s *CourtOwnerService) GetCourtStats(ctx context.Context, ownerID, courtID string, period string) (*dto.CourtStatsResponse, error) {
	// Verify ownership
	court, err := s.courtRepo.FindByID(ctx, courtID)
	if err != nil {
		return nil, err
	}
	if court == nil {
		return nil, domain.NotFound("Court not found")
	}
	if court.OwnerUserID == nil || *court.OwnerUserID != ownerID {
		return nil, domain.Forbidden("Court not owned by you")
	}

	from, to := parsePeriod(period)
	stats, err := s.coRepo.GetCourtStats(ctx, courtID, from, to)
	if err != nil {
		return nil, err
	}

	return &dto.CourtStatsResponse{
		TotalBookings:    stats.TotalBookings,
		TotalRevenue:     stats.TotalRevenue,
		OccupancyRate:    stats.OccupancyRate,
		CancellationRate: stats.CancellationRate,
	}, nil
}

func parsePeriod(period string) (time.Time, time.Time) {
	now := time.Now()
	switch period {
	case "week":
		return now.AddDate(0, 0, -7), now
	case "month":
		return now.AddDate(0, -1, 0), now
	default:
		return now.AddDate(0, 0, -1), now
	}
}

func (s *CourtOwnerService) CloseSubCourt(ctx context.Context, ownerID, courtID, subCourtID string, req dto.CloseSubCourtRequest) error {
	return s.coRepo.CloseSubCourt(ctx, subCourtID, req.Date, req.StartTime, req.EndTime, req.Reason)
}

func (s *CourtOwnerService) OpenSubCourt(ctx context.Context, ownerID, courtID, subCourtID, date string) error {
	return s.coRepo.OpenSubCourt(ctx, subCourtID, date)
}

func (s *CourtOwnerService) CloseCourt(ctx context.Context, ownerID, courtID string, req dto.CloseCourtRequest) error {
	return s.coRepo.CloseAllSubCourts(ctx, courtID, req.Date, req.StartTime, req.EndTime, req.Reason)
}

func (s *CourtOwnerService) OpenCourt(ctx context.Context, ownerID, courtID, date string) error {
	return s.coRepo.OpenAllSubCourts(ctx, courtID, date)
}
