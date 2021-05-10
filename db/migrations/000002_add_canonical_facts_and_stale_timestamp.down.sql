DROP INDEX idx_canonical_facts_gin;

ALTER TABLE connections
    DROP COLUMN canonical_facts;

ALTER TABLE connections
    DROP COLUMN stale_timestamp;
