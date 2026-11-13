CREATE TABLE global_tools (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    unit_price DOUBLE PRECISION NOT NULL CHECK (unit_price >= 0 AND unit_price < 'Infinity'::double precision),
    resource_minutes DOUBLE PRECISION NOT NULL CHECK (resource_minutes > 0 AND resource_minutes < 'Infinity'::double precision),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$')
);
