package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
)

type courtStore interface {
	FindAll(ctx context.Context) ([]*domain.Court, error)
	FindByID(ctx context.Context, id string) (*domain.Court, error)
	FindNearby(ctx context.Context, lat, lng, radiusMeters float64) ([]*domain.Court, []float64, error)
	Create(ctx context.Context, c *domain.Court) (*domain.Court, error)
	Update(ctx context.Context, id string, fields map[string]interface{}) (*domain.Court, error)
	Delete(ctx context.Context, id string) error
}

type CourtService struct {
	repo courtStore
}

func NewCourtService(repo courtStore) *CourtService {
	return &CourtService{repo: repo}
}

const (
	courtNearbyMinRadiusKm = 5.0
	courtNearbyMaxRadiusKm = 50.0
	courtNearbyDefaultKm   = courtNearbyMinRadiusKm
)

// List returns all courts.
func (s *CourtService) List(ctx context.Context) ([]dto.CourtResponse, error) {
	courts, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("court list: %w", err)
	}
	resp := make([]dto.CourtResponse, len(courts))
	for i, c := range courts {
		resp[i] = mapCourtToDTO(c)
	}
	return resp, nil
}

// NearbyQuery holds parsed parameters for a nearby search.
type NearbyQuery struct {
	Lat      float64
	Lng      float64
	RadiusKm float64
}

// ParseNearbyRadius validates the radius query string. Accepted format is "<n>km" in [5, 50].
// Returns the default radius when raw is empty.
func ParseNearbyRadius(raw string) (float64, error) {
	if raw == "" {
		return courtNearbyDefaultKm, nil
	}
	if !strings.HasSuffix(raw, "km") {
		return 0, domain.BadRequest(fmt.Sprintf(
			"radius must include unit suffix, e.g. radius=%.0fkm (accepted range: %.0f–%.0fkm)",
			courtNearbyMinRadiusKm, courtNearbyMinRadiusKm, courtNearbyMaxRadiusKm))
	}
	parsed, err := strconv.ParseFloat(strings.TrimSuffix(raw, "km"), 64)
	if err != nil {
		return 0, domain.BadRequest("radius value must be a number in km, e.g. radius=10km")
	}
	if parsed < courtNearbyMinRadiusKm || parsed > courtNearbyMaxRadiusKm {
		return 0, domain.BadRequest(fmt.Sprintf("radius must be between %.0fkm and %.0fkm",
			courtNearbyMinRadiusKm, courtNearbyMaxRadiusKm))
	}
	return parsed, nil
}

// Nearby returns courts within radius, with distance annotations.
func (s *CourtService) Nearby(ctx context.Context, q NearbyQuery) ([]dto.CourtResponse, error) {
	courts, distances, err := s.repo.FindNearby(ctx, q.Lat, q.Lng, q.RadiusKm*1000)
	if err != nil {
		return nil, fmt.Errorf("court nearby: %w", err)
	}
	resp := make([]dto.CourtResponse, len(courts))
	for i, c := range courts {
		cr := mapCourtToDTO(c)
		if i < len(distances) {
			distMeters := distances[i]
			distKm := distMeters / 1000
			cr.Distance = &distMeters
			cr.DistanceKm = &distKm
		}
		resp[i] = cr
	}
	return resp, nil
}

// Get returns a single court by ID.
func (s *CourtService) Get(ctx context.Context, id string) (*dto.CourtResponse, error) {
	court, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("court get: %w", err)
	}
	if court == nil {
		return nil, domain.NotFound("Court not found")
	}
	resp := mapCourtToDTO(court)
	return &resp, nil
}

// Create creates a court.
func (s *CourtService) Create(ctx context.Context, req *dto.CreateCourtRequest) (*dto.CourtResponse, error) {
	c := &domain.Court{
		Name:            req.Name,
		Description:     req.Description,
		PhoneNumbers:    req.PhoneNumbers,
		AddressStreet:   req.AddressStreet,
		AddressWard:     req.AddressWard,
		AddressDistrict: req.AddressDistrict,
		AddressCity:     req.AddressCity,
		Details:         req.Details,
		OpeningHours:    req.OpeningHours,
		Lat:             req.Lat,
		Lng:             req.Lng,
	}
	created, err := s.repo.Create(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("court create: %w", err)
	}
	resp := mapCourtToDTO(created)
	return &resp, nil
}

// Update applies partial updates to a court.
func (s *CourtService) Update(ctx context.Context, id string, req *dto.UpdateCourtRequest) (*dto.CourtResponse, error) {
	fields := map[string]interface{}{}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.PhoneNumbers != nil {
		fields["phone_numbers"] = req.PhoneNumbers
	}
	if req.Details != nil {
		fields["details"] = req.Details
	}
	if req.OpeningHours != nil {
		fields["opening_hours"] = req.OpeningHours
	}
	updated, err := s.repo.Update(ctx, id, fields)
	if err != nil {
		return nil, fmt.Errorf("court update: %w", err)
	}
	resp := mapCourtToDTO(updated)
	return &resp, nil
}

// Delete removes a court by ID.
func (s *CourtService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("court delete: %w", err)
	}
	return nil
}

func mapCourtToDTO(c *domain.Court) dto.CourtResponse {
	resp := dto.CourtResponse{
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
		CreatedAt:       c.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:       c.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	if resp.PhoneNumbers == nil {
		resp.PhoneNumbers = []string{}
	}
	return resp
}
