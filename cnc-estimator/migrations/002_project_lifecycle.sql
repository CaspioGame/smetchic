BEGIN;

CREATE UNIQUE INDEX parts_company_project_name_uq
    ON parts (company_id, project_id, name);

CREATE UNIQUE INDEX estimates_company_revision_name_uq
    ON estimates (company_id, part_revision_id, name);

ALTER TABLE estimates
    ADD CONSTRAINT estimates_company_id_revision_uq
    UNIQUE (company_id, id, part_revision_id);

ALTER TABLE estimate_versions
    ADD COLUMN part_revision_id TEXT;

DROP TRIGGER estimate_versions_immutable ON estimate_versions;
DROP TRIGGER estimate_versions_immutable_truncate ON estimate_versions;

UPDATE estimate_versions AS versions
SET part_revision_id = estimates.part_revision_id
FROM estimates
WHERE estimates.company_id = versions.company_id
  AND estimates.id = versions.estimate_id;

ALTER TABLE estimate_versions
    ALTER COLUMN part_revision_id SET NOT NULL;

ALTER TABLE estimate_versions
    ADD CONSTRAINT estimate_versions_estimate_revision_fk
    FOREIGN KEY (company_id, estimate_id, part_revision_id)
    REFERENCES estimates (company_id, id, part_revision_id)
    ON DELETE RESTRICT;

CREATE INDEX estimate_versions_part_revision_id_idx
    ON estimate_versions (part_revision_id);

CREATE TRIGGER estimate_versions_immutable
    BEFORE UPDATE OR DELETE ON estimate_versions
    FOR EACH ROW EXECUTE FUNCTION prevent_estimate_version_mutation();

CREATE TRIGGER estimate_versions_immutable_truncate
    BEFORE TRUNCATE ON estimate_versions
    FOR EACH STATEMENT EXECUTE FUNCTION prevent_estimate_version_mutation();

COMMIT;
