-- +goose Up
CREATE TABLE source_payload_parts (
    snapshot_version bigint NOT NULL REFERENCES source_snapshots(version),
    kind text NOT NULL CHECK (kind IN ('accounts', 'services', 'transactions')),
    batch_index integer NOT NULL CHECK (batch_index >= 0),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'array'),
    PRIMARY KEY (snapshot_version, kind, batch_index)
);

-- +goose Down
DROP TABLE source_payload_parts;
