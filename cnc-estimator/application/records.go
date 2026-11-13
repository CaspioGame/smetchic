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
	ID               string
	CompanyID        string
	PartID           string
	ProjectID        string
	ProjectName      string
	OrderID          string
	PartRevisionID   string
	Number           string
	Name             string
	Quantity         int
	Status           string
	WorkingInput     json.RawMessage
	CurrentVersionID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

const (
	EstimateStatusDraft      = "draft"
	EstimateStatusCalculated = "calculated"
	EstimateStatusApproved   = "approved"
	EstimateStatusArchived   = "archived"
)

type EstimateValueSource string

const (
	EstimateSourceAutomatic    EstimateValueSource = "automatic"
	EstimateSourceCatalog      EstimateValueSource = "catalog"
	EstimateSourceCompanyData  EstimateValueSource = "company_data"
	EstimateSourceUserOverride EstimateValueSource = "user_override"
	EstimateSourceAIOverride   EstimateValueSource = "ai_override"
)

type EstimateOverride struct {
	Value  float64             `json:"value"`
	Source EstimateValueSource `json:"source"`
	Reason string              `json:"reason,omitempty"`
}

type EstimateInput struct {
	Name              string              `json:"name"`
	ProjectID         string              `json:"project_id,omitempty"`
	OrderID           string              `json:"order_id,omitempty"`
	PartRevisionID    string              `json:"part_revision_id"`
	Quantity          int                 `json:"quantity"`
	QuantitySource    EstimateValueSource `json:"quantity_source"`
	EquipmentID       string              `json:"equipment_id"`
	EquipmentSource   EstimateValueSource `json:"equipment_source"`
	MaterialID        string              `json:"material_id,omitempty"`
	GlobalMaterialID  string              `json:"global_material_id,omitempty"`
	MaterialWeightKg  float64             `json:"material_weight_kg,omitempty"`
	StockBlankID      string              `json:"stock_blank_id,omitempty"`
	MaterialSource    EstimateValueSource `json:"material_source,omitempty"`
	StockBlankSource  EstimateValueSource `json:"stock_blank_source,omitempty"`
	ConsumableID      string              `json:"consumable_id,omitempty"`
	GlobalToolID      string              `json:"global_tool_id,omitempty"`
	ToolSource        EstimateValueSource `json:"tool_source,omitempty"`
	ToolUsageMinutes  float64             `json:"tool_usage_minutes"`
	ToolUsageSource   EstimateValueSource `json:"tool_usage_source,omitempty"`
	ToolUsageReason   string              `json:"tool_usage_reason,omitempty"`
	SetupTimeMinutes  float64             `json:"setup_time_minutes"`
	SetupSource       EstimateValueSource `json:"setup_source,omitempty"`
	SetupReason       string              `json:"setup_reason,omitempty"`
	ProgrammingHours  float64             `json:"programming_hours"`
	ProgrammingSource EstimateValueSource `json:"programming_source,omitempty"`
	ProgrammingReason string              `json:"programming_reason,omitempty"`
	MarkupPercent     float64             `json:"markup_percent"`
	MarkupSource      EstimateValueSource `json:"markup_source"`
	MarkupReason      string              `json:"markup_reason,omitempty"`
	MachineTime       *EstimateOverride   `json:"machine_time,omitempty"`
	ProjectLabor      *model.ProjectLabor `json:"project_labor,omitempty"`
	Modules           EstimateModules     `json:"modules"`
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

type EstimateDocument struct {
	Estimate EstimateRecord
	Project  ProjectRecord
	Part     PartRecord
	Revision PartRevisionRecord
	Order    OrderRecord
	Versions []EstimateVersionRecord
}

type EstimateVersionDetail struct {
	Estimate EstimateRecord
	Version  EstimateVersionRecord
}
