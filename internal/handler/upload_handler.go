package handler

import (
	"context"
	"log"
	"mime/multipart"
	"net/http"

	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/imageurl"
)

type matchImageUploader interface {
	UploadMatchImage(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error)
}

type UploadHandler struct {
	upload matchImageUploader
	images imageurl.Resolver
}

func NewUploadHandler(upload matchImageUploader, images imageurl.Resolver) *UploadHandler {
	return &UploadHandler{upload: upload, images: images}
}

const maxUploadSize = 5 << 20

func (h *UploadHandler) UploadMatchImage(w http.ResponseWriter, r *http.Request) {
	if h.upload == nil {
		sendError(w, "Image upload not available", "UPLOAD_UNAVAILABLE", 503)
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		sendError(w, "File too large or invalid form data", "BAD_REQUEST", 400)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		sendError(w, "No image file provided", "BAD_REQUEST", 400)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			// Send an error internal service
			// Currently we don't have it
			// only SendAppError and SendError are available
			log.Fatalf("Failed to close file: %v", err)
		}
	}(file)

	key, err := h.upload.UploadMatchImage(r.Context(), file, header)
	if err != nil {
		sendAppError(w, err)
		return
	}

	sendSuccess(w, dto.ImageUploadResponse{
		Key:      key,
		URL:      h.images.Match(key),
		FileName: header.Filename,
	}, 201)
}
