ALTER TABLE connections
    ALTER COLUMN org_id DROP NOT NULL;

ALTER TABLE connections
    ALTER COLUMN account SET NOT NULL;