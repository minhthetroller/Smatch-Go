package dto

import "encoding/json"

// SubmitBusinessProfileRequest from court owner web app
type SubmitBusinessProfileRequest struct {
	LegalRepresentativeName        string              `json:"legalRepresentativeName" validate:"required"`
	PersonalIDNumber               string              `json:"personalIdNumber" validate:"required"`
	TaxIDNumber                    string              `json:"taxIdNumber" validate:"required"`
	BankAccountNumber              string              `json:"bankAccountNumber" validate:"required"`
	BankName                       string              `json:"bankName" validate:"required"`
	BankBranch                     string              `json:"bankBranch" validate:"required"`
	BankAccountHolderName          string              `json:"bankAccountHolderName" validate:"required"`
	OperationalSpecs               json.RawMessage     `json:"operationalSpecs" validate:"required"`
}

// BusinessProfileResponse sent to web app
type BusinessProfileResponse struct {
	ID                              string          `json:"id"`
	UserID                          string          `json:"userId"`
	LegalRepresentativeName         string          `json:"legalRepresentativeName"`
	PersonalIDNumber                string          `json:"personalIdNumber"`
	PersonalIDFrontImageURL         *string         `json:"personalIdFrontImageUrl"`
	PersonalIDBackImageURL          *string         `json:"personalIdBackImageUrl"`
	BusinessRegistrationCertURL     *string         `json:"businessRegistrationCertUrl"`
	SportsBusinessEligibilityCertURL *string        `json:"sportsBusinessEligibilityCertUrl"`
	FireSafetyCertURL               *string         `json:"fireSafetyCertUrl"`
	TaxIDNumber                     string          `json:"taxIdNumber"`
	ProofOfAddressURL               *string         `json:"proofOfAddressUrl"`
	BankAccountNumber               string          `json:"bankAccountNumber"`
	BankName                        string          `json:"bankName"`
	BankBranch                      string          `json:"bankBranch"`
	BankAccountHolderName           string          `json:"bankAccountHolderName"`
	OperationalSpecs                json.RawMessage `json:"operationalSpecs"`
	Status                          string          `json:"status"`
	AdminNotes                      *string         `json:"adminNotes"`
	SubmittedAt                     string          `json:"submittedAt"`
	ReviewedAt                      *string         `json:"reviewedAt"`
	ReviewedBy                      *string         `json:"reviewedBy"`
}

// ReviewBusinessProfileRequest from admin web app
type ReviewBusinessProfileRequest struct {
	Action      string `json:"action" validate:"required,oneof=approve reject request_resubmit"`
	AdminNotes  string `json:"adminNotes" validate:"required"`
}

// UploadDocumentResponse sent after S3 upload
type UploadDocumentResponse struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}
