# CNC Estimator

PostgreSQL is the source of truth for the authenticated Web application,
company workspace, orders, and project lifecycle. The legacy JSON cabinet code
remains in the repository for compatibility but is not mounted by the main
application and is not used for new records. Existing JSON data is not
automatically imported. The CLI continues to use the existing Go calculation
core and demo catalog.

## Local PostgreSQL setup

Create the `cnc_estimator` database using your local PostgreSQL role:

```powershell
$pgUser = "your_postgres_role"
psql -U $pgUser -h localhost -p 5433 -d postgres -c "CREATE DATABASE cnc_estimator;"
```

Database migrations are embedded in the application and are applied
automatically by `go run .` before the Web server starts. To apply/check them
without starting the server:

```powershell
go run ./cmd/migrate
```

The runner records successful migrations in `schema_migrations`, applies each
one transactionally, and uses a PostgreSQL advisory lock to coordinate multiple
application instances. Existing schemas created manually from migrations
001–004 are detected and recorded without re-running those DDL statements;
missing migrations are then applied. If the schema is only partially present,
the runner stops and reports the affected migration instead of risking data.

## Check the connection

Set `DATABASE_URL` in the current PowerShell session. Use your own PostgreSQL
role and password; do not put real credentials in source files or commit them:

```powershell
$env:DATABASE_URL = "postgres://postgres:<password>@localhost:5433/cnc_estimator?sslmode=disable"
go run ./cmd/dbcheck
```

The check prints a success message if the server responds to `Ping`, and exits
with a non-zero status if the URL is missing or the database cannot be reached.
Starting the Web app requires `DATABASE_URL`. It idempotently seeds a default
demo company, settings, and DeKart S4040 into PostgreSQL only when their IDs
are absent; existing PostgreSQL records are not overwritten. The authenticated
workspace reads and writes company profile/settings, company equipment and
materials, consumables, orders, and project lifecycle data through PostgreSQL.
Global equipment/material catalogs are read-only. No JSON records are imported
automatically.
`go test ./...` runs isolated PostgreSQL lifecycle integration tests when
`DATABASE_URL` is set.

## Registration and login

The Web application uses PostgreSQL users and company memberships. A user can
register at `/register`; registration atomically creates a company, its default
settings, a user account, and an `owner` membership. Passwords must contain at
least six characters and are stored as bcrypt hashes. The login page is at
`http://localhost:8080/login`.

For local administration, create a user for an existing company interactively:

```powershell
go run ./cmd/createuser
```

Enter the company ID, email and name, then choose one of the existing roles
(`owner`, `admin`, `manager`, or `operator`). The password is read without
terminal echo and is never accepted as a command-line argument. Sessions are
held server side, last 12 hours, and are invalidated when the application
stops. A user with multiple company memberships is explicitly blocked until
company selection is implemented; the application does not choose a tenant
arbitrarily.
The session cookie is HttpOnly and SameSite=Lax; Secure is enabled for HTTPS
requests or when `APP_ENV=production`.

## Project lifecycle

After creating the database and setting `DATABASE_URL`, run:

```powershell
go run .
```

In the Web interface, create a project, upload `.part` files, and create
estimate versions for a part revision. Each upload stores the original source,
its SHA-256, parser version, and parsed Part. Re-uploading under the same part
name in the project creates a new revision. Each saved estimate version stores
input, settings, and result snapshots and is immutable in the database.
Equipment and current company settings for the calculation are read from
PostgreSQL; the calculation itself remains in the existing Go estimator.

## Inspect the schema

List the created tables:

```powershell
psql -U $pgUser -h localhost -p 5433 -d cnc_estimator -c "\dt"
```

Inspect foreign keys and indexes:

```powershell
psql -U $pgUser -h localhost -p 5433 -d cnc_estimator -c "\d+ stock_blanks"
psql -U $pgUser -h localhost -p 5433 -d cnc_estimator -c "\d+ parts"
psql -U $pgUser -h localhost -p 5433 -d cnc_estimator -c "\d+ estimate_versions"
```

The initial DDL is in [`migrations/001_initial.sql`](migrations/001_initial.sql);
project lifecycle constraints are in [`migrations/002_project_lifecycle.sql`](migrations/002_project_lifecycle.sql).
Authentication is in [`migrations/003_auth.sql`](migrations/003_auth.sql), and
workspace catalogs, consumables, and orders are in
[`migrations/004_workspace.sql`](migrations/004_workspace.sql). Migration
[`migrations/005_allow_multiple_orders_per_project.sql`](migrations/005_allow_multiple_orders_per_project.sql)
removes the old one-order-per-project restriction without changing existing
order data.
