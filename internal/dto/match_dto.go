package dto

// CreateMatchRequest from Flutter
type CreateMatchRequest struct {
	CourtID      string   `json:"courtId" validate:"required,uuid"`
	Title        *string  `json:"title,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Images       []string `json:"images,omitempty"`
	SkillLevel   string   `json:"skillLevel" validate:"required"`
	ShuttleType  string   `json:"shuttleType" validate:"required"`
	PlayerFormat string   `json:"playerFormat" validate:"required"`
	Date         string   `json:"date" validate:"required"`
	StartTime    string   `json:"startTime" validate:"required"`
	EndTime      string   `json:"endTime" validate:"required"`
	IsPrivate    bool     `json:"isPrivate"`
	Price        int      `json:"price"`
	SlotsNeeded  int      `json:"slotsNeeded" validate:"required,min=1,max=20"`
}

// UpdateMatchRequest from Flutter
type UpdateMatchRequest struct {
	Title        *string  `json:"title,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Images       []string `json:"images,omitempty"`
	SkillLevel   *string  `json:"skillLevel,omitempty"`
	ShuttleType  *string  `json:"shuttleType,omitempty"`
	PlayerFormat *string  `json:"playerFormat,omitempty"`
	Date         *string  `json:"date,omitempty"`
	StartTime    *string  `json:"startTime,omitempty"`
	EndTime      *string  `json:"endTime,omitempty"`
	IsPrivate    *bool    `json:"isPrivate,omitempty"`
	Price        *int     `json:"price,omitempty"`
	SlotsNeeded  *int     `json:"slotsNeeded,omitempty"`
	Status       *string  `json:"status,omitempty"`
}

// JoinMatchRequest from Flutter
type JoinMatchRequest struct {
	Message *string `json:"message,omitempty"`
}

// RespondToJoinRequest from Flutter (host action)
type RespondToJoinRequest struct {
	Status string `json:"status" validate:"required,oneof=ACCEPTED REJECTED"`
}

// MatchQueryParams from query string
type MatchQueryParams struct {
	CourtID        string
	SkillLevel     string
	PlayerFormat   string
	Status         string
	Date           string
	DateFrom       string
	DateTo         string
	IncludeExpired bool
	Page           int
	Limit          int
}

// MatchPlayerResponse nested in MatchWithPlayersResponse
type MatchPlayerResponse struct {
	ID           string  `json:"id"`
	MatchID      string  `json:"matchId"`
	UserID       string  `json:"userId"`
	UserName     string  `json:"userName"`
	UserPhotoURL *string `json:"userPhotoUrl"`
	Status       string  `json:"status"`
	Message      *string `json:"message"`
	Position     *int    `json:"position"`
	RequestedAt  string  `json:"requestedAt"`
	RespondedAt  *string `json:"respondedAt"`
}

// MatchResponse sent to Flutter
type MatchResponse struct {
	ID            string   `json:"id"`
	CourtID       string   `json:"courtId"`
	CourtName     string   `json:"courtName"`
	CourtAddress  string   `json:"courtAddress"`
	HostUserID    string   `json:"hostUserId"`
	HostName      string   `json:"hostName"`
	Title         *string  `json:"title"`
	Description   *string  `json:"description"`
	Images        []string `json:"images"`
	SkillLevel    string   `json:"skillLevel"`
	ShuttleType   string   `json:"shuttleType"`
	PlayerFormat  string   `json:"playerFormat"`
	Date          string   `json:"date"`
	StartTime     string   `json:"startTime"`
	EndTime       string   `json:"endTime"`
	IsPrivate     bool     `json:"isPrivate"`
	Price         int      `json:"price"`
	SlotsNeeded   int      `json:"slotsNeeded"`
	SlotsAccepted int      `json:"slotsAccepted"`
	Status        string   `json:"status"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
}

// CurrentUserStatusResponse nested in MatchWithPlayersResponse
type CurrentUserStatusResponse struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	Position    *int    `json:"position"`
	RequestedAt string  `json:"requestedAt"`
	RespondedAt *string `json:"respondedAt"`
}

// MatchWithPlayersResponse sent to Flutter for match detail
type MatchWithPlayersResponse struct {
	MatchResponse
	Players           []MatchPlayerResponse      `json:"players"`
	CurrentUserStatus *CurrentUserStatusResponse `json:"currentUserStatus"`
}
