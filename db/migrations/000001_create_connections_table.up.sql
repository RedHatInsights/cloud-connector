CREATE TABLE connections (
    id SERIAL PRIMARY KEY,
    account varchar(10) NOT NULL,
    client_id varchar(100) NOT NULL,
    dispatchers jsonb NOT NULL default '{}',
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);
