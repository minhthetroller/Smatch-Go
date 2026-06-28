package service

import (
	"context"
	"fmt"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
)

type searchRepository interface {
	SearchCourts(ctx context.Context, query string, page, limit int) ([]*domain.Court, int, error)
	GetAllCourtNames(ctx context.Context) ([]struct{ ID, Name, District string }, error)
}

type searchRedis interface {
	SearchAutocomplete(ctx context.Context, prefix string, limit int) ([]AutocompleteSuggestion, error)
	TrackSearch(ctx context.Context, query string) error
	GetPopularSearches(ctx context.Context, limit int) ([]string, error)
	ClearAutocomplete(ctx context.Context) error
	AddToAutocomplete(ctx context.Context, courtID, courtName string, terms []string, score float64) error
	GetAutocompleteCount(ctx context.Context) (int64, error)
}

type SearchService struct {
	redis     searchRedis
	searchRep searchRepository
}

func NewSearchService(redis *RedisService, repo searchRepository) *SearchService {
	var r searchRedis
	if redis != nil {
		r = redis
	}
	return &SearchService{redis: r, searchRep: repo}
}

// Autocomplete returns suggestions for a prefix.
func (s *SearchService) Autocomplete(ctx context.Context, prefix string) (dto.AutocompleteResponse, error) {
	if s.redis == nil {
		return dto.AutocompleteResponse{Suggestions: []dto.AutocompleteSuggestion{}}, nil
	}
	suggestions, err := s.redis.SearchAutocomplete(ctx, prefix, 10)
	if err != nil {
		return dto.AutocompleteResponse{}, fmt.Errorf("search autocomplete: %w", err)
	}
	dtos := make([]dto.AutocompleteSuggestion, 0, len(suggestions))
	for _, sug := range suggestions {
		dtos = append(dtos, dto.AutocompleteSuggestion{ID: sug.ID, Text: sug.Text, Score: sug.Score})
	}
	return dto.AutocompleteResponse{Suggestions: dtos}, nil
}

// SearchCourtsResult holds search results and pagination metadata.
type SearchCourtsResult struct {
	Courts []dto.CourtResponse
	Total  int
}

// SearchCourts performs a court search and tracks the query.
func (s *SearchService) SearchCourts(ctx context.Context, query string, page, limit int) (SearchCourtsResult, error) {
	courts, total, err := s.searchRep.SearchCourts(ctx, query, page, limit)
	if err != nil {
		return SearchCourtsResult{}, fmt.Errorf("search courts: %w", err)
	}
	resp := make([]dto.CourtResponse, len(courts))
	for i, c := range courts {
		resp[i] = mapCourtToDTO(c)
	}
	if s.redis != nil {
		_ = s.redis.TrackSearch(ctx, query) // best-effort
	}
	return SearchCourtsResult{Courts: resp, Total: total}, nil
}

// Popular returns the most popular recent search queries.
func (s *SearchService) Popular(ctx context.Context) (dto.PopularSearchesResponse, error) {
	var queries []string
	if s.redis != nil {
		q, err := s.redis.GetPopularSearches(ctx, 10)
		if err != nil {
			return dto.PopularSearchesResponse{}, fmt.Errorf("popular searches: %w", err)
		}
		queries = q
	}
	if queries == nil {
		queries = []string{}
	}
	return dto.PopularSearchesResponse{Queries: queries}, nil
}

// Reindex rebuilds the autocomplete index from court names.
func (s *SearchService) Reindex(ctx context.Context) (int, error) {
	courts, err := s.searchRep.GetAllCourtNames(ctx)
	if err != nil {
		return 0, fmt.Errorf("reindex: %w", err)
	}
	if s.redis != nil {
		_ = s.redis.ClearAutocomplete(ctx) // best-effort
		for _, c := range courts {
			terms := []string{c.Name}
			if c.District != "" {
				terms = append(terms, c.District)
			}
			_ = s.redis.AddToAutocomplete(ctx, c.ID, c.Name, terms, 0) // best-effort
		}
	}
	return len(courts), nil
}

// Stats returns autocomplete index stats.
func (s *SearchService) Stats(ctx context.Context) (int64, error) {
	if s.redis == nil {
		return 0, nil
	}
	count, err := s.redis.GetAutocompleteCount(ctx)
	if err != nil {
		return 0, fmt.Errorf("autocomplete stats: %w", err)
	}
	return count, nil
}
