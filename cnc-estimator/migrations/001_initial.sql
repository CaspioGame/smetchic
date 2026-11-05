BEGIN;

CREATE TABLE companies (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    email TEXT NOT NULL CHECK (length(btrim(email)) > 0),
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_uq ON users (lower(email));

CREATE TABLE company_memberships (
    company_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'manager', 'operator')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, user_id),
    CONSTRAINT company_memberships_company_fk
        FOREIGN KEY (company_id) REFERENCES companies (id) ON DELETE RESTRICT,
    CONSTRAINT company_memberships_user_fk
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT
);

CREATE INDEX company_memberships_user_id_idx ON company_memberships (user_id);

CREATE TABLE company_settings (
    company_id TEXT PRIMARY KEY,
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    default_markup_percent NUMERIC(8, 3) NOT NULL DEFAULT 0
        CHECK (default_markup_percent >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT company_settings_company_fk
        FOREIGN KEY (company_id) REFERENCES companies (id) ON DELETE RESTRICT
);

CREATE TABLE equipment_models (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    manufacturer TEXT NOT NULL CHECK (length(btrim(manufacturer)) > 0),
    type TEXT NOT NULL CHECK (type IN (
        'milling', 'turning', 'laser', '3d_printer', 'welding', 'press', 'other'
    )),
    description TEXT NOT NULL DEFAULT '',
    work_area_x_mm NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (work_area_x_mm >= 0),
    work_area_y_mm NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (work_area_y_mm >= 0),
    work_area_z_mm NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (work_area_z_mm >= 0),
    max_rpm NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (max_rpm >= 0),
    spindle_power_kw NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (spindle_power_kw >= 0),
    max_feed_mm_min NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (max_feed_mm_min >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (manufacturer, name)
);

CREATE TABLE equipment (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL,
    equipment_model_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    hourly_rate NUMERIC(14, 4) NOT NULL CHECK (hourly_rate > 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    status TEXT NOT NULL CHECK (status IN ('active', 'maintenance', 'inactive')),
    inventory_number TEXT,
    serial_number TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id),
    CONSTRAINT equipment_company_fk
        FOREIGN KEY (company_id) REFERENCES companies (id) ON DELETE RESTRICT,
    CONSTRAINT equipment_model_fk
        FOREIGN KEY (equipment_model_id) REFERENCES equipment_models (id) ON DELETE RESTRICT
);

CREATE INDEX equipment_company_id_idx ON equipment (company_id);

CREATE TABLE materials (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    grade TEXT NOT NULL CHECK (length(btrim(grade)) > 0),
    category TEXT NOT NULL CHECK (category IN (
        'aluminum', 'steel', 'stainless_steel', 'brass', 'copper', 'plastic', 'other'
    )),
    density_kg_m3 NUMERIC(14, 4) NOT NULL CHECK (density_kg_m3 > 0),
    price_per_kg NUMERIC(14, 4) NOT NULL CHECK (price_per_kg >= 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id),
    CONSTRAINT materials_company_fk
        FOREIGN KEY (company_id) REFERENCES companies (id) ON DELETE RESTRICT
);

CREATE INDEX materials_company_id_idx ON materials (company_id);

CREATE TABLE stock_blanks (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL,
    material_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    form TEXT NOT NULL CHECK (form IN ('rectangular', 'round')),
    width NUMERIC(14, 4),
    length NUMERIC(14, 4) NOT NULL,
    height NUMERIC(14, 4),
    diameter NUMERIC(14, 4),
    price_per_piece NUMERIC(14, 4) NOT NULL DEFAULT 0 CHECK (price_per_piece >= 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id),
    CONSTRAINT stock_blanks_company_fk
        FOREIGN KEY (company_id) REFERENCES companies (id) ON DELETE RESTRICT,
    CONSTRAINT stock_blanks_material_company_fk
        FOREIGN KEY (company_id, material_id)
        REFERENCES materials (company_id, id) ON DELETE RESTRICT,
    CONSTRAINT stock_blanks_dimensions_check CHECK (
        (
            form = 'rectangular'
            AND width IS NOT NULL AND width > 0
            AND length > 0
            AND height IS NOT NULL AND height > 0
            AND diameter IS NULL
        )
        OR
        (
            form = 'round'
            AND width IS NULL
            AND length > 0
            AND height IS NULL
            AND diameter IS NOT NULL AND diameter > 0
        )
    )
);

CREATE INDEX stock_blanks_company_id_idx ON stock_blanks (company_id);
CREATE INDEX stock_blanks_material_id_idx ON stock_blanks (material_id);

CREATE TABLE projects (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id),
    CONSTRAINT projects_company_fk
        FOREIGN KEY (company_id) REFERENCES companies (id) ON DELETE RESTRICT
);

CREATE INDEX projects_company_id_idx ON projects (company_id);

CREATE TABLE parts (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id),
    CONSTRAINT parts_project_company_fk
        FOREIGN KEY (company_id, project_id)
        REFERENCES projects (company_id, id) ON DELETE RESTRICT
);

CREATE INDEX parts_company_id_idx ON parts (company_id);
CREATE INDEX parts_project_id_idx ON parts (project_id);

CREATE TABLE part_revisions (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL,
    part_id TEXT NOT NULL,
    revision_number INTEGER NOT NULL CHECK (revision_number > 0),
    source_format TEXT NOT NULL CHECK (length(btrim(source_format)) > 0),
    source_filename TEXT NOT NULL CHECK (length(btrim(source_filename)) > 0),
    source_content TEXT NOT NULL,
    source_hash TEXT NOT NULL CHECK (source_hash ~ '^[0-9a-f]{64}$'),
    parser_version TEXT,
    parsed_part_json JSONB NOT NULL CHECK (jsonb_typeof(parsed_part_json) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id),
    UNIQUE (company_id, part_id, revision_number),
    CONSTRAINT part_revisions_part_company_fk
        FOREIGN KEY (company_id, part_id)
        REFERENCES parts (company_id, id) ON DELETE RESTRICT
);

CREATE INDEX part_revisions_company_id_idx ON part_revisions (company_id);
CREATE INDEX part_revisions_part_id_idx ON part_revisions (part_id);

CREATE TABLE estimates (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL,
    part_revision_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id),
    CONSTRAINT estimates_revision_company_fk
        FOREIGN KEY (company_id, part_revision_id)
        REFERENCES part_revisions (company_id, id) ON DELETE RESTRICT
);

CREATE INDEX estimates_company_id_idx ON estimates (company_id);
CREATE INDEX estimates_part_revision_id_idx ON estimates (part_revision_id);

CREATE TABLE estimate_versions (
    id TEXT PRIMARY KEY CHECK (length(btrim(id)) > 0),
    company_id TEXT NOT NULL,
    estimate_id TEXT NOT NULL,
    version_number INTEGER NOT NULL CHECK (version_number > 0),
    equipment_id TEXT,
    equipment_model_id TEXT,
    input_snapshot JSONB NOT NULL CHECK (jsonb_typeof(input_snapshot) = 'object'),
    settings_snapshot JSONB NOT NULL CHECK (jsonb_typeof(settings_snapshot) = 'object'),
    result_snapshot JSONB NOT NULL CHECK (jsonb_typeof(result_snapshot) = 'object'),
    machine_time_hours NUMERIC(14, 4) NOT NULL CHECK (machine_time_hours >= 0),
    hourly_rate NUMERIC(14, 4) NOT NULL CHECK (hourly_rate >= 0),
    machine_cost NUMERIC(14, 4) NOT NULL CHECK (machine_cost >= 0),
    markup_percent NUMERIC(8, 3) NOT NULL CHECK (markup_percent >= 0),
    final_price NUMERIC(14, 4) NOT NULL CHECK (final_price >= 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id),
    UNIQUE (company_id, estimate_id, version_number),
    CONSTRAINT estimate_versions_estimate_company_fk
        FOREIGN KEY (company_id, estimate_id)
        REFERENCES estimates (company_id, id) ON DELETE RESTRICT
);

CREATE INDEX estimate_versions_company_id_idx ON estimate_versions (company_id);
CREATE INDEX estimate_versions_estimate_id_idx ON estimate_versions (estimate_id);

CREATE FUNCTION prevent_estimate_version_mutation() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'estimate_versions are immutable';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER estimate_versions_immutable
    BEFORE UPDATE OR DELETE ON estimate_versions
    FOR EACH ROW EXECUTE FUNCTION prevent_estimate_version_mutation();

CREATE TRIGGER estimate_versions_immutable_truncate
    BEFORE TRUNCATE ON estimate_versions
    FOR EACH STATEMENT EXECUTE FUNCTION prevent_estimate_version_mutation();

COMMIT;
