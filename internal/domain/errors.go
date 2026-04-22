package domain

import "errors"

var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
	ErrBadRequest = errors.New("bad request")
	ErrForbidden  = errors.New("forbidden")
	ErrUnauth     = errors.New("unauthorized")

	ErrBusinessProfileNotFound  = errors.New("business profile not found")
	ErrBusinessProfileAlreadyExists = errors.New("business profile already exists")
	ErrBusinessProfileNotApproved   = errors.New("business profile not approved")
	ErrNotCourtOwner = errors.New("not a court owner")
	ErrNotAdmin      = errors.New("not an admin")
	ErrInvalidFileType = errors.New("invalid file type")
	ErrUploadFailed    = errors.New("upload failed")
	ErrCourtNotOwned   = errors.New("court not owned")
)

type AppError struct {
	Code    string
	Message string
	Status  int
	Err     error
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Err }

func NotFound(msg string) *AppError {
	return &AppError{Code: "NOT_FOUND", Message: msg, Status: 404, Err: ErrNotFound}
}

func BadRequest(msg string) *AppError {
	return &AppError{Code: "BAD_REQUEST", Message: msg, Status: 400, Err: ErrBadRequest}
}

func Conflict(msg string) *AppError {
	return &AppError{Code: "CONFLICT", Message: msg, Status: 409, Err: ErrConflict}
}

func Forbidden(msg string) *AppError {
	return &AppError{Code: "FORBIDDEN", Message: msg, Status: 403, Err: ErrForbidden}
}

func Unauthorized(msg string) *AppError {
	return &AppError{Code: "UNAUTHORIZED", Message: msg, Status: 401, Err: ErrUnauth}
}
