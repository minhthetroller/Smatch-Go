package service

import (
	"context"
	"errors"
	"mime/multipart"
	"testing"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/imageurl"
	"github.com/smatch/badminton-backend/internal/repository"
)

var testImageResolver = imageurl.New("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")

type stubProfileUploader struct {
	key       string
	err       error
	gotUserID string
	gotOldKey string
}

func (s *stubProfileUploader) UploadProfilePhoto(_ context.Context, userID, oldKey string, _ multipart.File, _ *multipart.FileHeader) (string, error) {
	s.gotUserID = userID
	s.gotOldKey = oldKey
	return s.key, s.err
}

type stubUserRepo struct {
	updatedID     string
	updatedFields map[string]interface{}
	returnUser    *domain.User
	returnErr     error
}

func (s *stubUserRepo) Upsert(context.Context, *domain.User) (*domain.User, bool, error) {
	return s.returnUser, false, s.returnErr
}
func (s *stubUserRepo) UpdateProfile(_ context.Context, id string, fields map[string]interface{}) (*domain.User, error) {
	s.updatedID = id
	s.updatedFields = fields
	return s.returnUser, s.returnErr
}
func (s *stubUserRepo) AddFCMToken(context.Context, string, string) error    { return nil }
func (s *stubUserRepo) RemoveFCMToken(context.Context, string, string) error { return nil }
func (s *stubUserRepo) Delete(context.Context, string) error                 { return nil }
func (s *stubUserRepo) FindByUsername(context.Context, string) (*domain.User, error) {
	return nil, nil
}

func newAuthSvcWithStubs(uploader *stubProfileUploader, userRepo *stubUserRepo) *AuthService {
	svc := &AuthService{images: testImageResolver}
	if uploader != nil {
		svc.upload = uploader
	}
	svc.userRepo = userRepo
	return svc
}

func TestAuthService_MapUserToDTO_ResolvesPhotoURL(t *testing.T) {
	svc := &AuthService{images: testImageResolver}
	user := &domain.User{
		ID:       "user-1",
		PhotoURL: strPtr("profile/user-1/abc.jpg"),
	}

	resp := svc.mapUserToDTO(user)

	if resp.PhotoURL == nil || *resp.PhotoURL != "http://localhost:4566/smatch-profiles/profile/user-1/abc.jpg" {
		t.Errorf("expected resolved photoUrl, got %v", resp.PhotoURL)
	}
}

func TestAuthService_MapUserToDTO_TolerantFullURL(t *testing.T) {
	svc := &AuthService{images: testImageResolver}
	user := &domain.User{
		ID:       "user-1",
		PhotoURL: strPtr("https://cdn.example.com/already.jpg"),
	}

	resp := svc.mapUserToDTO(user)

	if resp.PhotoURL == nil || *resp.PhotoURL != "https://cdn.example.com/already.jpg" {
		t.Errorf("expected full URL to pass through, got %v", resp.PhotoURL)
	}
}

func TestAuthService_UploadPhoto_NilUploader(t *testing.T) {
	svc := &AuthService{images: testImageResolver} // upload nil
	_, err := svc.UploadProfilePhoto(context.Background(), &domain.User{ID: "user-1"}, nil, nil)
	if err == nil {
		t.Fatal("expected error when uploader is nil")
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		if appErr.Code != "UPLOAD_UNAVAILABLE" || appErr.Status != 503 {
			t.Errorf("expected UPLOAD_UNAVAILABLE 503, got code=%s status=%d", appErr.Code, appErr.Status)
		}
	} else {
		t.Errorf("expected *domain.AppError, got %T", err)
	}
}

func TestAuthService_UploadPhoto_Success(t *testing.T) {
	uploader := &stubProfileUploader{key: "profile/user-1/abc.jpg"}
	userRepo := &stubUserRepo{
		returnUser: &domain.User{
			ID:        "user-1",
			PhotoURL:  strPtr("profile/user-1/abc.jpg"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	svc := newAuthSvcWithStubs(uploader, userRepo)

	result, err := svc.UploadProfilePhoto(context.Background(), &domain.User{ID: "user-1"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if userRepo.updatedFields["photo_url"] != "profile/user-1/abc.jpg" {
		t.Errorf("expected UpdateProfile called with photo_url=key, got %v", userRepo.updatedFields["photo_url"])
	}
	_ = result
}

func TestAuthService_UploadPhoto_ReplacesOldPhoto(t *testing.T) {
	uploader := &stubProfileUploader{key: "profile/user-1/new.jpg"}
	userRepo := &stubUserRepo{
		returnUser: &domain.User{
			ID:        "user-1",
			PhotoURL:  strPtr("profile/user-1/new.jpg"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	svc := newAuthSvcWithStubs(uploader, userRepo)

	_, err := svc.UploadProfilePhoto(context.Background(), &domain.User{
		ID:       "user-1",
		PhotoURL: strPtr("profile/user-1/old.jpg"),
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if uploader.gotOldKey != "profile/user-1/old.jpg" {
		t.Errorf("expected old key passed to uploader, got %q", uploader.gotOldKey)
	}
}

func TestAuthService_MyBookings(t *testing.T) {
	notes := "hi"
	rows := []*repository.BookingRow{
		{
			ID: "b1", Date: "2026-01-02", StartTime: "10:00", EndTime: "11:00",
			TotalPrice: 100, Status: "confirmed", Notes: &notes, CreatedAt: "2026-01-01",
			CourtID: "c1", CourtName: "Court One", SubCourtID: "sc1", SubCourtName: "SC One",
		},
	}
	availRepo := &stubAuthAvailRepo{bookings: rows, total: 1}
	svc := &AuthService{availRepo: availRepo, images: testImageResolver}

	items, total, err := svc.MyBookings(context.Background(), "user-1", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total=%d items=%d, want 1/1", total, len(items))
	}
	if items[0].Court.Name != "Court One" || items[0].SubCourt.Name != "SC One" {
		t.Errorf("unexpected court/subcourt mapping: %+v", items[0])
	}
}

func TestAuthService_LinkBookings_NoPhone(t *testing.T) {
	svc := &AuthService{images: testImageResolver, availRepo: &stubAuthAvailRepo{}}
	err := svc.LinkBookings(context.Background(), &domain.User{ID: "u1"})
	if err == nil {
		t.Fatal("expected error for missing phone")
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Status != 400 {
		t.Errorf("expected 400 bad request, got %v", err)
	}
}

type stubAuthAvailRepo struct {
	bookings []*repository.BookingRow
	total    int
	err      error
}

func (s *stubAuthAvailRepo) GetUserBookings(context.Context, string, int, int) ([]*repository.BookingRow, int, error) {
	return s.bookings, s.total, s.err
}
func (s *stubAuthAvailRepo) LinkGuestBookings(context.Context, string, string) (int64, error) {
	return 0, nil
}
