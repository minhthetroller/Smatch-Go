package domain

import (
	"encoding/json"
	"time"
)

type BusinessProfileStatus string

const (
	BusinessProfilePending         BusinessProfileStatus = "pending"
	BusinessProfileApproved        BusinessProfileStatus = "approved"
	BusinessProfileRejected        BusinessProfileStatus = "rejected"
	BusinessProfileResubmitReq     BusinessProfileStatus = "resubmit_requested"
)

type OperationalSpecs struct {
	SubcourtCount   int                `json:"subcourt_count"`
	SurfaceType     string             `json:"surface_type"`
	OperatingHours  OperatingHours     `json:"operating_hours"`
	BasePricing     []BasePricingRule  `json:"base_pricing"`
}

type OperatingHours struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

type BasePricingRule struct {
	DayType      string `json:"day_type"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	PricePerHour int    `json:"price_per_hour"`
}

type BusinessProfile struct {
	ID                            string
	UserID                        string
	LegalRepresentativeName       string
	PersonalIDNumber              string
	PersonalIDFrontImageURL       *string
	PersonalIDBackImageURL        *string
	BusinessRegistrationCertURL   *string
	SportsBusinessEligibilityCertURL *string
	FireSafetyCertURL             *string
	TaxIDNumber                   string
	ProofOfAddressURL             *string
	BankAccountNumber             string
	BankName                      string
	BankBranch                    string
	BankAccountHolderName         string
	OperationalSpecs              json.RawMessage
	Status                        BusinessProfileStatus
	AdminNotes                    *string
	SubmittedAt                   time.Time
	ReviewedAt                    *time.Time
	ReviewedBy                    *string
}

type AdminAuditLog struct {
	ID          string
	AdminUserID string
	Action      string
	TargetType  *string
	TargetID    *string
	Details     json.RawMessage
	CreatedAt   time.Time
}

type PlatformStats struct {
	TotalActiveUsers        int64
	TotalCourtOwners        int64
	TotalCourts             int64
	TotalRevenue            int64
	PendingApplications     int64
	RecentSignups           int64
}

type CourtStats struct {
	TotalBookings       int64
	TotalRevenue        int64
	OccupancyRate       float64
	CancellationRate    float64
}
