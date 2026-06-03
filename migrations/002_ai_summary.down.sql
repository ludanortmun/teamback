DROP INDEX idx_summaries_assignment;
DROP TABLE assignment_summaries;

ALTER TABLE answers DROP CONSTRAINT answers_assignment_author_version_unique;
ALTER TABLE answers DROP COLUMN submitted_at;
ALTER TABLE answers DROP COLUMN version;
ALTER TABLE answers ADD CONSTRAINT answers_assignment_id_author_id_key UNIQUE (assignment_id, author_id);
