package postgres

import (
	"context"

	"smetchic/model"
)

func (repository *LifecycleRepository) ListGlobalTools(ctx context.Context) ([]model.Consumable, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, name, unit_price, resource_minutes, currency, source_url
		FROM global_tools ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tools []model.Consumable
	for rows.Next() {
		var tool model.Consumable
		if err := rows.Scan(&tool.ID, &tool.Name, &tool.UnitPrice, &tool.ResourceMinutes, &tool.Currency, &tool.SourceURL); err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, rows.Err()
}
