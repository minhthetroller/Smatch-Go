package service

import (
	"context"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/repository"
)

type CourtOwnerService struct {
	coRepo   *repository.CourtOwnerRepository
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

func (s *CourtOwnerService) GetCourt(ctx context.Context, ownerID, courtID string) (*dto.CourtOwnerCourtDetailResponse, error) {
	court, err := s.coRepo.GetCourtByOwner(ctx, ownerID, courtID)
	if err != nil {
		return nil, err
	}
	if court == nil {
		return nil, domain.NotFound("Court not found")
	}

	subCourts, err := s.coRepo.ListSubCourtsByCourt(ctx, courtID)
	if err != nil {
		return nil, err
	}

	pricingRules, err := s.coRepo.ListPricingRulesByCourt(ctx, courtID)
	if err != nil {
		return nil, err
	}

	closures, err := s.coRepo.ListUpcomingClosuresByCourt(ctx, courtID, time.Now())
	if err != nil {
		return nil, err
	}

	resp := &dto.CourtOwnerCourtDetailResponse{
		ID:              court.ID,
		Name:            court.Name,
		Description:     court.Description,
		PhoneNumbers:    court.PhoneNumbers,
		AddressStreet:   court.AddressStreet,
		AddressWard:     court.AddressWard,
		AddressDistrict: court.AddressDistrict,
		AddressCity:     court.AddressCity,
		Details:         court.Details,
		OpeningHours:    court.OpeningHours,
		Lat:             court.Lat,
		Lng:             court.Lng,
		IsActive:        true, // default; could be derived from closures
		CreatedAt:       court.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       court.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	for _, sc := range subCourts {
		resp.SubCourts = append(resp.SubCourts, dto.SubCourtResponse{
			ID:          sc.ID,
			Name:        sc.Name,
			Description: sc.Description,
			IsActive:    sc.IsActive,
		})
	}

	for _, pr := range pricingRules {
		resp.PricingRules = append(resp.PricingRules, dto.PricingRuleResponse{
			ID:           pr.ID,
			Name:         pr.Name,
			DayType:      pr.DayType,
			StartTime:    pr.StartTime,
			EndTime:      pr.EndTime,
			PricePerHour: pr.PricePerHour,
			IsActive:     pr.IsActive,
		})
	}

	for _, cl := range closures {
		resp.UpcomingClosures = append(resp.UpcomingClosures, dto.SubCourtClosureResponse{
			ID:         cl.ID,
			SubCourtID: cl.SubCourtID,
			Date:       cl.Date.Format("2006-01-02"),
			StartTime:  cl.StartTime,
			EndTime:    cl.EndTime,
			Reason:     cl.Reason,
		})
	}

	return resp, nil
}

func (s *CourtOwnerService) GetCourtStats(ctx context.Context, ownerID, courtID string, period string, granularity string) (*dto.CourtStatsDetailResponse, error) {
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

	resp := &dto.CourtStatsDetailResponse{
		Summary: dto.CourtStatsResponse{
			TotalBookings:    stats.TotalBookings,
			TotalRevenue:     stats.TotalRevenue,
			OccupancyRate:    stats.OccupancyRate,
			CancellationRate: stats.CancellationRate,
		},
	}

	if granularity == "daily" {
		daily, err := s.coRepo.GetCourtStatsDaily(ctx, courtID, from, to)
		if err != nil {
			return nil, err
		}
		for _, d := range daily {
			resp.DailyBreakdown = append(resp.DailyBreakdown, dto.CourtStatsDailyItem{
				Date:          d.Date.Format("2006-01-02"),
				Bookings:      d.Bookings,
				Revenue:       d.Revenue,
				Cancellations: d.Cancellations,
			})
		}
	}

	return resp, nil
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
