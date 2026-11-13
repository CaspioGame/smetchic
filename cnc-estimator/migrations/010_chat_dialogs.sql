CREATE TABLE chat_dialogs (
    id TEXT PRIMARY KEY,
    company_id TEXT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    state JSONB NOT NULL DEFAULT '{}',
    revision INTEGER NOT NULL DEFAULT 0,
    busy_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chat_dialogs_owner_idx ON chat_dialogs(company_id, user_id, updated_at DESC);
