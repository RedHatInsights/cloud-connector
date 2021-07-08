DROP INDEX idx_tags_gin;

ALTER TABLE connections
    DROP COLUMN tags;
