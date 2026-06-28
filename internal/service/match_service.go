package service

import (
	"context"
	"fmt"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/imageurl"
	"github.com/smatch/badminton-backend/internal/repository"
	ws "github.com/smatch/badminton-backend/internal/websocket"
)

type matchRepository interface {
	FindAll(ctx context.Context, f repository.MatchFilter) ([]*repository.MatchRow, int, error)
	FindByID(ctx context.Context, id string) (*repository.MatchRow, error)
	FindByHostUserID(ctx context.Context, userID string, includeExpired bool) ([]*repository.MatchRow, error)
	FindByPlayerUserID(ctx context.Context, userID string, includeExpired bool) ([]*repository.MatchRow, error)
	CreateAndFetch(ctx context.Context, m *domain.Match, hostUserID string) (*repository.MatchRow, error)
	UpdateStatus(ctx context.Context, id string, status domain.MatchStatus) error
	Update(ctx context.Context, id string, fields map[string]interface{}) (*repository.MatchRow, error)
	FindPlayer(ctx context.Context, matchID, userID string) (*repository.MatchPlayerRow, error)
	FindPlayerByID(ctx context.Context, playerID string) (*repository.MatchPlayerRow, error)
	GetPlayersByMatchID(ctx context.Context, matchID string, status *string) ([]*repository.MatchPlayerRow, error)
	CountAcceptedPlayers(ctx context.Context, matchID string) (int, error)
	GetNextPosition(ctx context.Context, matchID string) (int, error)
	AddPlayer(ctx context.Context, matchID, userID string, message *string, status domain.MatchPlayerStatus) (*repository.MatchPlayerRow, error)
	UpdatePlayerStatus(ctx context.Context, playerID string, status domain.MatchPlayerStatus, position *int) (*repository.MatchPlayerRow, error)
}

type matchLocker interface {
	AcquireMatchPlayerLock(ctx context.Context, matchID, userID string) (bool, error)
	ReleaseMatchPlayerLock(ctx context.Context, matchID, userID string)
	GetMatchJoinQueuePosition(ctx context.Context, matchID, userID string) (int64, error)
}

// MatchService contains all match business logic: listing, lifecycle, joining
// (with public/private branching, slot counting, positions, Redis locking) and
// WebSocket notifications. It returns DTOs with resolved image URLs.
type MatchService struct {
	repo   matchRepository
	redis  matchLocker
	hub    *ws.Hub
	images imageurl.Resolver
}

func NewMatchService(repo *repository.MatchRepository, redis *RedisService, hub *ws.Hub, images imageurl.Resolver) *MatchService {
	var locker matchLocker
	if redis != nil {
		locker = redis
	}
	return &MatchService{repo: repo, redis: locker, hub: hub, images: images}
}

// matchError replicates the previous handler error codes/messages/status verbatim.
func matchError(code, message string, status int) *domain.AppError {
	var sentinel error
	switch status {
	case 404:
		sentinel = domain.ErrNotFound
	case 403:
		sentinel = domain.ErrForbidden
	case 409:
		sentinel = domain.ErrConflict
	case 400:
		sentinel = domain.ErrBadRequest
	}
	return &domain.AppError{Code: code, Message: message, Status: status, Err: sentinel}
}

// GetAllMatches lists matches with filters and pagination.
func (s *MatchService) GetAllMatches(ctx context.Context, filter repository.MatchFilter) ([]dto.MatchResponse, int, error) {
	matches, total, err := s.repo.FindAll(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("match list: %w", err)
	}
	resp := make([]dto.MatchResponse, len(matches))
	for i, m := range matches {
		resp[i] = s.mapMatchRowToDTO(m)
	}
	return resp, total, nil
}

// GetHostedMatches returns matches hosted by the user.
func (s *MatchService) GetHostedMatches(ctx context.Context, userID string, includeExpired bool) ([]dto.MatchResponse, error) {
	matches, err := s.repo.FindByHostUserID(ctx, userID, includeExpired)
	if err != nil {
		return nil, fmt.Errorf("hosted matches: %w", err)
	}
	resp := make([]dto.MatchResponse, len(matches))
	for i, m := range matches {
		resp[i] = s.mapMatchRowToDTO(m)
	}
	return resp, nil
}

// GetJoinedMatches returns matches the user has joined.
func (s *MatchService) GetJoinedMatches(ctx context.Context, userID string, includeExpired bool) ([]dto.MatchResponse, error) {
	matches, err := s.repo.FindByPlayerUserID(ctx, userID, includeExpired)
	if err != nil {
		return nil, fmt.Errorf("joined matches: %w", err)
	}
	resp := make([]dto.MatchResponse, len(matches))
	for i, m := range matches {
		resp[i] = s.mapMatchRowToDTO(m)
	}
	return resp, nil
}

// GetMatchResult holds a single match with players and the requesting user's status.
type GetMatchResult struct {
	dto.MatchWithPlayersResponse
}

// GetMatch returns a match with all players and the requesting user's status (if any).
func (s *MatchService) GetMatch(ctx context.Context, id string, currentUserID *string) (*GetMatchResult, error) {
	match, err := s.repo.FindByID(ctx, id)
	if err != nil || match == nil {
		return nil, matchError("NOT_FOUND", "Match not found", 404)
	}
	players, err := s.repo.GetPlayersByMatchID(ctx, id, nil)
	if err != nil {
		return nil, fmt.Errorf("get match players: %w", err)
	}
	resp := &GetMatchResult{MatchWithPlayersResponse: dto.MatchWithPlayersResponse{
		MatchResponse: s.mapMatchRowToDTO(match),
		Players:       s.mapPlayersToDTO(players),
	}}
	if currentUserID != nil {
		player, _ := s.repo.FindPlayer(ctx, id, *currentUserID)
		if player != nil {
			resp.CurrentUserStatus = mapCurrentUserStatus(player)
		}
	}
	return resp, nil
}

// CreateMatch validates the request and creates a new match hosted by the user.
func (s *MatchService) CreateMatch(ctx context.Context, req *dto.CreateMatchRequest, userID string) (dto.MatchResponse, error) {
	if req.SlotsNeeded < 1 || req.SlotsNeeded > 20 {
		return dto.MatchResponse{}, matchError("BAD_REQUEST", "slotsNeeded must be between 1 and 20", 400)
	}
	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return dto.MatchResponse{}, matchError("BAD_REQUEST", "Invalid date format, use YYYY-MM-DD", 400)
	}
	m := &domain.Match{
		CourtID:      req.CourtID,
		Title:        req.Title,
		Description:  req.Description,
		Images:       req.Images,
		SkillLevel:   domain.SkillLevel(req.SkillLevel),
		ShuttleType:  domain.ShuttleType(req.ShuttleType),
		PlayerFormat: domain.PlayerFormat(req.PlayerFormat),
		Date:         parsedDate,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		IsPrivate:    req.IsPrivate,
		Price:        req.Price,
		SlotsNeeded:  req.SlotsNeeded,
	}
	created, err := s.repo.CreateAndFetch(ctx, m, userID)
	if err != nil {
		return dto.MatchResponse{}, fmt.Errorf("create match: %w", err)
	}
	return s.mapMatchRowToDTO(created), nil
}

// UpdateMatch applies partial updates. Only the host may update.
func (s *MatchService) UpdateMatch(ctx context.Context, id string, req *dto.UpdateMatchRequest, userID string) (dto.MatchResponse, error) {
	match, err := s.repo.FindByID(ctx, id)
	if err != nil || match == nil {
		return dto.MatchResponse{}, matchError("NOT_FOUND", "Match not found", 404)
	}
	if match.HostUserID != userID {
		return dto.MatchResponse{}, matchError("FORBIDDEN", "Only the host can update this match", 403)
	}
	fields := map[string]interface{}{}
	if req.Title != nil {
		fields["title"] = *req.Title
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.Images != nil {
		fields["images"] = req.Images
	}
	if req.SkillLevel != nil {
		fields["skill_level"] = *req.SkillLevel
	}
	if req.ShuttleType != nil {
		fields["shuttle_type"] = *req.ShuttleType
	}
	if req.PlayerFormat != nil {
		fields["player_format"] = *req.PlayerFormat
	}
	if req.Date != nil {
		fields["date"] = *req.Date
	}
	if req.StartTime != nil {
		fields["start_time"] = *req.StartTime
	}
	if req.EndTime != nil {
		fields["end_time"] = *req.EndTime
	}
	if req.IsPrivate != nil {
		fields["is_private"] = *req.IsPrivate
	}
	if req.Price != nil {
		fields["price"] = *req.Price
	}
	if req.SlotsNeeded != nil {
		fields["slots_needed"] = *req.SlotsNeeded
	}
	if req.Status != nil {
		fields["status"] = *req.Status
	}
	updated, err := s.repo.Update(ctx, id, fields)
	if err != nil {
		return dto.MatchResponse{}, fmt.Errorf("update match: %w", err)
	}
	return s.mapMatchRowToDTO(updated), nil
}

// CancelMatch cancels a match. Only the host may cancel.
func (s *MatchService) CancelMatch(ctx context.Context, id, userID string) error {
	match, err := s.repo.FindByID(ctx, id)
	if err != nil || match == nil {
		return matchError("NOT_FOUND", "Match not found", 404)
	}
	if match.HostUserID != userID {
		return matchError("FORBIDDEN", "Only the host can cancel this match", 403)
	}
	if match.Status == domain.MatchStatusCancelled {
		return matchError("BAD_REQUEST", "Match is already cancelled", 400)
	}
	if err := s.repo.UpdateStatus(ctx, id, domain.MatchStatusCancelled); err != nil {
		return fmt.Errorf("cancel match: %w", err)
	}
	s.notifyMatchSubscribers(id, ws.MatchStatusChange{
		Type:    "match_status_change",
		MatchID: id,
		Status:  string(domain.MatchStatusCancelled),
		Message: "Match has been cancelled by the host",
	})
	return nil
}

// JoinMatch attempts to join a match. Public matches auto-accept; private
// matches go to PENDING for host approval. Acquires a Redis lock to prevent
// race conditions on slot counting.
func (s *MatchService) JoinMatch(ctx context.Context, id, userID string, message *string) (dto.MatchPlayerResponse, error) {
	match, err := s.repo.FindByID(ctx, id)
	if err != nil || match == nil {
		return dto.MatchPlayerResponse{}, matchError("NOT_FOUND", "Match not found", 404)
	}
	if match.Status != domain.MatchStatusOpen {
		return dto.MatchPlayerResponse{}, matchError("MATCH_NOT_OPEN", "Match is not open for joining", 400)
	}
	if match.HostUserID == userID {
		return dto.MatchPlayerResponse{}, matchError("HOST_CANNOT_JOIN", "Host cannot join their own match", 400)
	}
	existing, err := s.repo.FindPlayer(ctx, id, userID)
	if err != nil {
		return dto.MatchPlayerResponse{}, fmt.Errorf("check player: %w", err)
	}
	if existing != nil && (existing.Status == domain.MatchPlayerStatusAccepted ||
		existing.Status == domain.MatchPlayerStatusPending ||
		existing.Status == domain.MatchPlayerStatusPendingPayment) {
		return dto.MatchPlayerResponse{}, matchError("ALREADY_JOINED", "You have already joined or requested to join this match", 409)
	}

	// Acquire distributed lock to prevent race conditions.
	locked, err := s.acquireMatchPlayerLock(ctx, id, userID)
	if err != nil || !locked {
		return dto.MatchPlayerResponse{}, matchError("LOCK_FAILED", "Could not process join request at this time, please try again", 409)
	}
	defer s.releaseMatchPlayerLock(ctx, id, userID)

	var playerStatus domain.MatchPlayerStatus
	var position *int

	if !match.IsPrivate {
		acceptedCount, err := s.repo.CountAcceptedPlayers(ctx, id)
		if err != nil {
			return dto.MatchPlayerResponse{}, fmt.Errorf("count accepted: %w", err)
		}
		if acceptedCount >= match.SlotsNeeded {
			return dto.MatchPlayerResponse{}, matchError("MATCH_FULL", "Match is full", 400)
		}
		playerStatus = domain.MatchPlayerStatusAccepted
		pos, err := s.repo.GetNextPosition(ctx, id)
		if err != nil {
			return dto.MatchPlayerResponse{}, fmt.Errorf("next position: %w", err)
		}
		position = &pos
	} else {
		playerStatus = domain.MatchPlayerStatusPending
	}

	player, err := s.repo.AddPlayer(ctx, id, userID, message, playerStatus)
	if err != nil {
		return dto.MatchPlayerResponse{}, fmt.Errorf("add player: %w", err)
	}

	if position != nil && player != nil {
		player, err = s.repo.UpdatePlayerStatus(ctx, player.ID, playerStatus, position)
		if err != nil {
			return dto.MatchPlayerResponse{}, fmt.Errorf("assign position: %w", err)
		}
	}

	// For public matches, check if now full.
	if !match.IsPrivate && playerStatus == domain.MatchPlayerStatusAccepted {
		acceptedCount, _ := s.repo.CountAcceptedPlayers(ctx, id)
		if acceptedCount >= match.SlotsNeeded {
			_ = s.repo.UpdateStatus(ctx, id, domain.MatchStatusFull)
			s.notifyMatchSubscribers(id, ws.MatchStatusChange{
				Type:    "match_status_change",
				MatchID: id,
				Status:  string(domain.MatchStatusFull),
				Message: "Match is now full",
			})
		}
	}

	// Notify host of join request for private matches.
	if match.IsPrivate && player != nil {
		userName := buildDisplayName(player.UserFirstName, player.UserLastName, player.UserUsername)
		queuePos, _ := s.getMatchJoinQueuePosition(ctx, id, userID)
		s.notifyUser(match.HostUserID, ws.MatchJoinRequest{
			Type:         "match_join_request",
			MatchID:      id,
			PlayerID:     player.ID,
			UserID:       userID,
			UserName:     userName,
			UserPhotoURL: player.UserPhotoURL,
			Message:      message,
			QueuePos:     int(queuePos),
			Timestamp:    player.RequestedAt.Format("2006-01-02T15:04:05.000Z"),
		})
	}

	if player == nil {
		return dto.MatchPlayerResponse{}, nil
	}
	return s.mapPlayerRowToDTO(player), nil
}

// LeaveMatch marks the requesting user as left and re-opens the match if needed.
func (s *MatchService) LeaveMatch(ctx context.Context, id, userID string) error {
	player, err := s.repo.FindPlayer(ctx, id, userID)
	if err != nil || player == nil {
		return matchError("NOT_FOUND", "You are not a player in this match", 404)
	}
	if player.Status == domain.MatchPlayerStatusLeft {
		return matchError("ALREADY_LEFT", "You have already left this match", 400)
	}
	if _, err := s.repo.UpdatePlayerStatus(ctx, player.ID, domain.MatchPlayerStatusLeft, nil); err != nil {
		return fmt.Errorf("leave match: %w", err)
	}

	match, _ := s.repo.FindByID(ctx, id)
	if match != nil && match.Status == domain.MatchStatusFull {
		_ = s.repo.UpdateStatus(ctx, id, domain.MatchStatusOpen)
		s.notifyMatchSubscribers(id, ws.MatchStatusChange{
			Type:    "match_status_change",
			MatchID: id,
			Status:  string(domain.MatchStatusOpen),
			Message: "A slot has opened up",
		})
	}

	acceptedCount, _ := s.repo.CountAcceptedPlayers(ctx, id)
	slotsRemaining := 0
	if match != nil {
		slotsRemaining = match.SlotsNeeded - acceptedCount
	}
	userName := buildDisplayName(player.UserFirstName, player.UserLastName, player.UserUsername)
	s.notifyMatchSubscribers(id, ws.MatchPlayerLeft{
		Type:           "match_player_left",
		MatchID:        id,
		UserID:         userID,
		UserName:       userName,
		SlotsRemaining: slotsRemaining,
	})
	return nil
}

// GetJoinRequests returns pending join requests for a match (host only).
func (s *MatchService) GetJoinRequests(ctx context.Context, id, userID string) ([]dto.MatchPlayerResponse, error) {
	match, err := s.repo.FindByID(ctx, id)
	if err != nil || match == nil {
		return nil, matchError("NOT_FOUND", "Match not found", 404)
	}
	if match.HostUserID != userID {
		return nil, matchError("FORBIDDEN", "Only the host can view join requests", 403)
	}
	pendingStatus := string(domain.MatchPlayerStatusPending)
	players, err := s.repo.GetPlayersByMatchID(ctx, id, &pendingStatus)
	if err != nil {
		return nil, fmt.Errorf("get join requests: %w", err)
	}
	resp := s.mapPlayersToDTO(players)
	if resp == nil {
		resp = []dto.MatchPlayerResponse{}
	}
	return resp, nil
}

// RespondToJoinRequest accepts or rejects a pending join request (host only).
func (s *MatchService) RespondToJoinRequest(ctx context.Context, matchID, playerID, userID, status string) (dto.MatchPlayerResponse, error) {
	match, err := s.repo.FindByID(ctx, matchID)
	if err != nil || match == nil {
		return dto.MatchPlayerResponse{}, matchError("NOT_FOUND", "Match not found", 404)
	}
	if match.HostUserID != userID {
		return dto.MatchPlayerResponse{}, matchError("FORBIDDEN", "Only the host can respond to join requests", 403)
	}
	if status != "ACCEPTED" && status != "REJECTED" {
		return dto.MatchPlayerResponse{}, matchError("BAD_REQUEST", "Status must be ACCEPTED or REJECTED", 400)
	}

	player, err := s.repo.FindPlayer(ctx, matchID, playerID)
	if err != nil || player == nil {
		player, err = s.repo.FindPlayerByID(ctx, playerID)
		if err != nil || player == nil {
			return dto.MatchPlayerResponse{}, matchError("NOT_FOUND", "Player not found", 404)
		}
	}
	if player.Status != domain.MatchPlayerStatusPending {
		return dto.MatchPlayerResponse{}, matchError("INVALID_STATUS", "This request is no longer pending", 400)
	}

	var newStatus domain.MatchPlayerStatus
	var position *int

	if status == "ACCEPTED" {
		acceptedCount, err := s.repo.CountAcceptedPlayers(ctx, matchID)
		if err != nil {
			return dto.MatchPlayerResponse{}, fmt.Errorf("count accepted: %w", err)
		}
		if acceptedCount >= match.SlotsNeeded {
			return dto.MatchPlayerResponse{}, matchError("MATCH_FULL", "Match is full, cannot accept more players", 400)
		}
		if match.Price > 0 {
			newStatus = domain.MatchPlayerStatusPendingPayment
		} else {
			newStatus = domain.MatchPlayerStatusAccepted
			pos, err := s.repo.GetNextPosition(ctx, matchID)
			if err != nil {
				return dto.MatchPlayerResponse{}, fmt.Errorf("next position: %w", err)
			}
			position = &pos
		}
	} else {
		newStatus = domain.MatchPlayerStatusRejected
	}

	updated, err := s.repo.UpdatePlayerStatus(ctx, player.ID, newStatus, position)
	if err != nil {
		return dto.MatchPlayerResponse{}, fmt.Errorf("update player status: %w", err)
	}

	// Check if match is now full.
	if newStatus == domain.MatchPlayerStatusAccepted {
		acceptedCount, _ := s.repo.CountAcceptedPlayers(ctx, matchID)
		if acceptedCount >= match.SlotsNeeded {
			_ = s.repo.UpdateStatus(ctx, matchID, domain.MatchStatusFull)
			s.notifyMatchSubscribers(matchID, ws.MatchStatusChange{
				Type:    "match_status_change",
				MatchID: matchID,
				Status:  string(domain.MatchStatusFull),
				Message: "Match is now full",
			})
		}
	}

	msg := "Your join request has been accepted"
	if newStatus == domain.MatchPlayerStatusPendingPayment {
		msg = "Your join request has been accepted. Please complete payment."
	} else if newStatus == domain.MatchPlayerStatusRejected {
		msg = "Your join request has been rejected"
	}
	s.notifyUser(player.UserID, ws.MatchRequestResponse{
		Type:     "match_request_response",
		MatchID:  matchID,
		PlayerID: player.ID,
		Status:   string(newStatus),
		Position: position,
		Message:  msg,
	})

	return s.mapPlayerRowToDTO(updated), nil
}

// ==================== Redis helpers ====================

func (s *MatchService) acquireMatchPlayerLock(ctx context.Context, matchID, userID string) (bool, error) {
	if s.redis == nil {
		return true, nil
	}
	return s.redis.AcquireMatchPlayerLock(ctx, matchID, userID)
}

func (s *MatchService) releaseMatchPlayerLock(ctx context.Context, matchID, userID string) {
	if s.redis == nil {
		return
	}
	s.redis.ReleaseMatchPlayerLock(ctx, matchID, userID)
}

func (s *MatchService) getMatchJoinQueuePosition(ctx context.Context, matchID, userID string) (int64, error) {
	if s.redis == nil {
		return 0, nil
	}
	return s.redis.GetMatchJoinQueuePosition(ctx, matchID, userID)
}

// ==================== Hub helpers ====================

func (s *MatchService) notifyMatchSubscribers(matchID string, n ws.MatchNotification) {
	if s.hub == nil {
		return
	}
	s.notifyMatchSubscribers(matchID, n)
}

func (s *MatchService) notifyUser(userID string, n ws.MatchNotification) {
	if s.hub == nil {
		return
	}
	s.notifyUser(userID, n)
}

// ==================== Mapping helpers ====================

func (s *MatchService) mapMatchRowToDTO(m *repository.MatchRow) dto.MatchResponse {
	resp := dto.MatchResponse{
		ID:            m.ID,
		CourtID:       m.CourtID,
		CourtName:     m.CourtName,
		CourtAddress:  m.CourtAddressStr,
		HostUserID:    m.HostUserID,
		HostName:      buildDisplayName(m.HostFirstName, m.HostLastName, m.HostUsername),
		Title:         m.Title,
		Description:   m.Description,
		Images:        s.resolveMatchImages(m.Images),
		SkillLevel:    string(m.SkillLevel),
		ShuttleType:   string(m.ShuttleType),
		PlayerFormat:  string(m.PlayerFormat),
		Date:          m.Date.Format("2006-01-02"),
		StartTime:     m.StartTime,
		EndTime:       m.EndTime,
		IsPrivate:     m.IsPrivate,
		Price:         m.Price,
		SlotsNeeded:   m.SlotsNeeded,
		SlotsAccepted: m.SlotsAccepted,
		Status:        string(m.Status),
		CreatedAt:     m.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:     m.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	if resp.Images == nil {
		resp.Images = []string{}
	}
	return resp
}

func (s *MatchService) resolveMatchImages(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	resolved := make([]string, len(keys))
	for i, k := range keys {
		resolved[i] = s.images.Match(k)
	}
	return resolved
}

func (s *MatchService) mapPlayersToDTO(players []*repository.MatchPlayerRow) []dto.MatchPlayerResponse {
	resp := make([]dto.MatchPlayerResponse, len(players))
	for i, p := range players {
		resp[i] = s.mapPlayerRowToDTO(p)
	}
	return resp
}

func (s *MatchService) mapPlayerRowToDTO(p *repository.MatchPlayerRow) dto.MatchPlayerResponse {
	r := dto.MatchPlayerResponse{
		ID:           p.ID,
		MatchID:      p.MatchID,
		UserID:       p.UserID,
		UserName:     buildDisplayName(p.UserFirstName, p.UserLastName, p.UserUsername),
		UserPhotoURL: s.resolvePlayerPhotoURL(p.UserPhotoURL),
		Status:       string(p.Status),
		Message:      p.Message,
		Position:     p.Position,
		RequestedAt:  p.RequestedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	if p.RespondedAt != nil {
		t := p.RespondedAt.Format("2006-01-02T15:04:05.000Z")
		r.RespondedAt = &t
	}
	return r
}

func (s *MatchService) resolvePlayerPhotoURL(photoURL *string) *string {
	if photoURL == nil || *photoURL == "" {
		return photoURL
	}
	resolved := s.images.Profile(*photoURL)
	return &resolved
}

func mapCurrentUserStatus(p *repository.MatchPlayerRow) *dto.CurrentUserStatusResponse {
	resp := &dto.CurrentUserStatusResponse{
		ID:          p.ID,
		Status:      string(p.Status),
		Position:    p.Position,
		RequestedAt: p.RequestedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	if p.RespondedAt != nil {
		t := p.RespondedAt.Format("2006-01-02T15:04:05.000Z")
		resp.RespondedAt = &t
	}
	return resp
}
