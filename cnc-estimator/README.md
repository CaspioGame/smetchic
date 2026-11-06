# CNC Estimator

PostgreSQL is the source of truth for the Web application's project lifecycle,
company context/settings, and equipment used for lifecycle estimates. The
existing equipment/material/stock-blank cabinet remains a legacy JSON feature;
its records are not automatically imported into PostgreSQL. The CLI continues
to use the existing Go calculation core and demo catalog.

## Local PostgreSQL setup

Create the `cnc_estimator` database using your local PostgreSQL role:

```powershell
$pgUser = "your_postgres_role"
psql -U $pgUser -h localhost -p 5433 -d postgres -c "CREATE DATABASE cnc_estimator;"
```

Apply the initial schema from the repository root:

```powershell
psql -U $pgUser -h localhost -p 5433 -d cnc_estimator -v ON_ERROR_STOP=1 -f migrations/001_initial.sql
psql -U $pgUser -h localhost -p 5433 -d cnc_estimator -v ON_ERROR_STOP=1 -f migrations/002_project_lifecycle.sql
psql -U $pgUser -h localhost -p 5433 -d cnc_estimator -v ON_ERROR_STOP=1 -f migrations/003_auth.sql
```

The SQL file is also compatible with `psql` on other operating systems.

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
are absent; existing PostgreSQL records are not overwritten. **Проекты и
детали** reads and writes company, project, revision, estimate, version,
settings, and equipment data through PostgreSQL. The legacy JSON cabinet stays
available in its existing sections, but is no longer the lifecycle's source
for company or estimate data. No JSON records are imported automatically.
`go test ./...` runs isolated PostgreSQL lifecycle integration tests when
`DATABASE_URL` is set.

## User login

The Web application uses PostgreSQL users and company memberships. Passwords
are stored as bcrypt hashes. There is no public registration or seeded
password. After starting the application once (which creates the demo company),
create an owner interactively:

```powershell
go run ./cmd/createuser
```

Enter the company ID `demo-company`, an email and name, then choose one of the
existing roles (`owner`, `admin`, `manager`, or `operator`). The password is
read without terminal echo and is never accepted as a command-line argument.
The Web login is at `http://localhost:8080/login`. Sessions are held server
side, last 12 hours, and are invalidated when the application stops. A user
with multiple company memberships is explicitly blocked until company
selection is implemented; the application does not choose a tenant arbitrarily.
The session cookie is HttpOnly and SameSite=Lax; Secure is enabled for HTTPS
requests or when `APP_ENV=production`.

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
