DROP INDEX idx_permitted_tenants_gin;

ALTER TABLE connections
    DROP COLUMN permitted_tenants;
