package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/repository"
	"github.com/smatch/badminton-backend/internal/service"
	ws "github.com/smatch/badminton-backend/internal/websocket"
)

type MatchHandler struct {
	matchRepo    *repository.MatchRepository
	redisService *service.RedisService
	hub          *ws.Hub
	images       ImageURLResolver
}

func NewMatchHandler(mr *repository.MatchRepository, rs *service.RedisService, hub *ws.Hub, images ImageURLResolver) *MatchHandler {
	return &MatchHandler{matchRepo: mr, redisService: rs, hub: hub, images: images}
}

// GET /api/matches
func (h *MatchHandler) GetAllMatches(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	filter := repository.MatchFilter{
		Page:  page,
		Limit: limit,
	}
	if v := q.Get("courtId"); v != "" {
		filter.CourtID = &v
	}
	if v := q.Get("skillLevel"); v != "" {
		filter.SkillLevel = &v
	}
	if v := q.Get("playerFormat"); v != "" {
		filter.PlayerFormat = &v
	}
	if v := q.Get("status"); v != "" {
		filter.Status = &v
	}
	if v := q.Get("date"); v != "" {
		filter.Date = &v
	}
	if v := q.Get("dateFrom"); v != "" {
		filter.DateFrom = &v
	}
	if v := q.Get("dateTo"); v != "" {
		filter.DateTo = &v
	}
	filter.IncludeExpired = q.Get("includeExpired") == "true"

	matches, total, err := h.matchRepo.FindAll(r.Context(), filter)
	if err != nil {
		sendError(w, "Failed to get matches", "INTERNAL_ERROR", 500)
		return
	}

	resp := make([]dto.MatchResponse, len(matches))
	for i, m := range matches {
		resp[i] = h.mapMatchRowToDTO(m)
	}
	sendPaginated(w, resp, page, limit, total)
}

// GET /api/matches/hosted
func (h *MatchHandler) GetHostedMatches(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	includeExpired := r.URL.Query().Get("includeExpired") == "true"

	matches, err := h.matchRepo.FindByHostUserID(r.Context(), user.ID, includeExpired)
	if err != nil {
		sendError(w, "Failed to get hosted matches", "INTERNAL_ERROR", 500)
		return
	}

	resp := make([]dto.MatchResponse, len(matches))
	for i, m := range matches {
		resp[i] = h.mapMatchRowToDTO(m)
	}
	sendSuccess(w, resp, 200)
}

// GET /api/matches/joined
func (h *MatchHandler) GetJoinedMatches(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	includeExpired := r.URL.Query().Get("includeExpired") == "true"

	matches, err := h.matchRepo.FindByPlayerUserID(r.Context(), user.ID, includeExpired)
	if err != nil {
		sendError(w, "Failed to get joined matches", "INTERNAL_ERROR", 500)
		return
	}

	resp := make([]dto.MatchResponse, len(matches))
	for i, m := range matches {
		resp[i] = h.mapMatchRowToDTO(m)
	}
	sendSuccess(w, resp, 200)
}

// GET /api/matches/:id
func (h *MatchHandler) GetMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	match, err := h.matchRepo.FindByID(r.Context(), id)
	if err != nil || match == nil {
		sendError(w, "Match not found", "NOT_FOUND", 404)
		return
	}

	players, err := h.matchRepo.GetPlayersByMatchID(r.Context(), id, nil)
	if err != nil {
		sendError(w, "Failed to get match players", "INTERNAL_ERROR", 500)
		return
	}

	resp := dto.MatchWithPlayersResponse{
		MatchResponse: h.mapMatchRowToDTO(match),
		Players:       h.mapPlayersToDTO(players),
	}

	// If authenticated, attach current user status.
	user := middleware.UserFromContext(r.Context())
	if user != nil {
		player, _ := h.matchRepo.FindPlayer(r.Context(), id, user.ID)
		if player != nil {
			resp.CurrentUserStatus = mapCurrentUserStatus(player)
		}
	}

	sendSuccess(w, resp, 200)
}

// POST /api/matches
func (h *MatchHandler) CreateMatch(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	var req dto.CreateMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}

	if req.SlotsNeeded < 1 || req.SlotsNeeded > 20 {
		sendError(w, "slotsNeeded must be between 1 and 20", "BAD_REQUEST", 400)
		return
	}

	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		sendError(w, "Invalid date format, use YYYY-MM-DD", "BAD_REQUEST", 400)
		return
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

	created, err := h.matchRepo.CreateAndFetch(r.Context(), m, user.ID)
	if err != nil {
		sendError(w, "Failed to create match", "INTERNAL_ERROR", 500)
		return
	}

	sendSuccess(w, h.mapMatchRowToDTO(created), 201)
}

// PUT /api/matches/:id
func (h *MatchHandler) UpdateMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	match, err := h.matchRepo.FindByID(r.Context(), id)
	if err != nil || match == nil {
		sendError(w, "Match not found", "NOT_FOUND", 404)
		return
	}
	if match.HostUserID != user.ID {
		sendError(w, "Only the host can update this match", "FORBIDDEN", 403)
		return
	}

	var req dto.UpdateMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
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

	updated, err := h.matchRepo.Update(r.Context(), id, fields)
	if err != nil {
		sendError(w, "Failed to update match", "INTERNAL_ERROR", 500)
		return
	}
	sendSuccess(w, h.mapMatchRowToDTO(updated), 200)
}

// DELETE /api/matches/:id - sets status=CANCELLED
func (h *MatchHandler) CancelMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	match, err := h.matchRepo.FindByID(r.Context(), id)
	if err != nil || match == nil {
		sendError(w, "Match not found", "NOT_FOUND", 404)
		return
	}
	if match.HostUserID != user.ID {
		sendError(w, "Only the host can cancel this match", "FORBIDDEN", 403)
		return
	}
	if match.Status == domain.MatchStatusCancelled {
		sendError(w, "Match is already cancelled", "BAD_REQUEST", 400)
		return
	}

	if err := h.matchRepo.UpdateStatus(r.Context(), id, domain.MatchStatusCancelled); err != nil {
		sendError(w, "Failed to cancel match", "INTERNAL_ERROR", 500)
		return
	}

	// Notify subscribers.
	h.hub.NotifyMatchSubscribers(id, ws.MatchStatusChange{
		Type:    "match_status_change",
		MatchID: id,
		Status:  string(domain.MatchStatusCancelled),
		Message: "Match has been cancelled by the host",
	})

	w.WriteHeader(204)
}

// POST /api/matches/:id/join
func (h *MatchHandler) JoinMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	var req dto.JoinMatchRequest
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck

	match, err := h.matchRepo.FindByID(r.Context(), id)
	if err != nil || match == nil {
		sendError(w, "Match not found", "NOT_FOUND", 404)
		return
	}

	// Must be OPEN to join.
	if match.Status != domain.MatchStatusOpen {
		sendError(w, "Match is not open for joining", "MATCH_NOT_OPEN", 400)
		return
	}

	// Host cannot join own match.
	if match.HostUserID == user.ID {
		sendError(w, "Host cannot join their own match", "HOST_CANNOT_JOIN", 400)
		return
	}

	// Check if already a player.
	existing, err := h.matchRepo.FindPlayer(r.Context(), id, user.ID)
	if err != nil {
		sendError(w, "Failed to check player status", "INTERNAL_ERROR", 500)
		return
	}
	if existing != nil && (existing.Status == domain.MatchPlayerStatusAccepted ||
		existing.Status == domain.MatchPlayerStatusPending ||
		existing.Status == domain.MatchPlayerStatusPendingPayment) {
		sendError(w, "You have already joined or requested to join this match", "ALREADY_JOINED", 409)
		return
	}

	// Acquire distributed lock to prevent race conditions.
	locked, err := h.redisService.AcquireMatchPlayerLock(r.Context(), id, user.ID)
	if err != nil || !locked {
		sendError(w, "Could not process join request at this time, please try again", "LOCK_FAILED", 409)
		return
	}
	defer h.redisService.ReleaseMatchPlayerLock(r.Context(), id, user.ID)

	var playerStatus domain.MatchPlayerStatus
	var position *int

	if !match.IsPrivate {
		// Public match: check slots and auto-accept.
		acceptedCount, err := h.matchRepo.CountAcceptedPlayers(r.Context(), id)
		if err != nil {
			sendError(w, "Failed to check slots", "INTERNAL_ERROR", 500)
			return
		}
		if acceptedCount >= match.SlotsNeeded {
			sendError(w, "Match is full", "MATCH_FULL", 400)
			return
		}
		playerStatus = domain.MatchPlayerStatusAccepted
		pos, err := h.matchRepo.GetNextPosition(r.Context(), id)
		if err != nil {
			sendError(w, "Failed to assign position", "INTERNAL_ERROR", 500)
			return
		}
		position = &pos
	} else {
		// Private match: set to PENDING for host approval.
		playerStatus = domain.MatchPlayerStatusPending
	}

	player, err := h.matchRepo.AddPlayer(r.Context(), id, user.ID, req.Message, playerStatus)
	if err != nil {
		sendError(w, "Failed to join match", "INTERNAL_ERROR", 500)
		return
	}

	// If accepted and position assigned, update position.
	if position != nil && player != nil {
		player, err = h.matchRepo.UpdatePlayerStatus(r.Context(), player.ID, playerStatus, position)
		if err != nil {
			sendError(w, "Failed to assign position", "INTERNAL_ERROR", 500)
			return
		}
	}

	// For public matches, check if now full.
	if !match.IsPrivate && playerStatus == domain.MatchPlayerStatusAccepted {
		acceptedCount, _ := h.matchRepo.CountAcceptedPlayers(r.Context(), id)
		if acceptedCount >= match.SlotsNeeded {
			h.matchRepo.UpdateStatus(r.Context(), id, domain.MatchStatusFull) //nolint:errcheck
			h.hub.NotifyMatchSubscribers(id, ws.MatchStatusChange{
				Type:    "match_status_change",
				MatchID: id,
				Status:  string(domain.MatchStatusFull),
				Message: "Match is now full",
			})
		}
	}

	// Notify host of join request for private matches.
	if match.IsPrivate {
		userName := buildDisplayName(player.UserFirstName, player.UserLastName, player.UserUsername)
		queuePos, _ := h.redisService.GetMatchJoinQueuePosition(r.Context(), id, user.ID)
		h.hub.NotifyUser(match.HostUserID, ws.MatchJoinRequest{
			Type:         "match_join_request",
			MatchID:      id,
			PlayerID:     player.ID,
			UserID:       user.ID,
			UserName:     userName,
			UserPhotoURL: player.UserPhotoURL,
			Message:      req.Message,
			QueuePos:     int(queuePos),
			Timestamp:    player.RequestedAt.Format("2006-01-02T15:04:05.000Z"),
		})
	}

	sendSuccess(w, h.mapPlayerRowToDTO(player), 201)
}

// DELETE /api/matches/:id/leave
func (h *MatchHandler) LeaveMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	player, err := h.matchRepo.FindPlayer(r.Context(), id, user.ID)
	if err != nil || player == nil {
		sendError(w, "You are not a player in this match", "NOT_FOUND", 404)
		return
	}

	if player.Status == domain.MatchPlayerStatusLeft {
		sendError(w, "You have already left this match", "ALREADY_LEFT", 400)
		return
	}

	_, err = h.matchRepo.UpdatePlayerStatus(r.Context(), player.ID, domain.MatchPlayerStatusLeft, nil)
	if err != nil {
		sendError(w, "Failed to leave match", "INTERNAL_ERROR", 500)
		return
	}

	// If match was FULL, re-open it.
	match, _ := h.matchRepo.FindByID(r.Context(), id)
	if match != nil && match.Status == domain.MatchStatusFull {
		h.matchRepo.UpdateStatus(r.Context(), id, domain.MatchStatusOpen) //nolint:errcheck
		h.hub.NotifyMatchSubscribers(id, ws.MatchStatusChange{
			Type:    "match_status_change",
			MatchID: id,
			Status:  string(domain.MatchStatusOpen),
			Message: "A slot has opened up",
		})
	}

	// Notify subscribers of player leaving.
	acceptedCount, _ := h.matchRepo.CountAcceptedPlayers(r.Context(), id)
	slotsRemaining := 0
	if match != nil {
		slotsRemaining = match.SlotsNeeded - acceptedCount
	}
	userName := buildDisplayName(player.UserFirstName, player.UserLastName, player.UserUsername)
	h.hub.NotifyMatchSubscribers(id, ws.MatchPlayerLeft{
		Type:           "match_player_left",
		MatchID:        id,
		UserID:         user.ID,
		UserName:       userName,
		SlotsRemaining: slotsRemaining,
	})

	w.WriteHeader(204)
}

// GET /api/matches/:id/requests
func (h *MatchHandler) GetJoinRequests(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	match, err := h.matchRepo.FindByID(r.Context(), id)
	if err != nil || match == nil {
		sendError(w, "Match not found", "NOT_FOUND", 404)
		return
	}
	if match.HostUserID != user.ID {
		sendError(w, "Only the host can view join requests", "FORBIDDEN", 403)
		return
	}

	pendingStatus := string(domain.MatchPlayerStatusPending)
	players, err := h.matchRepo.GetPlayersByMatchID(r.Context(), id, &pendingStatus)
	if err != nil {
		sendError(w, "Failed to get join requests", "INTERNAL_ERROR", 500)
		return
	}

	resp := h.mapPlayersToDTO(players)
	if resp == nil {
		resp = []dto.MatchPlayerResponse{}
	}
	sendSuccess(w, resp, 200)
}

// PUT /api/matches/:id/requests/:playerId/respond
func (h *MatchHandler) RespondToJoinRequest(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "id")
	playerID := chi.URLParam(r, "playerId")
	user := middleware.UserFromContext(r.Context())

	match, err := h.matchRepo.FindByID(r.Context(), matchID)
	if err != nil || match == nil {
		sendError(w, "Match not found", "NOT_FOUND", 404)
		return
	}
	if match.HostUserID != user.ID {
		sendError(w, "Only the host can respond to join requests", "FORBIDDEN", 403)
		return
	}

	var req dto.RespondToJoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	if req.Status != "ACCEPTED" && req.Status != "REJECTED" {
		sendError(w, "Status must be ACCEPTED or REJECTED", "BAD_REQUEST", 400)
		return
	}

	// Find the player by playerID (user ID, not match_player row ID).
	player, err := h.matchRepo.FindPlayer(r.Context(), matchID, playerID)
	if err != nil || player == nil {
		// Try by match_player ID.
		player, err = h.matchRepo.FindPlayerByID(r.Context(), playerID)
		if err != nil || player == nil {
			sendError(w, "Player not found", "NOT_FOUND", 404)
			return
		}
	}

	if player.Status != domain.MatchPlayerStatusPending {
		sendError(w, "This request is no longer pending", "INVALID_STATUS", 400)
		return
	}

	var newStatus domain.MatchPlayerStatus
	var position *int

	if req.Status == "ACCEPTED" {
		// Check slots.
		acceptedCount, err := h.matchRepo.CountAcceptedPlayers(r.Context(), matchID)
		if err != nil {
			sendError(w, "Failed to check slots", "INTERNAL_ERROR", 500)
			return
		}
		if acceptedCount >= match.SlotsNeeded {
			sendError(w, "Match is full, cannot accept more players", "MATCH_FULL", 400)
			return
		}

		// If price > 0, move to PENDING_PAYMENT; otherwise ACCEPTED.
		if match.Price > 0 {
			newStatus = domain.MatchPlayerStatusPendingPayment
		} else {
			newStatus = domain.MatchPlayerStatusAccepted
			pos, err := h.matchRepo.GetNextPosition(r.Context(), matchID)
			if err != nil {
				sendError(w, "Failed to assign position", "INTERNAL_ERROR", 500)
				return
			}
			position = &pos
		}
	} else {
		newStatus = domain.MatchPlayerStatusRejected
	}

	updated, err := h.matchRepo.UpdatePlayerStatus(r.Context(), player.ID, newStatus, position)
	if err != nil {
		sendError(w, "Failed to update player status", "INTERNAL_ERROR", 500)
		return
	}

	// Check if match is now full.
	if newStatus == domain.MatchPlayerStatusAccepted {
		acceptedCount, _ := h.matchRepo.CountAcceptedPlayers(r.Context(), matchID)
		if acceptedCount >= match.SlotsNeeded {
			h.matchRepo.UpdateStatus(r.Context(), matchID, domain.MatchStatusFull) //nolint:errcheck
			h.hub.NotifyMatchSubscribers(matchID, ws.MatchStatusChange{
				Type:    "match_status_change",
				MatchID: matchID,
				Status:  string(domain.MatchStatusFull),
				Message: "Match is now full",
			})
		}
	}

	// Notify the requesting player.
	msg := "Your join request has been accepted"
	if newStatus == domain.MatchPlayerStatusPendingPayment {
		msg = "Your join request has been accepted. Please complete payment."
	} else if newStatus == domain.MatchPlayerStatusRejected {
		msg = "Your join request has been rejected"
	}
	h.hub.NotifyUser(player.UserID, ws.MatchRequestResponse{
		Type:     "match_request_response",
		MatchID:  matchID,
		PlayerID: player.ID,
		Status:   string(newStatus),
		Position: position,
		Message:  msg,
	})

	sendSuccess(w, h.mapPlayerRowToDTO(updated), 200)
}

// ==================== Mapping helpers ====================

func (h *MatchHandler) mapMatchRowToDTO(m *repository.MatchRow) dto.MatchResponse {
	resp := dto.MatchResponse{
		ID:            m.ID,
		CourtID:       m.CourtID,
		CourtName:     m.CourtName,
		CourtAddress:  m.CourtAddressStr,
		HostUserID:    m.HostUserID,
		HostName:      buildDisplayName(m.HostFirstName, m.HostLastName, m.HostUsername),
		Title:         m.Title,
		Description:   m.Description,
		Images:        h.resolveMatchImages(m.Images),
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

func (h *MatchHandler) resolveMatchImages(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	resolved := make([]string, len(keys))
	for i, k := range keys {
		resolved[i] = h.images.Match(k)
	}
	return resolved
}

func (h *MatchHandler) mapPlayersToDTO(players []*repository.MatchPlayerRow) []dto.MatchPlayerResponse {
	resp := make([]dto.MatchPlayerResponse, len(players))
	for i, p := range players {
		resp[i] = h.mapPlayerRowToDTO(p)
	}
	return resp
}

func (h *MatchHandler) mapPlayerRowToDTO(p *repository.MatchPlayerRow) dto.MatchPlayerResponse {
	r := dto.MatchPlayerResponse{
		ID:           p.ID,
		MatchID:      p.MatchID,
		UserID:       p.UserID,
		UserName:     buildDisplayName(p.UserFirstName, p.UserLastName, p.UserUsername),
		UserPhotoURL: h.resolvePlayerPhotoURL(p.UserPhotoURL),
		Status:       string(p.Status),
		Message:      p.Message,
		Position:     p.Position,
		RequestedAt:  p.RequestedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	if p.RespondedAt != nil {
		s := p.RespondedAt.Format("2006-01-02T15:04:05.000Z")
		r.RespondedAt = &s
	}
	return r
}

func (h *MatchHandler) resolvePlayerPhotoURL(photoURL *string) *string {
	if photoURL == nil || *photoURL == "" {
		return photoURL
	}
	resolved := h.images.Profile(*photoURL)
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
		s := p.RespondedAt.Format("2006-01-02T15:04:05.000Z")
		resp.RespondedAt = &s
	}
	return resp
}

func buildDisplayName(firstName, lastName, username *string) string {
	if firstName != nil && lastName != nil {
		return *firstName + " " + *lastName
	}
	if firstName != nil {
		return *firstName
	}
	if username != nil {
		return *username
	}
	return "Anonymous"
}
