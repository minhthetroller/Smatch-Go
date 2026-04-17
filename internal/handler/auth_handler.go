package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/repository"
	firebasepkg "github.com/smatch/badminton-backend/platform/firebase"
)

type AuthHandler struct {
	firebase  *firebasepkg.Client
	userRepo  *repository.UserRepository
	availRepo *repository.AvailabilityRepository
}

func NewAuthHandler(fb *firebasepkg.Client, ur *repository.UserRepository, ar *repository.AvailabilityRepository) *AuthHandler {
	return &AuthHandler{firebase: fb, userRepo: ur, availRepo: ar}
}

// POST /api/auth/verify - Verify Firebase ID token and upsert user.
func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var req dto.VerifyTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}

	decoded, err := h.firebase.VerifyIDToken(r.Context(), req.IDToken)
	if err != nil {
		sendError(w, "Invalid or expired token", "INVALID_TOKEN", 401)
		return
	}

	// Build user from Firebase token.
	u := &domain.User{
		FirebaseUID: decoded.UID,
		Provider:    decoded.Firebase.SignInProvider,
	}
	if email, ok := decoded.Claims["email"].(string); ok {
		u.Email = &email
	}
	if name, ok := decoded.Claims["name"].(string); ok {
		parts := splitName(name)
		u.FirstName = &parts[0]
		if len(parts) > 1 {
			u.LastName = &parts[1]
		}
	}
	if photo, ok := decoded.Claims["picture"].(string); ok {
		u.PhotoURL = &photo
	}

	created, isNew, err := h.userRepo.Upsert(r.Context(), u)
	if err != nil {
		sendError(w, "Failed to create user", "INTERNAL_ERROR", 500)
		return
	}

	sendSuccess(w, dto.AuthResponse{
		User:      mapUserToDTO(created),
		IsNewUser: isNew,
	}, 200)
}

// POST /api/auth/anonymous - Create or fetch anonymous user.
func (h *AuthHandler) Anonymous(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateAnonymousRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}

	u := &domain.User{
		FirebaseUID: req.FirebaseUID,
		Provider:    "anonymous",
		IsAnonymous: true,
	}

	created, isNew, err := h.userRepo.Upsert(r.Context(), u)
	if err != nil {
		sendError(w, "Failed to create user", "INTERNAL_ERROR", 500)
		return
	}

	sendSuccess(w, dto.AuthResponse{User: mapUserToDTO(created), IsNewUser: isNew}, 201)
}

// GET /api/auth/me - Return current authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	sendSuccess(w, mapUserToDTO(user), 200)
}

// PUT /api/auth/me - Update profile.
func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req dto.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}

	fields := map[string]interface{}{}
	if req.Username != nil {
		fields["username"] = *req.Username
	}
	if req.FirstName != nil {
		fields["first_name"] = *req.FirstName
	}
	if req.LastName != nil {
		fields["last_name"] = *req.LastName
	}
	if req.Gender != nil {
		fields["gender"] = *req.Gender
	}
	if req.PhoneNumber != nil {
		fields["phone_number"] = *req.PhoneNumber
	}
	if req.PhotoURL != nil {
		fields["photo_url"] = *req.PhotoURL
	}
	if req.AddressStreet != nil {
		fields["address_street"] = *req.AddressStreet
	}
	if req.AddressWard != nil {
		fields["address_ward"] = *req.AddressWard
	}
	if req.AddressDistrict != nil {
		fields["address_district"] = *req.AddressDistrict
	}
	if req.AddressCity != nil {
		fields["address_city"] = *req.AddressCity
	}

	updated, err := h.userRepo.UpdateProfile(r.Context(), user.ID, fields)
	if err != nil {
		sendError(w, "Failed to update profile", "INTERNAL_ERROR", 500)
		return
	}
	sendSuccess(w, mapUserToDTO(updated), 200)
}

// POST /api/auth/fcm-token - Add FCM token.
func (h *AuthHandler) AddFCMToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req dto.AddFCMTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	if err := h.userRepo.AddFCMToken(r.Context(), user.ID, req.Token); err != nil {
		sendError(w, "Failed to add FCM token", "INTERNAL_ERROR", 500)
		return
	}
	sendSuccess(w, map[string]bool{"success": true}, 200)
}

// DELETE /api/auth/fcm-token/:token - Remove FCM token.
func (h *AuthHandler) RemoveFCMToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	token := chi.URLParam(r, "token")
	if err := h.userRepo.RemoveFCMToken(r.Context(), user.ID, token); err != nil {
		sendError(w, "Failed to remove FCM token", "INTERNAL_ERROR", 500)
		return
	}
	w.WriteHeader(204)
}

// DELETE /api/auth/me - Delete account.
func (h *AuthHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if err := h.userRepo.Delete(r.Context(), user.ID); err != nil {
		sendError(w, "Failed to delete account", "INTERNAL_ERROR", 500)
		return
	}
	w.WriteHeader(204)
}

// GET /api/auth/me/bookings - Get user bookings.
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

	rows, total, err := h.availRepo.GetUserBookings(r.Context(), user.ID, page, limit)
	if err != nil {
		sendError(w, "Failed to get bookings", "INTERNAL_ERROR", 500)
		return
	}

	var items []dto.BookingHistoryItemResponse
	for _, b := range rows {
		item := dto.BookingHistoryItemResponse{
			ID:         b.ID,
			Date:       b.Date,
			StartTime:  b.StartTime,
			EndTime:    b.EndTime,
			TotalPrice: b.TotalPrice,
			Status:     b.Status,
			Notes:      b.Notes,
			CreatedAt:  b.CreatedAt,
		}
		item.Court.ID = b.CourtID
		item.Court.Name = b.CourtName
		item.SubCourt.ID = b.SubCourtID
		item.SubCourt.Name = b.SubCourtName
		items = append(items, item)
	}
	if items == nil {
		items = []dto.BookingHistoryItemResponse{}
	}
	sendPaginated(w, items, page, limit, total)
}

// GET /api/auth/username/check - Check username availability.
func (h *AuthHandler) CheckUsername(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		sendError(w, "username is required", "BAD_REQUEST", 400)
		return
	}
	existing, _ := h.userRepo.FindByUsername(r.Context(), username)
	sendSuccess(w, dto.UsernameAvailabilityResponse{
		Username:  username,
		Available: existing == nil,
	}, 200)
}

// GET /api/auth/username/lookup - Lookup email by username.
func (h *AuthHandler) LookupUsername(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		sendError(w, "username is required", "BAD_REQUEST", 400)
		return
	}
	user, _ := h.userRepo.FindByUsername(r.Context(), username)
	if user == nil || user.Email == nil {
		sendError(w, "Username not found", "NOT_FOUND", 404)
		return
	}
	sendSuccess(w, dto.UsernameLookupResponse{Username: username, Email: *user.Email}, 200)
}

func mapUserToDTO(u *domain.User) dto.UserProfileResponse {
	if u == nil {
		return dto.UserProfileResponse{}
	}
	resp := dto.UserProfileResponse{
		ID:          u.ID,
		FirebaseUID: u.FirebaseUID,
		Email:       u.Email,
		Username:    u.Username,
		Provider:    u.Provider,
		IsAnonymous: u.IsAnonymous,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		Gender:      u.Gender,
		PhoneNumber: u.PhoneNumber,
		PhotoURL:    u.PhotoURL,
		CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:   u.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	if u.AddressStreet != nil || u.AddressWard != nil || u.AddressDistrict != nil || u.AddressCity != nil {
		resp.Address = &dto.UserAddressResponse{
			Street:   u.AddressStreet,
			Ward:     u.AddressWard,
			District: u.AddressDistrict,
			City:     u.AddressCity,
		}
	}
	return resp
}

func splitName(name string) []string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == ' ' {
			return []string{name[:i], name[i+1:]}
		}
	}
	return []string{name}
}

// POST /api/auth/me/photo - Upload profile photo (stub: S3 upload wired separately)
func (h *AuthHandler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	sendError(w, "Photo upload not yet implemented", "NOT_IMPLEMENTED", 501)
}

// POST /api/auth/convert - Convert anonymous user to registered
func (h *AuthHandler) Convert(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req dto.ConvertAnonymousRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}

	fields := map[string]interface{}{
		"provider":     req.Provider,
		"is_anonymous": false,
	}
	if req.Email != nil    { fields["email"] = *req.Email }
	if req.Username != nil { fields["username"] = *req.Username }
	if req.NewFirebaseUID != nil { fields["firebase_uid"] = *req.NewFirebaseUID }
	if req.Profile != nil {
		if req.Profile.FirstName != nil { fields["first_name"] = *req.Profile.FirstName }
		if req.Profile.LastName != nil  { fields["last_name"] = *req.Profile.LastName }
		if req.Profile.Gender != nil    { fields["gender"] = *req.Profile.Gender }
		if req.Profile.PhoneNumber != nil { fields["phone_number"] = *req.Profile.PhoneNumber }
	}

	updated, err := h.userRepo.UpdateProfile(r.Context(), user.ID, fields)
	if err != nil {
		sendError(w, "Failed to convert account", "INTERNAL_ERROR", 500)
		return
	}
	sendSuccess(w, dto.AuthResponse{User: mapUserToDTO(updated), IsNewUser: false}, 200)
}

// POST /api/auth/link-bookings - Link guest bookings to authenticated user by phone
func (h *AuthHandler) LinkBookings(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user.PhoneNumber == nil {
		sendError(w, "User has no phone number on file", "BAD_REQUEST", 400)
		return
	}
	// Link bookings by phone number — update bookings where guest_phone matches and user_id is NULL
	_, err := h.availRepo.LinkGuestBookings(r.Context(), *user.PhoneNumber, user.ID)
	if err != nil {
		sendError(w, "Failed to link bookings", "INTERNAL_ERROR", 500)
		return
	}
	sendSuccess(w, map[string]bool{"success": true}, 200)
}
