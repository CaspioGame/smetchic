package model

import (
	"strings"
	"testing"
)

func TestCompanyValidation(t *testing.T) {
	tests := []struct {
		name    string
		company Company
		wantErr string
	}{
		{
			name:    "valid",
			company: Company{ID: "company-1", Name: "Example Company"},
		},
		{
			name:    "missing ID",
			company: Company{Name: "Example Company"},
			wantErr: "company ID",
		},
		{
			name:    "missing name",
			company: Company{ID: "company-1"},
			wantErr: "company name",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.company.Validate()
			assertValidation(t, err, test.wantErr)
		})
	}
}

func TestUserValidation(t *testing.T) {
	tests := []struct {
		name    string
		user    User
		wantErr string
	}{
		{
			name: "valid",
			user: User{ID: "user-1", Email: "person@example.com", Name: "Person"},
		},
		{
			name:    "invalid email",
			user:    User{ID: "user-1", Email: "invalid", Name: "Person"},
			wantErr: "user email",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.user.Validate()
			assertValidation(t, err, test.wantErr)
		})
	}
}

func TestCompanyMembershipValidation(t *testing.T) {
	validMembership := CompanyMembership{
		ID: "membership-1", CompanyID: "company-1", UserID: "user-1", Role: CompanyRoleOwner,
	}
	if err := validMembership.Validate(); err != nil {
		t.Fatalf("valid membership rejected: %v", err)
	}

	invalidRole := validMembership
	invalidRole.Role = CompanyRole("guest")
	assertValidation(t, invalidRole.Validate(), "unsupported company role")

	missingCompany := validMembership
	missingCompany.CompanyID = ""
	assertValidation(t, missingCompany.Validate(), "company ID")
}

func TestCompanySettingsValidation(t *testing.T) {
	validSettings := CompanySettings{
		CompanyID: "company-1", Currency: "RUB", DefaultMarkupPercent: 50,
	}
	if err := validSettings.Validate(); err != nil {
		t.Fatalf("valid company settings rejected: %v", err)
	}

	invalidMarkup := validSettings
	invalidMarkup.DefaultMarkupPercent = -1
	assertValidation(t, invalidMarkup.Validate(), "default markup")

	missingCurrency := validSettings
	missingCurrency.Currency = ""
	assertValidation(t, missingCurrency.Validate(), "currency")
}

func TestEquipmentModelValidation(t *testing.T) {
	validModel := EquipmentModel{
		ID: "model-1", Type: EquipmentMilling, Manufacturer: "Maker", Model: "Mill",
	}
	if err := validModel.Validate(); err != nil {
		t.Fatalf("valid equipment model rejected: %v", err)
	}

	invalidType := validModel
	invalidType.Type = EquipmentType("unknown")
	assertValidation(t, invalidType.Validate(), "unsupported equipment type")

	negativeWorkArea := validModel
	negativeWorkArea.WorkAreaX = -1
	assertValidation(t, negativeWorkArea.Validate(), "work area X")
}

func TestEquipmentValidation(t *testing.T) {
	validEquipment := Equipment{
		ID: "equipment-1", CompanyID: "company-1", EquipmentModelID: "model-1",
		Name: "Mill 1", Status: EquipmentStatusActive, MachineRatePerHour: 1500,
	}
	if err := validEquipment.Validate(); err != nil {
		t.Fatalf("valid equipment rejected: %v", err)
	}

	invalidStatus := validEquipment
	invalidStatus.Status = EquipmentStatus("broken")
	assertValidation(t, invalidStatus.Validate(), "unsupported equipment status")

	invalidRate := validEquipment
	invalidRate.MachineRatePerHour = 0
	assertValidation(t, invalidRate.Validate(), "rate per hour")
}

func assertValidation(t *testing.T, err error, wantSubstring string) {
	t.Helper()
	if wantSubstring == "" {
		if err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected validation error containing %q", wantSubstring)
	}
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("validation error = %q, want substring %q", err, wantSubstring)
	}
}
