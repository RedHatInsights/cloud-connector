ALTER TABLE connections
    ADD tenant_lookup_failure_count int DEFAULT 0;

ALTER TABLE connections
    ADD tenant_lookup_timestamp timestamptz;
