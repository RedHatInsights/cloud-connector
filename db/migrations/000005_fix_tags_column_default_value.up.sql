ALTER TABLE connections
    ALTER COLUMN tags SET NOT NULL;

ALTER TABLE connections
    ALTER COLUMN tags SET DEFAULT '{}';
