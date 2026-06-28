package dto

// VerifyTokenRequest from Flutter
type VerifyTokenRequest struct {
	IDToken string                `json:"idToken" validate:"required"`
	Profile *UpdateProfileRequest `json:"profile,omitempty"`
}

// CreateAnonymousRequest from Flutter
type CreateAnonymousRequest struct {
	FirebaseUID string `json:"firebaseUid" validate:"required"`
}

// ConvertAnonymousRequest from Flutter
type ConvertAnonymousRequest struct {
	NewFirebaseUID *string               `json:"newFirebaseUid,omitempty"`
	IDToken        *string               `json:"idToken,omitempty"`
	Provider       string                `json:"provider" validate:"required,oneof=google facebook password"`
	Email          *string               `json:"email,omitempty"`
	Username       *string               `json:"username,omitempty"`
	Profile        *UpdateProfileRequest `json:"profile,omitempty"`
}

// UpdateProfileRequest from Flutter
type UpdateProfileRequest struct {
	Username        *string `json:"username,omitempty"`
	FirstName       *string `json:"firstName,omitempty"`
	LastName        *string `json:"lastName,omitempty"`
	Gender          *string `json:"gender,omitempty"`
	PhoneNumber     *string `json:"phoneNumber,omitempty"`
	PhotoURL        *string `json:"photoUrl,omitempty"`
	AddressStreet   *string `json:"addressStreet,omitempty"`
	AddressWard     *string `json:"addressWard,omitempty"`
	AddressDistrict *string `json:"addressDistrict,omitempty"`
	AddressCity     *string `json:"addressCity,omitempty"`
}

// AddFCMTokenRequest from Flutter
type AddFCMTokenRequest struct {
	Token string `json:"token" validate:"required"`
}

// UserProfileResponse sent to Flutter
type UserProfileResponse struct {
	ID          string               `json:"id"`
	FirebaseUID string               `json:"firebaseUid"`
	Email       *string              `json:"email"`
	Username    *string              `json:"username"`
	Provider    string               `json:"provider"`
	IsAnonymous bool                 `json:"isAnonymous"`
	FirstName   *string              `json:"firstName"`
	LastName    *string              `json:"lastName"`
	Gender      *string              `json:"gender"`
	PhoneNumber *string              `json:"phoneNumber"`
	PhotoURL    *string              `json:"photoUrl"`
	Address     *UserAddressResponse `json:"address"`
	CreatedAt   string               `json:"createdAt"`
	UpdatedAt   string               `json:"updatedAt"`
}

// UserAddressResponse nested in UserProfileResponse
type UserAddressResponse struct {
	Street   *string `json:"street"`
	Ward     *string `json:"ward"`
	District *string `json:"district"`
	City     *string `json:"city"`
}

// AuthResponse sent to Flutter on verify/anonymous
type AuthResponse struct {
	User      UserProfileResponse `json:"user"`
	IsNewUser bool                `json:"isNewUser"`
}

// UsernameAvailabilityResponse sent to Flutter
type UsernameAvailabilityResponse struct {
	Username  string `json:"username"`
	Available bool   `json:"available"`
}

// UsernameLookupResponse sent to Flutter
type UsernameLookupResponse struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

// BookingHistoryItemResponse nested in BookingHistoryResponse
type BookingHistoryItemResponse struct {
	ID         string  `json:"id"`
	Date       string  `json:"date"`
	StartTime  string  `json:"startTime"`
	EndTime    string  `json:"endTime"`
	TotalPrice int     `json:"totalPrice"`
	Status     string  `json:"status"`
	Notes      *string `json:"notes"`
	CreatedAt  string  `json:"createdAt"`
	Court      struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		AddressDistrict *string `json:"addressDistrict"`
		AddressCity     *string `json:"addressCity"`
	} `json:"court"`
	SubCourt struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"subCourt"`
}
