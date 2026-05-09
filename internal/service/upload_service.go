package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/platform/blob"
)

type UploadService struct {
	blob    *blob.Client
	container string
}

func NewUploadService(blobClient *blob.Client, container string) *UploadService {
	return &UploadService{blob: blobClient, container: container}
}

var allowedExts = map[string]string{
	".pdf":  "application/pdf",
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

	if err := s.blob.PutObjectEncrypted(ctx, s.container, key, file, contentType); err != nil {
		return "", &domain.AppError{Code: "UPLOAD_FAILED", Message: "Failed to upload document", Status: 500, Err: err}
	}

	return fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", s.blob.AccountName, s.container, key), nil
}
