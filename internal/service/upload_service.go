package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/smatch/badminton-backend/internal/domain"
)

type s3Uploader interface {
	PutObject(ctx context.Context, bucket, key string, body io.Reader, contentType string) error
	PutObjectEncrypted(ctx context.Context, bucket, key string, body io.Reader, contentType string) error
	DeleteObject(ctx context.Context, bucket, key string) error
}

type UploadService struct {
	s3     s3Uploader
	bucket string
}

func NewUploadService(s3Client s3Uploader, bucket string) *UploadService {
	return &UploadService{s3: s3Client, bucket: bucket}
}

var allowedExts = map[string]string{
	".pdf":  "application/pdf",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
}

var allowedImageExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
}

func (s *UploadService) UploadDocument(ctx context.Context, file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType, ok := allowedExts[ext]
	if !ok {
		return "", domain.BadRequest("Invalid file type. Allowed: pdf, jpg, jpeg, png")
	}

	key := fmt.Sprintf("%s/%s-%d%s", folder, uuid.New().String(), time.Now().Unix(), ext)

	if err := s.s3.PutObjectEncrypted(ctx, s.bucket, key, file, contentType); err != nil {
		return "", &domain.AppError{Code: "UPLOAD_FAILED", Message: "Failed to upload document", Status: 500, Err: err}
	}

	// Return a presigned or public URL placeholder; adjust based on S3 setup
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key), nil
}

func (s *UploadService) UploadMatchImage(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType, ok := allowedImageExts[ext]
	if !ok {
		return "", domain.BadRequest("Unsupported image type. Allowed: jpg, jpeg, png")
	}

	key := fmt.Sprintf("matches/%s-%d%s", uuid.New().String(), time.Now().Unix(), ext)

	if err := s.s3.PutObject(ctx, s.bucket, key, file, contentType); err != nil {
		return "", &domain.AppError{Code: "UPLOAD_FAILED", Message: "Failed to upload image", Status: 500, Err: err}
	}

	return key, nil
}

// UploadProfilePhoto uploads a profile photo for a user and deletes the previous photo if any.
// Returns the S3 key (not a full URL).
func (s *UploadService) UploadProfilePhoto(ctx context.Context, userID, oldKey string, file multipart.File, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType, ok := allowedImageExts[ext]
	if !ok {
		return "", domain.BadRequest("Unsupported image type. Allowed: jpg, jpeg, png")
	}

	key := fmt.Sprintf("profile/%s/%s-%d%s", userID, uuid.New().String(), time.Now().Unix(), ext)

	if err := s.s3.PutObject(ctx, s.bucket, key, file, contentType); err != nil {
		return "", &domain.AppError{Code: "UPLOAD_FAILED", Message: "Failed to upload profile photo", Status: 500, Err: err}
	}

	if oldKey != "" {
		if err := s.s3.DeleteObject(ctx, s.bucket, oldKey); err != nil {
			fmt.Printf("[upload] warning: failed to delete old profile photo %q: %v\n", oldKey, err)
		}
	}

	return key, nil
}
