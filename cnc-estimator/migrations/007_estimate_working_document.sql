BEGIN;

CREATE TABLE estimate_number_counters (
    company_id TEXT PRIMARY KEY REFERENCES companies (id) ON DELETE RESTRICT,
    current_number BIGINT NOT NULL DEFAULT 0 CHECK (current_number >= 0)
);

ALTER TABLE estimates
    ADD COLUMN number TEXT,
    ADD COLUMN project_id TEXT,
    ADD COLUMN part_id TEXT,
    ADD COLUMN order_id TEXT,
    ADD COLUMN quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    ADD COLUMN status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'calculated', 'approved', 'archived')),
    ADD COLUMN working_input JSONB NOT NULL DEFAULT '{}'::jsonb
        CHECK (jsonb_typeof(working_input) = 'object'),
    ADD COLUMN current_version_id TEXT;

ALTER TABLE parts
    ADD CONSTRAINT parts_company_project_part_uq UNIQUE (company_id, project_id, id);

ALTER TABLE part_revisions
    ADD CONSTRAINT part_revisions_company_part_id_uq UNIQUE (company_id, part_id, id);

WITH numbered AS (
    SELECT
        estimates.company_id,
        estimates.id,
        projects.id AS project_id,
        parts.id AS part_id,
        'СМ-' || lpad(row_number() OVER (
            PARTITION BY estimates.company_id ORDER BY estimates.created_at, estimates.id
        )::TEXT, 6, '0') AS number,
        CASE WHEN EXISTS (
            SELECT 1 FROM estimate_versions
            WHERE estimate_versions.company_id = estimates.company_id
              AND estimate_versions.estimate_id = estimates.id
        ) THEN 'calculated' ELSE 'draft' END AS status
    FROM estimates
    JOIN part_revisions ON part_revisions.company_id = estimates.company_id
                       AND part_revisions.id = estimates.part_revision_id
    JOIN parts ON parts.company_id = part_revisions.company_id
              AND parts.id = part_revisions.part_id
    JOIN projects ON projects.company_id = parts.company_id
                  AND projects.id = parts.project_id
)
UPDATE estimates
SET number = numbered.number,
    project_id = numbered.project_id,
    part_id = numbered.part_id,
    status = numbered.status
FROM numbered
WHERE numbered.company_id = estimates.company_id AND numbered.id = estimates.id;

ALTER TABLE estimates
    ALTER COLUMN number SET NOT NULL,
    ALTER COLUMN project_id SET NOT NULL,
    ALTER COLUMN part_id SET NOT NULL;

ALTER TABLE estimates
    ADD CONSTRAINT estimates_company_number_uq UNIQUE (company_id, number),
    ADD CONSTRAINT estimates_project_part_fk
        FOREIGN KEY (company_id, project_id, part_id)
        REFERENCES parts (company_id, project_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT estimates_revision_part_fk
        FOREIGN KEY (company_id, part_id, part_revision_id)
        REFERENCES part_revisions (company_id, part_id, id) ON DELETE RESTRICT,
    ADD CONSTRAINT estimates_order_company_fk
        FOREIGN KEY (company_id, order_id)
        REFERENCES orders (company_id, id) ON DELETE RESTRICT;

ALTER TABLE estimate_versions
    ADD CONSTRAINT estimate_versions_company_estimate_id_uq
        UNIQUE (company_id, estimate_id, id);

ALTER TABLE estimates
    ADD CONSTRAINT estimates_current_version_fk
        FOREIGN KEY (company_id, id, current_version_id)
        REFERENCES estimate_versions (company_id, estimate_id, id) ON DELETE RESTRICT;

INSERT INTO estimate_number_counters (company_id, current_number)
SELECT companies.id, COUNT(estimates.id)
FROM companies
LEFT JOIN estimates ON estimates.company_id = companies.id
GROUP BY companies.id;

CREATE INDEX estimates_company_updated_idx ON estimates (company_id, updated_at DESC);
CREATE INDEX estimates_project_id_idx ON estimates (project_id);
CREATE INDEX estimates_order_id_idx ON estimates (order_id) WHERE order_id IS NOT NULL;

COMMIT;
