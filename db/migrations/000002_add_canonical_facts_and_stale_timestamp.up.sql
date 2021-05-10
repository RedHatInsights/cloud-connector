ALTER TABLE connections
    ADD canonical_facts jsonb NOT NULL default '{}';

CREATE INDEX idx_canonical_facts_gin ON connections USING gin (canonical_facts);

ALTER TABLE connections
    ADD stale_timestamp timestamptz NOT NULL DEFAULT NOW();
