CREATE TABLE IF NOT EXISTS novels (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    synopsis    TEXT,
    cover_image TEXT,
    author      TEXT,
    pov         TEXT,
    genre       TEXT,
    time_period TEXT,
    audience    TEXT,
    themes      TEXT[],
    tone        TEXT,
    extra_data  JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS concepts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    novel_id    UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    name        TEXT NOT NULL,
    aliases     TEXT[],
    tags        TEXT[],
    description TEXT,
    notes       TEXT,
    priority    INTEGER NOT NULL DEFAULT 0,
    image       TEXT,
    extra_data  JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS acts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    novel_id   UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    position   INTEGER NOT NULL DEFAULT 0,
    extra_data JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chapters (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    act_id     UUID NOT NULL REFERENCES acts(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    position   INTEGER NOT NULL DEFAULT 0,
    extra_data JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scenes (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_id          UUID NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    synopsis            TEXT,
    content             TEXT,
    tags                TEXT[],
    auto_update_context BOOLEAN NOT NULL DEFAULT FALSE,
    position            INTEGER NOT NULL DEFAULT 0,
    extra_data          JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scene_concepts (
    scene_id   UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    concept_id UUID NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    PRIMARY KEY (scene_id, concept_id)
);

CREATE TABLE IF NOT EXISTS concept_templates (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    novel_id      UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    template_data JSONB NOT NULL DEFAULT '{}',
    is_default    BOOLEAN NOT NULL DEFAULT FALSE,
    extra_data    JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
