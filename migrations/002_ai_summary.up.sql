-- Add versioning to answers (stop overwriting feedback, keep snapshots)
ALTER TABLE answers DROP CONSTRAINT answers_assignment_id_author_id_key;
ALTER TABLE answers ADD COLUMN version INT NOT NULL DEFAULT 1;
ALTER TABLE answers ADD COLUMN submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE answers ADD CONSTRAINT answers_assignment_author_version_unique UNIQUE (assignment_id, author_id, version);

-- AI summaries table
CREATE TABLE assignment_summaries (
    id TEXT PRIMARY KEY,
    assignment_id TEXT NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    summary TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('pending', 'completed', 'failed')) DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_summaries_assignment ON assignment_summaries(assignment_id);
