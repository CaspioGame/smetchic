BEGIN;

CREATE TABLE global_materials (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    grade TEXT NOT NULL CHECK (length(btrim(grade)) > 0),
    category TEXT NOT NULL CHECK (category IN (
        'aluminum', 'steel', 'stainless_steel', 'brass', 'copper', 'plastic', 'other'
    )),
    density_kg_m3 NUMERIC(14, 4) NOT NULL CHECK (density_kg_m3 > 0),
    reference_price_per_kg NUMERIC(14, 4) NOT NULL CHECK (reference_price_per_kg >= 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO global_materials (id, name, grade, category, density_kg_m3, reference_price_per_kg, currency, description)
VALUES
    ('ref-aluminum-6061', 'Алюминий', '6061', 'aluminum', 2700, 500, 'RUB', 'Демонстрационная ориентировочная цена. Уточняйте у поставщика.'),
    ('ref-steel-45', 'Сталь конструкционная', '45', 'steel', 7850, 180, 'RUB', 'Демонстрационная ориентировочная цена. Уточняйте у поставщика.'),
    ('ref-stainless-304', 'Нержавеющая сталь', 'AISI 304', 'stainless_steel', 8000, 520, 'RUB', 'Демонстрационная ориентировочная цена. Уточняйте у поставщика.'),
    ('ref-brass-l63', 'Латунь', 'Л63', 'brass', 8500, 700, 'RUB', 'Демонстрационная ориентировочная цена. Уточняйте у поставщика.'),
    ('ref-copper-m1', 'Медь', 'М1', 'copper', 8960, 1100, 'RUB', 'Демонстрационная ориентировочная цена. Уточняйте у поставщика.'),
    ('ref-plastic-abs', 'Пластик', 'ABS', 'plastic', 1050, 350, 'RUB', 'Демонстрационная ориентировочная цена. Уточняйте у поставщика.');

CREATE TABLE consumables (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL,
    category TEXT NOT NULL CHECK (length(btrim(category)) > 0),
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    manufacturer TEXT NOT NULL DEFAULT '',
    tool_type TEXT NOT NULL DEFAULT '',
    diameter_mm NUMERIC(14, 4),
    flute_count INTEGER,
    tool_material TEXT NOT NULL DEFAULT '',
    coating TEXT NOT NULL DEFAULT '',
    length_mm NUMERIC(14, 4),
    stock_quantity INTEGER NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),
    unit_price NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (unit_price >= 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(attributes) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id),
    CONSTRAINT consumables_company_fk
        FOREIGN KEY (company_id) REFERENCES companies (id) ON DELETE RESTRICT,
    CONSTRAINT consumables_dimensions_check CHECK (
        (diameter_mm IS NULL OR diameter_mm > 0)
        AND (flute_count IS NULL OR flute_count > 0)
        AND (length_mm IS NULL OR length_mm > 0)
    )
);

CREATE INDEX consumables_company_id_idx ON consumables (company_id);

CREATE TABLE orders (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL,
    created_by_user_id TEXT NOT NULL,
    project_id TEXT,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'in_progress', 'completed', 'cancelled')),
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(attributes) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id),
    CONSTRAINT orders_company_fk
        FOREIGN KEY (company_id) REFERENCES companies (id) ON DELETE RESTRICT,
    CONSTRAINT orders_creator_membership_fk
        FOREIGN KEY (company_id, created_by_user_id)
        REFERENCES company_memberships (company_id, user_id) ON DELETE RESTRICT,
    CONSTRAINT orders_project_company_fk
        FOREIGN KEY (company_id, project_id)
        REFERENCES projects (company_id, id) ON DELETE RESTRICT
);

CREATE INDEX orders_company_created_idx ON orders (company_id, created_at DESC);
CREATE INDEX orders_created_by_user_id_idx ON orders (created_by_user_id);
CREATE INDEX orders_project_id_idx ON orders (project_id) WHERE project_id IS NOT NULL;

COMMIT;
