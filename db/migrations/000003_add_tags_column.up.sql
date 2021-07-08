ALTER TABLE connections
    ADD tags jsonb;

CREATE INDEX idx_tags_gin ON connections USING gin (tags);
