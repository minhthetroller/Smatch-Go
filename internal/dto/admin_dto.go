package dto

// AdminStatsResponse sent to admin web app
type AdminStatsResponse struct {
	TotalActiveUsers        int64 `json:"totalActiveUsers"`
	TotalCourtOwners        int64 `json:"totalCourtOwners"`
	TotalCourts             int64 `json:"totalCourts"`
	TotalRevenue            int64 `json:"totalRevenue"`
	PendingApplications     int64 `json:"pendingApplications"`
	RecentSignups           int64 `json:"recentSignups"`
}

// ListBusinessProfilesQuery parsed from query params
type ListBusinessProfilesQuery struct {
	Status string `json:"status" validate:"omitempty,oneof=pending approved rejected resubmit_requested"`
	Page   int    `json:"page" validate:"min=1"`
	Limit  int    `json:"limit" validate:"min=1,max=100"`
}

// AuditLogResponse sent to admin web app
type AuditLogResponse struct {
	ID         string                 `json:"id"`
	AdminUserID string                `json:"adminUserId"`
	Action     string                 `json:"action"`
	TargetType *string                `json:"targetType"`
	TargetID   *string                `json:"targetId"`
	Details    map[string]interface{} `json:"details"`
	CreatedAt  string                 `json:"createdAt"`
}

// CourtOwnerStatsResponse sent to admin web app
type CourtOwnerStatsResponse struct {
	OwnerID        string `json:"ownerId"`
	OwnerName      string `json:"ownerName"`
	TotalCourts    int64  `json:"totalCourts"`
	TotalBookings  int64  `json:"totalBookings"`
	TotalRevenue   int64  `json:"totalRevenue"`
}
