ALTER TABLE connections
    ALTER COLUMN org_id SET NOT NULL;

ALTER TABLE connections
    ALTER COLUMN account DROP NOT NULL;