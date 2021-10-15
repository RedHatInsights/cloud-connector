ALTER TABLE connections
    DROP COLUMN IF EXISTS message_id;
ALTER TABLE connections
    DROP COLUMN IF EXISTS message_sent;
