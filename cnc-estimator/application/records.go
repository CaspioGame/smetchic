package application

import (
	"encoding/json"
	"time"

	"smetchic/model"
)

type ProjectRecord struct {
	ID          string
	CompanyID   string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PartRecord struct {
	ID          string
	CompanyID   string
	ProjectID   string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PartRevisionRecord struct {
	ID             string
	CompanyID      string
	PartID         string
	RevisionNumber int
	SourceFormat   string
	SourceFilename string
	SourceContent  string
	SourceHash     string
	ParserVersion  string
	ParsedPart     model.Part
	CreatedAt      time.Time
}

type EstimateRecord struct {
	ID             string
	CompanyID      string
	PartRevisionID string
	Name           string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type EstimateVersionRecord struct {
	ID               string
	CompanyID        string
	EstimateID       string
	PartRevisionID   string
	VersionNumber    int
	EquipmentID      string
	EquipmentModelID string
	InputSnapshot    json.RawMessage
	SettingsSnapshot json.RawMessage
	ResultSnapshot   json.RawMessage
	MachineTimeHours float64
	HourlyRate       float64
	MachineCost      float64
	MarkupPercent    float64
	FinalPrice       float64
	Currency         string
	CreatedAt        time.Time
}

type RevisionUpload struct {
	PartName        string
	PartDescription string
	SourceFilename  string
	SourceContent   []byte
}

type RevisionEstimate struct {
	Project   ProjectRecord
	Part      PartRecord
	Revision  PartRevisionRecord
	Estimates []EstimateRecord
}

type EstimateHistory struct {
	Estimate EstimateRecord
	Versions []EstimateVersionRecord
}

type EstimateVersionDetail struct {
	Estimate EstimateRecord
	Version  EstimateVersionRecord
}
