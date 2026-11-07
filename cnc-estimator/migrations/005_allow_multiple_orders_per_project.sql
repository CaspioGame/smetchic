BEGIN;

ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS orders_company_id_project_id_key;

COMMIT;
