package model

type ProductionCostBreakdown struct {
	Quantity                  int      `json:"quantity"`
	Currency                  string   `json:"currency"`
	MaterialCost              float64  `json:"material_cost"`
	MaterialPriceSource       string   `json:"material_price_source"`
	EquipmentEconomicsApplied bool     `json:"equipment_economics_applied"`
	MachineCost               float64  `json:"machine_cost"`
	DepreciationCost          float64  `json:"depreciation_cost"`
	ElectricityCost           float64  `json:"electricity_cost"`
	MaintenanceCost           float64  `json:"maintenance_cost"`
	RepairCost                float64  `json:"repair_cost"`
	OtherEquipmentCost        float64  `json:"other_equipment_cost"`
	OperatorCost              float64  `json:"operator_cost"`
	ProgrammingCost           float64  `json:"programming_cost"`
	ToolingCost               float64  `json:"tooling_cost"`
	SetupCost                 float64  `json:"setup_cost"`
	RentCost                  float64  `json:"rent_cost"`
	UtilitiesCost             float64  `json:"utilities_cost"`
	OtherOverheadCost         float64  `json:"other_overhead_cost"`
	BatchCost                 float64  `json:"batch_cost"`
	PerPieceCost              float64  `json:"per_piece_cost"`
	TotalCost                 float64  `json:"total_cost"`
	Profit                    float64  `json:"profit"`
	MarkupPercent             float64  `json:"markup_percent"`
	SellingPrice              float64  `json:"selling_price"`
	SellingPricePerPiece      float64  `json:"selling_price_per_piece"`
	Warnings                  []string `json:"warnings,omitempty"`
}
