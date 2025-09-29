-- BrandPulse initial schema.
-- Mirrors shared/bp_core/models.py. A change here needs a change there.
-- Enum values are the StrEnum values in models.py, verbatim.

BEGIN;

CREATE TYPE source AS ENUM (
    'x', 'reddit', 'youtube', 'news',
    'playstore', 'appstore', 'amazon', 'flipkart', 'instagram', 'web'
);
CREATE TYPE sentiment_label AS ENUM ('negative', 'neutral', 'positive', 'mixed');
CREATE TYPE alert_kind AS ENUM (
    'spike', 'crisis', 'influencer_mention', 'competitor_move', 'review_bomb'
);
CREATE TYPE severity AS ENUM ('low', 'medium', 'high', 'critical');
CREATE TYPE alert_status AS ENUM ('open', 'acked', 'snoozed', 'resolved');
CREATE TYPE run_kind AS ENUM ('onboard', 'scheduled', 'on_demand', 'crisis_replay');
CREATE TYPE run_status AS ENUM ('running', 'ok', 'partial', 'failed');


CREATE TABLE brands (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    website         TEXT,
    plan            TEXT NOT NULL DEFAULT 'starter',
    whatsapp_number TEXT,
    -- Hard ceiling on Anakin credits this brand may burn in one UTC day.
    daily_credit_budget INTEGER NOT NULL DEFAULT 150,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE TABLE brand_profiles (
    brand_id          TEXT NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    version           INTEGER NOT NULL DEFAULT 1,
    keywords          JSONB NOT NULL DEFAULT '[]',
    hashtags          JSONB NOT NULL DEFAULT '[]',
    products          JSONB NOT NULL DEFAULT '[]',
    competitors       JSONB NOT NULL DEFAULT '[]',
    sources           JSONB NOT NULL DEFAULT '[]',
    negative_keywords JSONB NOT NULL DEFAULT '[]',
    source_handles    JSONB NOT NULL DEFAULT '{}',
    voice             JSONB NOT NULL DEFAULT '{}',
    confirmed_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (brand_id, version)
);


CREATE TABLE mentions (
    id               TEXT PRIMARY KEY,
    brand_id         TEXT NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    source           source NOT NULL,
    external_id      TEXT NOT NULL,
    url              TEXT,
    author           TEXT,
    author_followers INTEGER NOT NULL DEFAULT 0,
    text             TEXT NOT NULL,
    lang             TEXT NOT NULL DEFAULT 'en',
    posted_at        TIMESTAMPTZ NOT NULL,
    engagement       JSONB NOT NULL DEFAULT '{}',
    rating           REAL,
    matched_keyword  TEXT,
    -- sha256(normalised text + source); the dedupe key across keywords and runs.
    content_hash     TEXT NOT NULL,
    raw              JSONB NOT NULL DEFAULT '{}',
    collected_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (brand_id, content_hash)
);

-- The detector's baseline query is "count per source per hour over 14 days".
CREATE INDEX mentions_brand_posted_idx ON mentions (brand_id, posted_at DESC);
CREATE INDEX mentions_brand_source_posted_idx ON mentions (brand_id, source, posted_at DESC);


CREATE TABLE mention_enrichment (
    mention_id      TEXT PRIMARY KEY REFERENCES mentions(id) ON DELETE CASCADE,
    sentiment       REAL NOT NULL,
    sentiment_label sentiment_label NOT NULL,
    emotion         TEXT NOT NULL DEFAULT 'neutral',
    intent          TEXT NOT NULL DEFAULT 'other',
    aspects         JSONB NOT NULL DEFAULT '[]',
    is_about_brand  BOOLEAN NOT NULL DEFAULT TRUE,
    about_competitor TEXT,
    model           TEXT NOT NULL DEFAULT '',
    cost_paise      REAL NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE TABLE topics (
    id            TEXT PRIMARY KEY,
    brand_id      TEXT NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    window_start  TIMESTAMPTZ NOT NULL,
    window_end    TIMESTAMPTZ NOT NULL,
    label         TEXT NOT NULL,
    summary       TEXT NOT NULL DEFAULT '',
    size          INTEGER NOT NULL DEFAULT 0,
    sentiment_mix JSONB NOT NULL DEFAULT '{}',
    trend         REAL NOT NULL DEFAULT 1.0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE topic_mentions (
    topic_id   TEXT NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    mention_id TEXT NOT NULL REFERENCES mentions(id) ON DELETE CASCADE,
    PRIMARY KEY (topic_id, mention_id)
);


CREATE TABLE alerts (
    id         TEXT PRIMARY KEY,
    brand_id   TEXT NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    kind       alert_kind NOT NULL,
    severity   severity NOT NULL,
    title      TEXT NOT NULL,
    why        TEXT NOT NULL,
    evidence   JSONB NOT NULL DEFAULT '[]',
    sample_mention_ids JSONB NOT NULL DEFAULT '[]',
    status     alert_status NOT NULL DEFAULT 'open',
    -- Suppresses a duplicate alert for the same rule in the same bucket.
    dedupe_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (brand_id, dedupe_key)
);


CREATE TABLE reply_drafts (
    id         TEXT PRIMARY KEY,
    alert_id   TEXT REFERENCES alerts(id) ON DELETE CASCADE,
    mention_id TEXT REFERENCES mentions(id) ON DELETE CASCADE,
    channel    TEXT NOT NULL,
    text       TEXT NOT NULL,
    tone       TEXT NOT NULL DEFAULT '',
    do_not_say JSONB NOT NULL DEFAULT '[]',
    status     TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (alert_id IS NOT NULL OR mention_id IS NOT NULL)
);


CREATE TABLE briefs (
    id           TEXT PRIMARY KEY,
    brand_id     TEXT NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    period       TEXT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end   TIMESTAMPTZ NOT NULL,
    payload      JSONB NOT NULL,
    delivered_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (brand_id, period, period_start)
);


-- Every Anakin response lands here before it is parsed. A re-run inside the
-- same time_bucket is served from this table and costs zero credits.
CREATE TABLE fetch_cache (
    source      source NOT NULL,
    query_hash  TEXT NOT NULL,
    time_bucket TEXT NOT NULL,
    payload     JSONB NOT NULL,
    credits     INTEGER NOT NULL DEFAULT 0,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (source, query_hash, time_bucket)
);


CREATE TABLE runs (
    id                TEXT PRIMARY KEY,
    brand_id          TEXT NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    kind              run_kind NOT NULL,
    -- Idempotency key: one run per (brand, kind, time_bucket).
    time_bucket       TEXT NOT NULL,
    started_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at       TIMESTAMPTZ,
    status            run_status NOT NULL DEFAULT 'running',
    credits_used      INTEGER NOT NULL DEFAULT 0,
    tokens_used       INTEGER NOT NULL DEFAULT 0,
    cost_paise        REAL NOT NULL DEFAULT 0,
    sources_attempted JSONB NOT NULL DEFAULT '[]',
    sources_skipped   JSONB NOT NULL DEFAULT '[]',
    degraded_reason   TEXT,
    mentions_collected INTEGER NOT NULL DEFAULT 0,
    errors            JSONB NOT NULL DEFAULT '[]',
    UNIQUE (brand_id, kind, time_bucket)
);


-- Per-source yield, written after each run. The orchestrator reads this to
-- decide which sources to keep when the flow guard forces it to drop some.
CREATE TABLE source_yield (
    brand_id        TEXT NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    source          source NOT NULL,
    day             DATE NOT NULL,
    mentions        INTEGER NOT NULL DEFAULT 0,
    credits         INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (brand_id, source, day)
);

COMMIT;
