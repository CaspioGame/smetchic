package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"smetchic/application"
	"smetchic/model"
)

func (repository *LifecycleRepository) UpdateCompany(ctx context.Context, company model.Company, settings model.CompanySettings) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin company profile update: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE companies SET name = $2, updated_at = now() WHERE id = $1`,
		company.ID, company.Name)
	if err != nil {
		return fmt.Errorf("update company profile: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check company profile update: %w", err)
	}
	if affected == 0 {
		return application.ErrNotFound
	}
	result, err = tx.ExecContext(ctx, `
		UPDATE company_settings
		SET currency = $2, default_markup_percent = $3, updated_at = now()
		WHERE company_id = $1`,
		settings.CompanyID, settings.Currency, settings.DefaultMarkupPercent)
	if err != nil {
		return fmt.Errorf("update company settings: %w", err)
	}
	affected, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check company settings update: %w", err)
	}
	if affected == 0 {
		return application.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit company profile update: %w", err)
	}
	return nil
}

func (repository *LifecycleRepository) ListCompanyUsers(ctx context.Context, companyID string) ([]application.CompanyUserRecord, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT users.id, users.email, users.name, users.created_at, users.updated_at, memberships.role
		FROM company_memberships AS memberships
		JOIN users ON users.id = memberships.user_id
		WHERE memberships.company_id = $1
		ORDER BY users.name, users.email`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list company users: %w", err)
	}
	defer rows.Close()
	users := make([]application.CompanyUserRecord, 0)
	for rows.Next() {
		var item application.CompanyUserRecord
		if err := rows.Scan(
			&item.User.ID, &item.User.Email, &item.User.Name,
			&item.User.CreatedAt, &item.User.UpdatedAt, &item.Role,
		); err != nil {
			return nil, fmt.Errorf("scan company user: %w", err)
		}
		users = append(users, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate company users: %w", err)
	}
	return users, nil
}

func (repository *LifecycleRepository) ListEquipmentModels(ctx context.Context) ([]model.EquipmentModel, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, type, manufacturer, name, work_area_x_mm, work_area_y_mm, work_area_z_mm,
		       max_rpm, spindle_power_kw, max_feed_mm_min
		FROM equipment_models ORDER BY manufacturer, name`)
	if err != nil {
		return nil, fmt.Errorf("list global equipment models: %w", err)
	}
	defer rows.Close()
	items := make([]model.EquipmentModel, 0)
	for rows.Next() {
		var item model.EquipmentModel
		if err := rows.Scan(
			&item.ID, &item.Type, &item.Manufacturer, &item.Model,
			&item.WorkAreaX, &item.WorkAreaY, &item.WorkAreaZ, &item.MaxRPM,
			&item.SpindlePower, &item.MaxFeed,
		); err != nil {
			return nil, fmt.Errorf("scan global equipment model: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global equipment models: %w", err)
	}
	return items, nil
}

func (repository *LifecycleRepository) SaveEquipment(ctx context.Context, item model.Equipment) error {
	result, err := repository.db.ExecContext(ctx, `
		INSERT INTO equipment (
			id, company_id, equipment_model_id, name, hourly_rate, currency, status,
			inventory_number, serial_number
		)
		SELECT $1, $2, $3, $4, $5, settings.currency, $6, NULLIF($7, ''), NULLIF($8, '')
		FROM company_settings AS settings WHERE settings.company_id = $2
		ON CONFLICT (id) DO UPDATE SET
			equipment_model_id = EXCLUDED.equipment_model_id,
			name = EXCLUDED.name, hourly_rate = EXCLUDED.hourly_rate,
			currency = EXCLUDED.currency, status = EXCLUDED.status,
			inventory_number = EXCLUDED.inventory_number,
			serial_number = EXCLUDED.serial_number, updated_at = now()
		WHERE equipment.company_id = EXCLUDED.company_id`,
		item.ID, item.CompanyID, item.EquipmentModelID, item.Name, item.MachineRatePerHour,
		item.Status, item.InventoryNumber, item.SerialNumber)
	if err != nil {
		return fmt.Errorf("save company equipment: %w", err)
	}
	return requireAffected(result, "save company equipment")
}

func (repository *LifecycleRepository) DeleteEquipment(ctx context.Context, companyID, equipmentID string) error {
	result, err := repository.db.ExecContext(ctx, `
		DELETE FROM equipment WHERE company_id = $1 AND id = $2`, companyID, equipmentID)
	if err != nil {
		return fmt.Errorf("delete company equipment (it may be used by a saved estimate): %w", err)
	}
	return requireAffected(result, "delete company equipment")
}

func (repository *LifecycleRepository) ListMaterials(ctx context.Context, companyID string) ([]model.Material, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, name, grade, category, density_kg_m3, price_per_kg,
		       currency, created_at, updated_at
		FROM materials WHERE company_id = $1 ORDER BY name, grade`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list company materials: %w", err)
	}
	defer rows.Close()
	items := make([]model.Material, 0)
	for rows.Next() {
		var item model.Material
		if err := rows.Scan(
			&item.ID, &item.CompanyID, &item.Name, &item.Grade, &item.Category,
			&item.Density, &item.PricePerKg, &item.Currency, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan company material: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate company materials: %w", err)
	}
	return items, nil
}

func (repository *LifecycleRepository) SaveMaterial(ctx context.Context, item model.Material) error {
	result, err := repository.db.ExecContext(ctx, `
		INSERT INTO materials (
			id, company_id, name, grade, category, density_kg_m3, price_per_kg, currency
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name, grade = EXCLUDED.grade, category = EXCLUDED.category,
			density_kg_m3 = EXCLUDED.density_kg_m3, price_per_kg = EXCLUDED.price_per_kg,
			currency = EXCLUDED.currency, updated_at = now()
		WHERE materials.company_id = EXCLUDED.company_id`,
		item.ID, item.CompanyID, item.Name, item.Grade, item.Category,
		item.Density, item.PricePerKg, item.Currency)
	if err != nil {
		return fmt.Errorf("save company material: %w", err)
	}
	return requireAffected(result, "save company material")
}

func (repository *LifecycleRepository) DeleteMaterial(ctx context.Context, companyID, materialID string) error {
	result, err := repository.db.ExecContext(ctx, `
		DELETE FROM materials WHERE company_id = $1 AND id = $2`, companyID, materialID)
	if err != nil {
		return fmt.Errorf("delete company material (it may be used by stock blanks): %w", err)
	}
	return requireAffected(result, "delete company material")
}

func (repository *LifecycleRepository) ListGlobalMaterials(ctx context.Context) ([]model.GlobalMaterial, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, name, grade, category, density_kg_m3, reference_price_per_kg,
		       currency, description, created_at, updated_at
		FROM global_materials ORDER BY name, grade`)
	if err != nil {
		return nil, fmt.Errorf("list global materials: %w", err)
	}
	defer rows.Close()
	items := make([]model.GlobalMaterial, 0)
	for rows.Next() {
		var item model.GlobalMaterial
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Grade, &item.Category, &item.Density,
			&item.ReferencePriceKg, &item.Currency, &item.Description,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan global material: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global materials: %w", err)
	}
	return items, nil
}

func (repository *LifecycleRepository) ListConsumables(ctx context.Context, companyID string) ([]model.Consumable, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, category, name, manufacturer, tool_type,
		       COALESCE(diameter_mm, 0), COALESCE(flute_count, 0), tool_material, coating,
		       COALESCE(length_mm, 0), stock_quantity, unit_price, currency, attributes,
		       created_at, updated_at
		FROM consumables WHERE company_id = $1 ORDER BY name, id`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list company consumables: %w", err)
	}
	defer rows.Close()
	items := make([]model.Consumable, 0)
	for rows.Next() {
		var item model.Consumable
		var attributes []byte
		if err := rows.Scan(
			&item.ID, &item.CompanyID, &item.Category, &item.Name,
			&item.Manufacturer, &item.ToolType, &item.DiameterMm, &item.FluteCount,
			&item.ToolMaterial, &item.Coating, &item.LengthMm, &item.StockQuantity,
			&item.UnitPrice, &item.Currency, &attributes, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan company consumable: %w", err)
		}
		item.Attributes = append(item.Attributes[:0], attributes...)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate company consumables: %w", err)
	}
	return items, nil
}

func (repository *LifecycleRepository) SaveConsumable(ctx context.Context, item model.Consumable) error {
	result, err := repository.db.ExecContext(ctx, `
		INSERT INTO consumables (
			id, company_id, category, name, manufacturer, tool_type, diameter_mm,
			flute_count, tool_material, coating, length_mm, stock_quantity,
			unit_price, currency, attributes
		) VALUES (
			$1, $2, $3, $4, $5, $6, NULLIF($7, 0), NULLIF($8, 0), $9, $10,
			NULLIF($11, 0), $12, $13, $14, $15::jsonb
		)
		ON CONFLICT (id) DO UPDATE SET
			category = EXCLUDED.category, name = EXCLUDED.name,
			manufacturer = EXCLUDED.manufacturer, tool_type = EXCLUDED.tool_type,
			diameter_mm = EXCLUDED.diameter_mm, flute_count = EXCLUDED.flute_count,
			tool_material = EXCLUDED.tool_material, coating = EXCLUDED.coating,
			length_mm = EXCLUDED.length_mm, stock_quantity = EXCLUDED.stock_quantity,
			unit_price = EXCLUDED.unit_price, currency = EXCLUDED.currency,
			attributes = EXCLUDED.attributes, updated_at = now()
		WHERE consumables.company_id = EXCLUDED.company_id`,
		item.ID, item.CompanyID, item.Category, item.Name, item.Manufacturer, item.ToolType,
		item.DiameterMm, item.FluteCount, item.ToolMaterial, item.Coating, item.LengthMm,
		item.StockQuantity, item.UnitPrice, item.Currency, string(item.Attributes))
	if err != nil {
		return fmt.Errorf("save company consumable: %w", err)
	}
	return requireAffected(result, "save company consumable")
}

func (repository *LifecycleRepository) DeleteConsumable(ctx context.Context, companyID, consumableID string) error {
	result, err := repository.db.ExecContext(ctx, `
		DELETE FROM consumables WHERE company_id = $1 AND id = $2`, companyID, consumableID)
	if err != nil {
		return fmt.Errorf("delete company consumable: %w", err)
	}
	return requireAffected(result, "delete company consumable")
}

func (repository *LifecycleRepository) ListOrders(ctx context.Context, companyID string) ([]application.OrderRecord, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, company_id, created_by_user_id, COALESCE(project_id, ''),
		       name, description, status, attributes, created_at, updated_at
		FROM orders WHERE company_id = $1 ORDER BY created_at DESC, id`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list company orders: %w", err)
	}
	defer rows.Close()
	items := make([]application.OrderRecord, 0)
	for rows.Next() {
		item, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("scan company order: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate company orders: %w", err)
	}
	return items, nil
}

func (repository *LifecycleRepository) CreateOrder(ctx context.Context, order application.OrderRecord) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO orders (
			id, company_id, created_by_user_id, project_id, name, description, status, attributes
		) VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8::jsonb)`,
		order.ID, order.CompanyID, order.CreatedByID, order.ProjectID,
		order.Name, order.Description, order.Status, string(order.Attributes))
	if err != nil {
		return fmt.Errorf("create company order: %w", err)
	}
	return nil
}

func (repository *LifecycleRepository) UpdateOrder(ctx context.Context, order application.OrderRecord) error {
	result, err := repository.db.ExecContext(ctx, `
		UPDATE orders SET project_id = NULLIF($3, ''), name = $4, description = $5,
			status = $6, updated_at = now()
		WHERE company_id = $1 AND id = $2`,
		order.CompanyID, order.ID, order.ProjectID, order.Name, order.Description, order.Status)
	if err != nil {
		return fmt.Errorf("update company order: %w", err)
	}
	return requireAffected(result, "update company order")
}

func (repository *LifecycleRepository) GetOrder(ctx context.Context, companyID, orderID string) (application.OrderRecord, error) {
	item, err := scanOrder(repository.db.QueryRowContext(ctx, `
		SELECT id, company_id, created_by_user_id, COALESCE(project_id, ''),
		       name, description, status, attributes, created_at, updated_at
		FROM orders WHERE company_id = $1 AND id = $2`, companyID, orderID))
	if err != nil {
		return application.OrderRecord{}, notFound("get company order", err)
	}
	return item, nil
}

type workspaceRowScanner interface {
	Scan(...any) error
}

func scanOrder(row workspaceRowScanner) (application.OrderRecord, error) {
	var item application.OrderRecord
	var attributes []byte
	err := row.Scan(
		&item.ID, &item.CompanyID, &item.CreatedByID, &item.ProjectID,
		&item.Name, &item.Description, &item.Status, &attributes,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return application.OrderRecord{}, err
	}
	item.Attributes = json.RawMessage(attributes)
	return item, nil
}

func requireAffected(result sql.Result, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", operation, err)
	}
	if affected == 0 {
		return application.ErrNotFound
	}
	return nil
}
