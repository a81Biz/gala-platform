-- infra/postgres/init.sql

CREATE TABLE IF NOT EXISTS assets (
  id           TEXT PRIMARY KEY,
  kind         TEXT NOT NULL,
  provider     TEXT NOT NULL,
  object_key   TEXT NOT NULL,
  mime         TEXT NOT NULL,
  size_bytes   BIGINT NOT NULL,
  checksum     TEXT NULL,
  label        TEXT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS jobs (
  id           TEXT PRIMARY KEY,
  name         TEXT NULL,
  status       TEXT NOT NULL,
  params_json  TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at   TIMESTAMPTZ NULL,
  finished_at  TIMESTAMPTZ NULL,
  error_text   TEXT NULL
);

CREATE TABLE IF NOT EXISTS job_outputs (
  id                 TEXT PRIMARY KEY,
  job_id             TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  variant            INT NOT NULL DEFAULT 1,
  video_asset_id     TEXT NOT NULL REFERENCES assets(id),
  thumbnail_asset_id TEXT NULL REFERENCES assets(id),
  captions_asset_id  TEXT NULL REFERENCES assets(id),
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ✅ TEMPLATES (Punto 4.1)
CREATE TABLE IF NOT EXISTS templates (
  id           TEXT PRIMARY KEY,
  type         TEXT NOT NULL,
  name         TEXT NOT NULL UNIQUE,
  duration_ms  INT NULL,
  format       JSONB NULL,
  params_schema JSONB NULL,
  defaults     JSONB NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at   TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_assets_kind ON assets(kind);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_job_outputs_job_id ON job_outputs(job_id);

CREATE INDEX IF NOT EXISTS idx_templates_active
  ON templates (created_at)
  WHERE deleted_at IS NULL;
