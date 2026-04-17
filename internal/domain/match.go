package domain

import "time"

type SkillLevel string

const (
	SkillLevelTBY         SkillLevel = "TBY"
	SkillLevelY           SkillLevel = "Y"
	SkillLevelYPlus       SkillLevel = "Y_PLUS"
	SkillLevelYPlusPlus   SkillLevel = "Y_PLUS_PLUS"
	SkillLevelTBK         SkillLevel = "TBK"
	SkillLevelTB          SkillLevel = "TB"
	SkillLevelTBPlus      SkillLevel = "TB_PLUS"
	SkillLevelTBPlusPlus  SkillLevel = "TB_PLUS_PLUS"
	SkillLevelK           SkillLevel = "K"
	SkillLevelKPlus       SkillLevel = "K_PLUS"
	SkillLevelGIOI        SkillLevel = "GIOI"
)

type ShuttleType string

const (
	ShuttleTC77           ShuttleType = "TC77"
	ShuttleBASAO          ShuttleType = "BASAO"
	ShuttleYonexAS30      ShuttleType = "YONEX_AS30"
	ShuttleYonexAS40      ShuttleType = "YONEX_AS40"
	ShuttleYonexAS50      ShuttleType = "YONEX_AS50"
	ShuttleVictorMaster1  ShuttleType = "VICTOR_MASTER_1"
	ShuttleVictorChamp1   ShuttleType = "VICTOR_CHAMPION_1"
	ShuttleRSLClassic     ShuttleType = "RSL_CLASSIC"
	ShuttleLindan40       ShuttleType = "LINDAN_40"
	ShuttleLindan50       ShuttleType = "LINDAN_50"
	ShuttleOther          ShuttleType = "OTHER"
)

type PlayerFormat string

const (
	PlayerFormatSingleMale   PlayerFormat = "SINGLE_MALE"
	PlayerFormatSingleFemale PlayerFormat = "SINGLE_FEMALE"
	PlayerFormatDoubleMale   PlayerFormat = "DOUBLE_MALE"
	PlayerFormatDoubleFemale PlayerFormat = "DOUBLE_FEMALE"
	PlayerFormatMixedDouble  PlayerFormat = "MIXED_DOUBLE"
	PlayerFormatAny          PlayerFormat = "ANY"
)

type MatchStatus string

const (
	MatchStatusOpen       MatchStatus = "OPEN"
	MatchStatusFull       MatchStatus = "FULL"
	MatchStatusInProgress MatchStatus = "IN_PROGRESS"
	MatchStatusCompleted  MatchStatus = "COMPLETED"
	MatchStatusCancelled  MatchStatus = "CANCELLED"
)

type MatchPlayerStatus string

const (
	MatchPlayerStatusPending        MatchPlayerStatus = "PENDING"
	MatchPlayerStatusPendingPayment MatchPlayerStatus = "PENDING_PAYMENT"
	MatchPlayerStatusAccepted       MatchPlayerStatus = "ACCEPTED"
	MatchPlayerStatusRejected       MatchPlayerStatus = "REJECTED"
	MatchPlayerStatusLeft           MatchPlayerStatus = "LEFT"
	MatchPlayerStatusExpired        MatchPlayerStatus = "EXPIRED"
)

type Match struct {
	ID          string
	CourtID     string
	HostUserID  string

	Title        *string
	Description  *string
	Images       []string

	SkillLevel   SkillLevel
	ShuttleType  ShuttleType
	PlayerFormat PlayerFormat

	Date      time.Time
	StartTime string // HH:MM
	EndTime   string // HH:MM

	IsPrivate   bool
	Price       int
	SlotsNeeded int

	Status MatchStatus

	CreatedAt time.Time
	UpdatedAt time.Time
}

type MatchPlayer struct {
	ID      string
	MatchID string
	UserID  string

	Status  MatchPlayerStatus
	Message *string
	Position *int

	RequestedAt time.Time
	RespondedAt *time.Time
}
