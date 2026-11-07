BEGIN;

CREATE TABLE company_economics (
    company_id TEXT PRIMARY KEY REFERENCES companies (id) ON DELETE RESTRICT,
    default_markup_percent NUMERIC(8, 3) NOT NULL DEFAULT 0 CHECK (default_markup_percent >= 0),
    monthly_rent NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (monthly_rent >= 0),
    rent_allocation_method TEXT NOT NULL DEFAULT 'by_production_hours'
        CHECK (rent_allocation_method = 'by_production_hours'),
    monthly_production_hours NUMERIC(14, 4) NOT NULL DEFAULT 0
        CHECK (monthly_production_hours >= 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT company_economics_rent_hours_check
        CHECK (monthly_rent = 0 OR monthly_production_hours > 0)
);

INSERT INTO company_economics (company_id, default_markup_percent, currency)
SELECT company_id, default_markup_percent, currency FROM company_settings;

CREATE TABLE equipment_economics (
    company_id TEXT NOT NULL,
    equipment_id TEXT NOT NULL,
    purchase_price NUMERIC(14, 4) NOT NULL CHECK (purchase_price >= 0),
    residual_value NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (residual_value >= 0),
    useful_life_years NUMERIC(8, 3) NOT NULL CHECK (useful_life_years > 0),
    annual_available_hours NUMERIC(14, 4) NOT NULL CHECK (annual_available_hours > 0),
    consumption_kw NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (consumption_kw >= 0),
    electricity_price_per_kwh NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (electricity_price_per_kwh >= 0),
    annual_maintenance_cost NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (annual_maintenance_cost >= 0),
    annual_repair_cost NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (annual_repair_cost >= 0),
    annual_other_cost NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (annual_other_cost >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, equipment_id),
    CONSTRAINT equipment_economics_residual_check CHECK (residual_value <= purchase_price),
    CONSTRAINT equipment_economics_equipment_fk
        FOREIGN KEY (company_id, equipment_id)
        REFERENCES equipment (company_id, id) ON DELETE RESTRICT
);

CREATE TABLE personnel_rates (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL REFERENCES companies (id) ON DELETE RESTRICT,
    role TEXT NOT NULL CHECK (role IN ('operator', 'programmer', 'setup')),
    monthly_cost NUMERIC(14, 4) NOT NULL CHECK (monthly_cost >= 0),
    monthly_available_hours NUMERIC(14, 4) NOT NULL CHECK (monthly_available_hours > 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, role),
    UNIQUE (company_id, id)
);

CREATE INDEX personnel_rates_company_id_idx ON personnel_rates (company_id);

CREATE TABLE company_overheads (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL REFERENCES companies (id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    category TEXT NOT NULL CHECK (category IN ('utilities', 'other')),
    amount NUMERIC(14, 4) NOT NULL CHECK (amount >= 0),
    period TEXT NOT NULL CHECK (period IN ('monthly', 'annual')),
    allocation_method TEXT NOT NULL CHECK (allocation_method = 'by_production_hours'),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id)
);

CREATE INDEX company_overheads_company_active_idx ON company_overheads (company_id, active);

ALTER TABLE consumables
    ADD COLUMN resource_minutes NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (resource_minutes >= 0),
    ADD COLUMN resource_units NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (resource_units >= 0),
    ADD COLUMN resource_unit_name TEXT NOT NULL DEFAULT '';

COMMIT;
