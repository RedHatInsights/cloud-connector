ALTER TABLE connections
    ADD permitted_tenants jsonb DEFAULT '[]';

CREATE INDEX idx_permitted_tenants_gin ON connections USING gin (permitted_tenants);
