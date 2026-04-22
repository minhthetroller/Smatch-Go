package handler

import (
	"encoding/json"
	"mime/multipart"
	"net/http"

	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/service"
)

type BusinessProfileHandler struct {
	service *service.BusinessProfileService
}

func NewBusinessProfileHandler(s *service.BusinessProfileService) *BusinessProfileHandler {
	return &BusinessProfileHandler{service: s}
}

func (h *BusinessProfileHandler) Submit(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		sendError(w, "Unauthorized", "UNAUTHORIZED", 401)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		sendError(w, "Failed to parse form", "BAD_REQUEST", 400)
		return
	}

	var req dto.SubmitBusinessProfileRequest
	if err := json.Unmarshal([]byte(r.FormValue("data")), &req); err != nil {
		sendError(w, "Invalid JSON data", "BAD_REQUEST", 400)
		return
	}

	files := make(map[string]*multipart.FileHeader)
	for _, key := range []string{"personalIdFront", "personalIdBack", "businessRegistrationCert", "sportsBusinessEligibilityCert", "fireSafetyCert", "proofOfAddress"} {
		if fh := r.MultipartForm.File[key]; len(fh) > 0 {
			files[key] = fh[0]
		}
	}

	resp, err := h.service.SubmitApplication(r.Context(), user.ID, req, files)
	if err != nil {
		sendAppError(w, err)
		return
	}

	sendSuccess(w, resp, 201)
}

func (h *BusinessProfileHandler) GetMine(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		sendError(w, "Unauthorized", "UNAUTHORIZED", 401)
		return
	}

	resp, err := h.service.GetApplication(r.Context(), user.ID)
	if err != nil {
		sendAppError(w, err)
		return
	}

	sendSuccess(w, resp, 200)
}
