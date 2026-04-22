ALTER TABLE users
    ADD COLUMN stripe_customer_id   TEXT UNIQUE,
    ADD COLUMN subscription_status  TEXT NOT NULL DEFAULT 'free',
    ADD COLUMN subscription_tier    TEXT,
    ADD COLUMN subscription_ends_at TIMESTAMPTZ,
    ADD COLUMN is_admin             BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX ON users (stripe_customer_id);
CREATE INDEX ON users (is_admin) WHERE is_admin = TRUE;

CREATE TABLE stripe_transactions (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                UUID REFERENCES users(id) ON DELETE SET NULL,
    stripe_event_id        TEXT NOT NULL UNIQUE,
    stripe_customer_id     TEXT,
    stripe_subscription_id TEXT,
    event_type             TEXT NOT NULL,
    amount_cents           INTEGER,
    currency               TEXT,
    status                 TEXT,
    raw_event              JSONB NOT NULL DEFAULT '{}',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE admin_logs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id  UUID NOT NULL REFERENCES users(id),
    action         TEXT NOT NULL,
    target_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    details        JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
