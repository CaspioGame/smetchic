package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"smetchic/model"
)

func (repository *LifecycleRepository) GetCompanyEconomics(ctx context.Context, companyID string) (model.CompanyEconomics, error) {
	var item model.CompanyEconomics
	err := repository.db.QueryRowContext(ctx, `
		SELECT company_id, default_markup_percent, monthly_rent, rent_allocation_method,
		       monthly_production_hours, currency
		FROM company_economics WHERE company_id = $1`, companyID).Scan(
		&item.CompanyID, &item.DefaultMarkupPercent, &item.MonthlyRent,
		&item.RentAllocationMethod, &item.MonthlyProductionHours, &item.Currency)
	if err != nil {
		return model.CompanyEconomics{}, notFound("get company production economics", err)
	}
	return item, nil
}

func (repository *LifecycleRepository) SaveCompanyEconomics(ctx context.Context, item model.CompanyEconomics) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin company economics update: %w", err)
	}
	defer tx.Rollback()

	settingsResult, err := tx.ExecContext(ctx, `
		UPDATE company_settings
		SET currency = $2, default_markup_percent = $3, updated_at = now()
		WHERE company_id = $1`, item.CompanyID, item.Currency, item.DefaultMarkupPercent)
	if err != nil {
		return fmt.Errorf("update company settings from production economics: %w", err)
	}
	if err := requireAffected(settingsResult, "update company settings from production economics"); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO company_economics (
			company_id, default_markup_percent, monthly_rent, rent_allocation_method,
			monthly_production_hours, currency
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (company_id) DO UPDATE SET
			default_markup_percent = EXCLUDED.default_markup_percent,
			monthly_rent = EXCLUDED.monthly_rent,
			rent_allocation_method = EXCLUDED.rent_allocation_method,
			monthly_production_hours = EXCLUDED.monthly_production_hours,
			currency = EXCLUDED.currency, updated_at = now()`,
		item.CompanyID, item.DefaultMarkupPercent, item.MonthlyRent, item.RentAllocationMethod,
		item.MonthlyProductionHours, item.Currency)
	if err != nil {
		return fmt.Errorf("save company production economics: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit company production economics: %w", err)
	}
	return nil
}

func (repository *LifecycleRepository) ListEquipmentEconomics(ctx context.Context, companyID string) ([]model.EquipmentEconomics, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT company_id, equipment_id, purchase_price, residual_value, useful_life_years,
		       annual_available_hours, consumption_kw, electricity_price_per_kwh,
		       annual_maintenance_cost, annual_repair_cost, annual_other_cost
		FROM equipment_economics WHERE company_id = $1 ORDER BY equipment_id`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list equipment economics: %w", err)
	}
	defer rows.Close()
	items := make([]model.EquipmentEconomics, 0)
	for rows.Next() {
		var item model.EquipmentEconomics
		if err := rows.Scan(
			&item.CompanyID, &item.EquipmentID, &item.PurchasePrice, &item.ResidualValue,
			&item.UsefulLifeYears, &item.AnnualAvailableHours, &item.ConsumptionKW,
			&item.ElectricityPricePerKWh, &item.AnnualMaintenanceCost,
			&item.AnnualRepairCost, &item.AnnualOtherCost,
		); err != nil {
			return nil, fmt.Errorf("scan equipment economics: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate equipment economics: %w", err)
	}
	return items, nil
}

func (repository *LifecycleRepository) GetEquipmentEconomics(ctx context.Context, companyID, equipmentID string) (*model.EquipmentEconomics, error) {
	var item model.EquipmentEconomics
	err := repository.db.QueryRowContext(ctx, `
		SELECT company_id, equipment_id, purchase_price, residual_value, useful_life_years,
		       annual_available_hours, consumption_kw, electricity_price_per_kwh,
		       annual_maintenance_cost, annual_repair_cost, annual_other_cost
		FROM equipment_economics WHERE company_id = $1 AND equipment_id = $2`,
		companyID, equipmentID).Scan(
		&item.CompanyID, &item.EquipmentID, &item.PurchasePrice, &item.ResidualValue,
		&item.UsefulLifeYears, &item.AnnualAvailableHours, &item.ConsumptionKW,
		&item.ElectricityPricePerKWh, &item.AnnualMaintenanceCost,
		&item.AnnualRepairCost, &item.AnnualOtherCost)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get equipment economics: %w", err)
	}
	return &item, nil
}

func (repository *LifecycleRepository) SaveEquipmentEconomics(ctx context.Context, item model.EquipmentEconomics) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO equipment_economics (
			company_id, equipment_id, purchase_price, residual_value, useful_life_years,
			annual_available_hours, consumption_kw, electricity_price_per_kwh,
			annual_maintenance_cost, annual_repair_cost, annual_other_cost
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (company_id, equipment_id) DO UPDATE SET
			purchase_price = EXCLUDED.purchase_price, residual_value = EXCLUDED.residual_value,
			useful_life_years = EXCLUDED.useful_life_years,
			annual_available_hours = EXCLUDED.annual_available_hours,
			consumption_kw = EXCLUDED.consumption_kw,
			electricity_price_per_kwh = EXCLUDED.electricity_price_per_kwh,
			annual_maintenance_cost = EXCLUDED.annual_maintenance_cost,
			annual_repair_cost = EXCLUDED.annual_repair_cost,
			annual_other_cost = EXCLUDED.annual_other_cost, updated_at = now()`,
		item.CompanyID, item.EquipmentID, item.PurchasePrice, item.ResidualValue,
		item.UsefulLifeYears, item.AnnualAvailableHours, item.ConsumptionKW,
		item.ElectricityPricePerKWh, item.AnnualMaintenanceCost,
		item.AnnualRepairCost, item.AnnualOtherCost)
	if err != nil {
		return fmt.Errorf("save equipment economics: %w", err)
	}
	return nil
}

func (repository *LifecycleRepository) DeleteEquipmentEconomics(ctx context.Context, companyID, equipmentID string) error {
	result, err := repository.db.ExecContext(ctx, `
		DELETE FROM equipment_economics WHERE company_id = $1 AND equipment_id = $2`,
		companyID, equipmentID)
	if err != nil {
		return fmt.Errorf("delete equipment economics: %w", err)
	}
	return requireAffected(result, "delete equipment economics")
}

func (repository *LifecycleRepository) ListPersonnelRates(ctx context.Context, companyID string) ([]model.PersonnelRate, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, role, monthly_cost, monthly_available_hours, currency
		FROM personnel_rates WHERE company_id = $1 ORDER BY role`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list personnel rates: %w", err)
	}
	defer rows.Close()
	items := make([]model.PersonnelRate, 0)
	for rows.Next() {
		var item model.PersonnelRate
		if err := rows.Scan(
			&item.ID, &item.CompanyID, &item.Role, &item.MonthlyCost,
			&item.MonthlyAvailableHours, &item.Currency,
		); err != nil {
			return nil, fmt.Errorf("scan personnel rate: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate personnel rates: %w", err)
	}
	return items, nil
}

func (repository *LifecycleRepository) SavePersonnelRate(ctx context.Context, item model.PersonnelRate) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO personnel_rates (
			id, company_id, role, monthly_cost, monthly_available_hours, currency
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (company_id, role) DO UPDATE SET
			monthly_cost = EXCLUDED.monthly_cost,
			monthly_available_hours = EXCLUDED.monthly_available_hours,
			currency = EXCLUDED.currency, updated_at = now()`,
		item.ID, item.CompanyID, item.Role, item.MonthlyCost,
		item.MonthlyAvailableHours, item.Currency)
	if err != nil {
		return fmt.Errorf("save personnel rate: %w", err)
	}
	return nil
}

func (repository *LifecycleRepository) DeletePersonnelRate(ctx context.Context, companyID, rateID string) error {
	result, err := repository.db.ExecContext(ctx, `
		DELETE FROM personnel_rates WHERE company_id = $1 AND id = $2`, companyID, rateID)
	if err != nil {
		return fmt.Errorf("delete personnel rate: %w", err)
	}
	return requireAffected(result, "delete personnel rate")
}

func (repository *LifecycleRepository) ListCompanyOverheads(ctx context.Context, companyID string) ([]model.CompanyOverhead, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, name, category, amount, period, allocation_method,
		       active, currency
		FROM company_overheads WHERE company_id = $1 ORDER BY name, id`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list company overheads: %w", err)
	}
	defer rows.Close()
	items := make([]model.CompanyOverhead, 0)
	for rows.Next() {
		var item model.CompanyOverhead
		if err := rows.Scan(
			&item.ID, &item.CompanyID, &item.Name, &item.Category, &item.Amount,
			&item.Period, &item.AllocationMethod, &item.Active, &item.Currency,
		); err != nil {
			return nil, fmt.Errorf("scan company overhead: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate company overheads: %w", err)
	}
	return items, nil
}

func (repository *LifecycleRepository) SaveCompanyOverhead(ctx context.Context, item model.CompanyOverhead) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO company_overheads (
			id, company_id, name, category, amount, period, allocation_method,
			active, currency
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (company_id, id) DO UPDATE SET
			name = EXCLUDED.name, category = EXCLUDED.category,
			amount = EXCLUDED.amount, period = EXCLUDED.period,
			allocation_method = EXCLUDED.allocation_method,
			active = EXCLUDED.active, currency = EXCLUDED.currency, updated_at = now()`,
		item.ID, item.CompanyID, strings.TrimSpace(item.Name), item.Category,
		item.Amount, item.Period, item.AllocationMethod, item.Active, item.Currency)
	if err != nil {
		return fmt.Errorf("save company overhead: %w", err)
	}
	return nil
}

func (repository *LifecycleRepository) DeleteCompanyOverhead(ctx context.Context, companyID, overheadID string) error {
	result, err := repository.db.ExecContext(ctx, `
		DELETE FROM company_overheads WHERE company_id = $1 AND id = $2`, companyID, overheadID)
	if err != nil {
		return fmt.Errorf("delete company overhead: %w", err)
	}
	return requireAffected(result, "delete company overhead")
}

func (repository *LifecycleRepository) ListStockBlanks(ctx context.Context, companyID string) ([]model.StockBlank, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, material_id, name, form,
		       COALESCE(length, 0), COALESCE(width, 0), COALESCE(height, 0),
		       COALESCE(diameter, 0), price_per_piece, currency, created_at, updated_at
		FROM stock_blanks WHERE company_id = $1 ORDER BY name, id`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list company stock blanks: %w", err)
	}
	defer rows.Close()
	items := make([]model.StockBlank, 0)
	for rows.Next() {
		var item model.StockBlank
		if err := rows.Scan(
			&item.ID, &item.CompanyID, &item.MaterialID, &item.Name, &item.Shape,
			&item.Length, &item.Width, &item.Height, &item.Diameter, &item.PricePerPiece,
			&item.Currency, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan company stock blank: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate company stock blanks: %w", err)
	}
	return items, nil
}

func (repository *LifecycleRepository) SaveStockBlank(ctx context.Context, item model.StockBlank) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO stock_blanks (
			id, company_id, material_id, name, form, width, length, height,
			diameter, price_per_piece, currency
		) VALUES (
			$1, $2, $3, $4, $5, NULLIF($6, 0), $7, NULLIF($8, 0),
			NULLIF($9, 0), $10, $11
		)
		ON CONFLICT (company_id, id) DO UPDATE SET
			material_id = EXCLUDED.material_id, name = EXCLUDED.name, form = EXCLUDED.form,
			width = EXCLUDED.width, length = EXCLUDED.length, height = EXCLUDED.height,
			diameter = EXCLUDED.diameter, price_per_piece = EXCLUDED.price_per_piece,
			currency = EXCLUDED.currency, updated_at = now()`,
		item.ID, item.CompanyID, item.MaterialID, item.Name, item.Shape,
		item.Width, item.Length, item.Height, item.Diameter, item.PricePerPiece, item.Currency)
	if err != nil {
		return fmt.Errorf("save company stock blank: %w", err)
	}
	return nil
}

func (repository *LifecycleRepository) DeleteStockBlank(ctx context.Context, companyID, stockBlankID string) error {
	result, err := repository.db.ExecContext(ctx, `
		DELETE FROM stock_blanks WHERE company_id = $1 AND id = $2`, companyID, stockBlankID)
	if err != nil {
		return fmt.Errorf("delete company stock blank: %w", err)
	}
	return requireAffected(result, "delete company stock blank")
}
