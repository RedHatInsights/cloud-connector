ALTER TABLE connections
    ADD org_id varchar(20);

CREATE INDEX idx_org_id ON connections (org_id);