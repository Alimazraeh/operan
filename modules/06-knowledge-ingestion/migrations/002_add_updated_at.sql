-- Track last mutation time on jobs (UpdateStatus stamps it on every change).
ALTER TABLE ingestion_jobs ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
