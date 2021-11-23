ALTER TABLE connections
    ADD permitted_tenants jsonb NOT NULL DEFAULT '[]';

CREATE INDEX idx_permitted_tenants_gin ON connections USING gin (permitted_tenants);
