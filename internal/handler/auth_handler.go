package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Verify POST /api/auth/verify - Verify Firebase ID token and upsert user.
func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var req dto.VerifyTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.Verify(r.Context(), &req)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// Anonymous POST /api/auth/anonymous - Create or fetch anonymous user.
func (h *AuthHandler) Anonymous(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateAnonymousRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.Anonymous(r.Context(), &req)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 201)
}

// Me GET /api/auth/me - Return current authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	sendSuccess(w, h.svc.GetProfile(r.Context(), user), 200)
}

// UpdateMe PUT /api/auth/me - Update profile.
func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req dto.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.UpdateProfile(r.Context(), user.ID, &req)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// AddFCMToken POST /api/auth/fcm-token - Add FCM token.
func (h *AuthHandler) AddFCMToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req dto.AddFCMTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	if err := h.svc.AddFCMToken(r.Context(), user.ID, req.Token); err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, map[string]bool{"success": true}, 200)
}

// RemoveFCMToken DELETE /api/auth/fcm-token/:token - Remove FCM token.
func (h *AuthHandler) RemoveFCMToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	token := chi.URLParam(r, "token")
	if err := h.svc.RemoveFCMToken(r.Context(), user.ID, token); err != nil {
		sendAppError(w, err)
		return
	}
	w.WriteHeader(204)
}

// DeleteMe DELETE /api/auth/me - Delete account.
func (h *AuthHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if err := h.svc.DeleteAccount(r.Context(), user.ID); err != nil {
		sendAppError(w, err)
		return
	}
	w.WriteHeader(204)
}

// MyBookings GET /api/auth/me/bookings - Get user bookings.
func (h *AuthHandler) MyBookings(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	items, total, err := h.svc.MyBookings(r.Context(), user.ID, page, limit)
	if err != nil {
		sendAppError(w, err)
		return
	}
	if items == nil {
		items = []dto.BookingHistoryItemResponse{}
	}
	sendPaginated(w, items, page, limit, total)
}

// CheckUsername GET /api/auth/username/check - Check username availability.
func (h *AuthHandler) CheckUsername(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		sendError(w, "username is required", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.CheckUsername(r.Context(), username)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// LookupUsername GET /api/auth/username/lookup - Lookup email by username.
func (h *AuthHandler) LookupUsername(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		sendError(w, "username is required", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.LookupUsername(r.Context(), username)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// UploadPhoto POST /api/auth/me/photo - Upload profile photo
func (h *AuthHandler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		sendError(w, "File too large or invalid form data", "BAD_REQUEST", 400)
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		sendError(w, "No image file provided", "BAD_REQUEST", 400)
		return
	}
	defer file.Close()

	user := middleware.UserFromContext(r.Context())
	resp, err := h.svc.UploadProfilePhoto(r.Context(), user, file, header)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, dto.ProfilePhotoUploadResponse{User: resp.User}, 201)
}

// Convert POST /api/auth/convert - Convert anonymous user to registered
func (h *AuthHandler) Convert(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req dto.ConvertAnonymousRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.Convert(r.Context(), user.ID, &req)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, dto.AuthResponse{User: resp.User, IsNewUser: resp.IsNewUser}, 200)
}

// LinkBookings POST /api/auth/link-bookings - Link guest bookings to authenticated user by phone
func (h *AuthHandler) LinkBookings(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if err := h.svc.LinkBookings(r.Context(), user); err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, map[string]bool{"success": true}, 200)
}
