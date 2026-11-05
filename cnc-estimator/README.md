# CNC Estimator

The Web application currently uses the existing JSON storage. PostgreSQL support
in this stage is limited to the initial schema and a connection check; it is not
connected to the Web CRUD or the calculation pipeline.

## Local PostgreSQL setup

Create the `cnc_estimator` database using your local PostgreSQL role:

```powershell
$pgUser = "your_postgres_role"
psql -U $pgUser -h localhost -p 5432 -d postgres -c "CREATE DATABASE cnc_estimator;"
```

Apply the initial schema from the repository root:

```powershell
psql -U $pgUser -h localhost -p 5432 -d cnc_estimator -v ON_ERROR_STOP=1 -f migrations/001_initial.sql
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
`go test ./...` includes an optional PostgreSQL integration test when
`DATABASE_URL` is set.

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

The initial DDL is in [`migrations/001_initial.sql`](migrations/001_initial.sql).
