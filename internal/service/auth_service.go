package service

import (
	"context"
	"fmt"
	"mime/multipart"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/imageurl"
	"github.com/smatch/badminton-backend/internal/repository"
	firebasepkg "github.com/smatch/badminton-backend/platform/firebase"
)

type authUserRepository interface {
	Upsert(ctx context.Context, u *domain.User) (*domain.User, bool, error)
	UpdateProfile(ctx context.Context, id string, fields map[string]interface{}) (*domain.User, error)
	AddFCMToken(ctx context.Context, userID, token string) error
	RemoveFCMToken(ctx context.Context, userID, token string) error
	Delete(ctx context.Context, id string) error
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
}

type authAvailabilityRepository interface {
	GetUserBookings(ctx context.Context, userID string, page, limit int) ([]*repository.BookingRow, int, error)
	LinkGuestBookings(ctx context.Context, phone, userID string) (int64, error)
}

type profilePhotoUploader interface {
	UploadProfilePhoto(ctx context.Context, userID, oldKey string, file multipart.File, header *multipart.FileHeader) (string, error)
}

// AuthService contains auth profile/booking/business logic, returning DTOs.
type AuthService struct {
	firebase  *firebasepkg.Client
	userRepo  authUserRepository
	availRepo authAvailabilityRepository
	upload    profilePhotoUploader
	images    imageurl.Resolver
}

func NewAuthService(fb *firebasepkg.Client, ur authUserRepository, ar authAvailabilityRepository, upload *UploadService, images imageurl.Resolver) *AuthService {
	var uploader profilePhotoUploader
	if upload != nil {
		uploader = upload
	}
	return &AuthService{firebase: fb, userRepo: ur, availRepo: ar, upload: uploader, images: images}
}

// Verify validates a Firebase ID token and upserts the user.
func (s *AuthService) Verify(ctx context.Context, req *dto.VerifyTokenRequest) (dto.AuthResponse, error) {
	decoded, err := s.firebase.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		return dto.AuthResponse{}, &domain.AppError{Code: "INVALID_TOKEN", Message: "Invalid or expired token", Status: 401, Err: domain.ErrUnauth}
	}

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

	created, isNew, err := s.userRepo.Upsert(ctx, u)
	if err != nil {
		return dto.AuthResponse{}, fmt.Errorf("auth verify upsert: %w", err)
	}
	return dto.AuthResponse{User: s.mapUserToDTO(created), IsNewUser: isNew}, nil
}

// Anonymous creates or fetches an anonymous user.
func (s *AuthService) Anonymous(ctx context.Context, req *dto.CreateAnonymousRequest) (dto.AuthResponse, error) {
	u := &domain.User{
		FirebaseUID: req.FirebaseUID,
		Provider:    "anonymous",
		IsAnonymous: true,
	}
	created, isNew, err := s.userRepo.Upsert(ctx, u)
	if err != nil {
		return dto.AuthResponse{}, fmt.Errorf("auth anonymous upsert: %w", err)
	}
	return dto.AuthResponse{User: s.mapUserToDTO(created), IsNewUser: isNew}, nil
}

// GetProfile returns the DTO for the given authenticated user.
func (s *AuthService) GetProfile(_ context.Context, user *domain.User) dto.UserProfileResponse {
	return s.mapUserToDTO(user)
}

// UpdateProfile applies validated profile field updates.
func (s *AuthService) UpdateProfile(ctx context.Context, userID string, req *dto.UpdateProfileRequest) (*dto.UserProfileResponse, error) {
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
	updated, err := s.userRepo.UpdateProfile(ctx, userID, fields)
	if err != nil {
		return nil, fmt.Errorf("auth update profile: %w", err)
	}
	resp := s.mapUserToDTO(updated)
	return &resp, nil
}

// AddFCMToken registers a device token for the user.
func (s *AuthService) AddFCMToken(ctx context.Context, userID, token string) error {
	if err := s.userRepo.AddFCMToken(ctx, userID, token); err != nil {
		return fmt.Errorf("auth add fcm token: %w", err)
	}
	return nil
}

// RemoveFCMToken removes a device token for the user.
func (s *AuthService) RemoveFCMToken(ctx context.Context, userID, token string) error {
	if err := s.userRepo.RemoveFCMToken(ctx, userID, token); err != nil {
		return fmt.Errorf("auth remove fcm token: %w", err)
	}
	return nil
}

// DeleteAccount permanently deletes the user account.
func (s *AuthService) DeleteAccount(ctx context.Context, userID string) error {
	if err := s.userRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("auth delete: %w", err)
	}
	return nil
}

// MyBookings returns paginated booking history for the user.
func (s *AuthService) MyBookings(ctx context.Context, userID string, page, limit int) ([]dto.BookingHistoryItemResponse, int, error) {
	rows, total, err := s.availRepo.GetUserBookings(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("auth my bookings: %w", err)
	}
	items := make([]dto.BookingHistoryItemResponse, 0, len(rows))
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
	return items, total, nil
}

// CheckUsername reports whether a username is available.
func (s *AuthService) CheckUsername(ctx context.Context, username string) (dto.UsernameAvailabilityResponse, error) {
	existing, _ := s.userRepo.FindByUsername(ctx, username)
	return dto.UsernameAvailabilityResponse{Username: username, Available: existing == nil}, nil
}

// LookupUsername returns the email associated with a username, if any.
func (s *AuthService) LookupUsername(ctx context.Context, username string) (*dto.UsernameLookupResponse, error) {
	user, _ := s.userRepo.FindByUsername(ctx, username)
	if user == nil || user.Email == nil {
		return nil, domain.NotFound("Username not found")
	}
	return &dto.UsernameLookupResponse{Username: username, Email: *user.Email}, nil
}

// ConvertAnonymousResult is the result of converting an anonymous user.
type ConvertAnonymousResult struct {
	User      dto.UserProfileResponse
	IsNewUser bool
}

// Convert converts an anonymous user to a registered account.
//
// If an idToken is provided, it is verified with Firebase and the provider
// profile photo (`picture` claim) and provider UID are extracted so the
// converted user carries the OAuth profile photo (e.g. Google account photo).
// The picture is only applied when the client has not explicitly set a
// photoUrl in the profile payload.
func (s *AuthService) Convert(ctx context.Context, userID string, req *dto.ConvertAnonymousRequest) (ConvertAnonymousResult, error) {
	fields := map[string]interface{}{
		"provider":     req.Provider,
		"is_anonymous": false,
	}

	var explicitPhotoURL *string
	if req.Profile != nil {
		explicitPhotoURL = req.Profile.PhotoURL
	}

	// Verify the OAuth ID token (if provided) to source provider identity + photo.
	if req.IDToken != nil && *req.IDToken != "" {
		decoded, err := s.firebase.VerifyIDToken(ctx, *req.IDToken)
		if err != nil {
			return ConvertAnonymousResult{}, &domain.AppError{Code: "INVALID_TOKEN", Message: "Invalid or expired token", Status: 401, Err: domain.ErrUnauth}
		}
		// Derive the new Firebase UID from the verified token when not provided.
		if req.NewFirebaseUID == nil || *req.NewFirebaseUID == "" {
			uid := decoded.UID
			req.NewFirebaseUID = &uid
		}
		// Carry the provider email when not already supplied.
		if _, hasEmail := fields["email"]; !hasEmail {
			if email, ok := decoded.Claims["email"].(string); ok && email != "" {
				fields["email"] = email
			}
		}
		// Capture the OAuth profile photo (e.g. Google picture) unless the
		// client explicitly set one in the profile payload.
		if explicitPhotoURL == nil || *explicitPhotoURL == "" {
			if picture, ok := decoded.Claims["picture"].(string); ok && picture != "" {
				fields["photo_url"] = picture
			}
		}
	}

	if req.Email != nil {
		fields["email"] = *req.Email
	}
	if req.Username != nil {
		fields["username"] = *req.Username
	}
	if req.NewFirebaseUID != nil {
		fields["firebase_uid"] = *req.NewFirebaseUID
	}
	if req.Profile != nil {
		if req.Profile.FirstName != nil {
			fields["first_name"] = *req.Profile.FirstName
		}
		if req.Profile.LastName != nil {
			fields["last_name"] = *req.Profile.LastName
		}
		if req.Profile.Gender != nil {
			fields["gender"] = *req.Profile.Gender
		}
		if req.Profile.PhoneNumber != nil {
			fields["phone_number"] = *req.Profile.PhoneNumber
		}
	}

	updated, err := s.userRepo.UpdateProfile(ctx, userID, fields)
	if err != nil {
		return ConvertAnonymousResult{}, fmt.Errorf("auth convert: %w", err)
	}
	return ConvertAnonymousResult{User: s.mapUserToDTO(updated), IsNewUser: false}, nil
}

// LinkBookings links guest bookings to the authenticated user by phone number.
func (s *AuthService) LinkBookings(ctx context.Context, user *domain.User) error {
	if user.PhoneNumber == nil {
		return domain.BadRequest("User has no phone number on file")
	}
	if _, err := s.availRepo.LinkGuestBookings(ctx, *user.PhoneNumber, user.ID); err != nil {
		return fmt.Errorf("auth link bookings: %w", err)
	}
	return nil
}

// ProfilePhotoUploadResult is the result of a profile photo upload.
type ProfilePhotoUploadResult struct {
	User dto.UserProfileResponse
}

// UploadProfilePhoto uploads a profile photo for a user, replacing the prior one.
func (s *AuthService) UploadProfilePhoto(ctx context.Context, user *domain.User, file multipart.File, header *multipart.FileHeader) (ProfilePhotoUploadResult, error) {
	if s.upload == nil {
		return ProfilePhotoUploadResult{}, &domain.AppError{Code: "UPLOAD_UNAVAILABLE", Message: "Photo upload not available", Status: 503}
	}

	var oldKey string
	if user.PhotoURL != nil && *user.PhotoURL != "" {
		oldKey = *user.PhotoURL
	}

	key, err := s.upload.UploadProfilePhoto(ctx, user.ID, oldKey, file, header)
	if err != nil {
		return ProfilePhotoUploadResult{}, err
	}

	updated, err := s.userRepo.UpdateProfile(ctx, user.ID, map[string]interface{}{"photo_url": key})
	if err != nil {
		return ProfilePhotoUploadResult{}, fmt.Errorf("auth upload profile photo: %w", err)
	}
	return ProfilePhotoUploadResult{User: s.mapUserToDTO(updated)}, nil
}

// ==================== Helpers ====================

func (s *AuthService) mapUserToDTO(u *domain.User) dto.UserProfileResponse {
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
		PhotoURL:    s.resolvePhotoURL(u.PhotoURL),
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

func (s *AuthService) resolvePhotoURL(photoURL *string) *string {
	if photoURL == nil || *photoURL == "" {
		return photoURL
	}
	resolved := s.images.Profile(*photoURL)
	return &resolved
}

func splitName(name string) []string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == ' ' {
			return []string{name[:i], name[i+1:]}
		}
	}
	return []string{name}
}
