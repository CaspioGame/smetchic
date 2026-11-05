# CNC Estimator

The existing equipment/material/stock-blank cabinet and CLI continue to use
JSON and the existing Go calculation core. When `DATABASE_URL` is set, the Web
application additionally enables the PostgreSQL project lifecycle: projects,
uploaded `.part` revisions, estimates, and immutable estimate versions. The
legacy JSON CRUD has not been migrated.

## Local PostgreSQL setup

Create the `cnc_estimator` database using your local PostgreSQL role:

```powershell
$pgUser = "your_postgres_role"
psql -U $pgUser -h localhost -p 5432 -d postgres -c "CREATE DATABASE cnc_estimator;"
```

Apply the initial schema from the repository root:

```powershell
psql -U $pgUser -h localhost -p 5432 -d cnc_estimator -v ON_ERROR_STOP=1 -f migrations/001_initial.sql
psql -U $pgUser -h localhost -p 5432 -d cnc_estimator -v ON_ERROR_STOP=1 -f migrations/002_project_lifecycle.sql
```

The SQL file is also compatible with `psql` on other operating systems.

## Check the connection

Set `DATABASE_URL` in the current PowerShell session. Use your own PostgreSQL
role and password; do not put real credentials in source files or commit them:

```powershell
$env:DATABASE_URL = "postgres://your_postgres_role:your_local_password@localhost:5432/cnc_estimator?sslmode=disable"
go run ./cmd/dbcheck
```

The check prints a success message if the server responds to `Ping`, and exits
with a non-zero status if the URL is missing or the database cannot be reached.
Starting the Web app with `DATABASE_URL` set also connects PostgreSQL, ensures
the current JSON demo company and its settings exist without overwriting them,
and enables **Проекты и детали**. The project flow writes lifecycle records to
PostgreSQL; the existing cabinet settings/equipment remain sourced from JSON.
Without `DATABASE_URL`, the current JSON Web application remains available.
`go test ./...` includes an optional connection test when `DATABASE_URL` is set.

## Project lifecycle

After applying both migrations and setting `DATABASE_URL`, run:

```powershell
go run .
```

In the Web interface, create a project, upload `.part` files, and create
estimate versions for a part revision. Each upload stores the original source,
its SHA-256, parser version, and parsed Part. Re-uploading under the same part
name in the project creates a new revision. Each saved estimate version stores
input, settings, and result snapshots and is immutable in the database.
Equipment and current company settings for the calculation are read from the
existing JSON store; the calculation itself remains in the Go estimator.

## Inspect the schema

List the created tables:

```powershell
psql -U $pgUser -h localhost -p 5432 -d cnc_estimator -c "\dt"
```

Inspect foreign keys and indexes:

```powershell
psql -U $pgUser -h localhost -p 5432 -d cnc_estimator -c "\d+ stock_blanks"
psql -U $pgUser -h localhost -p 5432 -d cnc_estimator -c "\d+ parts"
psql -U $pgUser -h localhost -p 5432 -d cnc_estimator -c "\d+ estimate_versions"
```

The initial DDL is in [`migrations/001_initial.sql`](migrations/001_initial.sql);
project lifecycle constraints are in [`migrations/002_project_lifecycle.sql`](migrations/002_project_lifecycle.sql).
