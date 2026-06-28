package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/service"
)

type AdminHandler struct {
	service *service.AdminService
}

func NewAdminHandler(s *service.AdminService) *AdminHandler {
	return &AdminHandler{service: s}
}

func (h *AdminHandler) ListApplications(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	items, total, err := h.service.ListApplications(r.Context(), status, page, limit)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendPaginated(w, items, page, limit, int(total))
}

func (h *AdminHandler) GetApplication(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// Reuse list with empty status and single item; or add FindByID to service
	items, _, err := h.service.ListApplications(r.Context(), "", 1, 1)
	if err != nil {
		sendAppError(w, err)
		return
	}
	for _, item := range items {
		if item.ID == id {
			sendSuccess(w, item, 200)
			return
		}
	}
	sendError(w, "Application not found", "NOT_FOUND", 404)
}

func (h *AdminHandler) ReviewApplication(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	// Extract action from URL path: /api/admin/business-profiles/{id}/approve
	action := ""
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) > 0 {
		action = parts[len(parts)-1]
	}

	var req dto.ReviewBusinessProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request", "BAD_REQUEST", 400)
		return
	}
	req.Action = action

	if err := h.service.ReviewApplication(r.Context(), admin.ID, id, req); err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, map[string]string{"status": "reviewed"}, 200)
}

func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetPlatformStats(r.Context())
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, stats, 200)
}
