package domain

import "time"

type User struct {
	ID          string
	FirebaseUID string

	Email       *string
	Username    *string
	Provider    string
	IsAnonymous bool

	FirstName   *string
	LastName    *string
	Gender      *string
	PhoneNumber *string
	PhotoURL    *string

	AddressStreet   *string
	AddressWard     *string
	AddressDistrict *string
	AddressCity     *string

	FCMTokens []string

	CreatedAt time.Time
	UpdatedAt time.Time
}
