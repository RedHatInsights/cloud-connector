ALTER TABLE connections
    ADD message_id varchar(40);
ALTER TABLE connections
    ADD message_sent timestamptz NOT NULL DEFAULT NOW();
