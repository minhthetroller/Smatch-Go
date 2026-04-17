package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/smatch/badminton-backend/internal/domain"
)

type successEnvelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type paginationMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"totalPages"`
	HasNext    bool `json:"hasNext"`
	HasPrev    bool `json:"hasPrev"`
}

type paginatedEnvelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Meta    struct {
		Pagination paginationMeta `json:"pagination"`
	} `json:"meta"`
}

type errorBody struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type errorEnvelope struct {
	Success bool      `json:"success"`
	Error   errorBody `json:"error"`
}

func sendSuccess(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(successEnvelope{Success: true, Data: data})
}

func sendPaginated(w http.ResponseWriter, data interface{}, page, limit, total int) {
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}
	env := paginatedEnvelope{Success: true, Data: data}
	env.Meta.Pagination = paginationMeta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(env)
}

func sendError(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Success: false,
		Error:   errorBody{Message: message, Code: code},
	})
}

func sendAppError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		sendError(w, appErr.Message, appErr.Code, appErr.Status)
		return
	}
	sendError(w, "Internal server error", "INTERNAL_ERROR", http.StatusInternalServerError)
}
