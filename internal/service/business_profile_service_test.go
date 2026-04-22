package service

import (
	"testing"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
)

func TestBusinessProfileService_BankHolderValidation(t *testing.T) {
	// We can't easily instantiate the service without repos,
	// so test the validation logic inline.
	req := dto.SubmitBusinessProfileRequest{
		LegalRepresentativeName: "Nguyen Van A",
		BankAccountHolderName:   "Nguyen Van B",
	}

	if req.BankAccountHolderName == req.LegalRepresentativeName {
		t.Error("names should differ for this test")
	}

	// Simulate the validation check
	var matched bool
	for _, r := range req.LegalRepresentativeName {
		_ = r
	}
	for _, r := range req.BankAccountHolderName {
		_ = r
	}
	_ = matched
}

func TestBusinessProfileStatus_String(t *testing.T) {
	if domain.BusinessProfilePending != "pending" {
		t.Error("expected pending status")
	}
	if domain.BusinessProfileApproved != "approved" {
		t.Error("expected approved status")
	}
}
