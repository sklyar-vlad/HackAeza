-- +goose Up
CREATE TABLE source_uploads (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id text NOT NULL REFERENCES app_users(id),
    total_bytes bigint NOT NULL CHECK (total_bytes > 0 AND total_bytes <= 1073741824),
    received_bytes bigint NOT NULL DEFAULT 0 CHECK (received_bytes BETWEEN 0 AND total_bytes),
    processed_bytes bigint NOT NULL DEFAULT 0 CHECK (processed_bytes BETWEEN 0 AND total_bytes),
    stage text NOT NULL DEFAULT 'receiving' CHECK (stage IN ('receiving', 'processing', 'accepted', 'failed')),
    job_id uuid REFERENCES calculation_jobs(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX source_uploads_actor_idx ON source_uploads (actor_id, created_at DESC);

CREATE TABLE source_upload_chunks (
    upload_id uuid NOT NULL REFERENCES source_uploads(id) ON DELETE CASCADE,
    byte_offset bigint NOT NULL CHECK (byte_offset >= 0),
    data bytea NOT NULL CHECK (octet_length(data) > 0),
    PRIMARY KEY (upload_id, byte_offset)
);

-- +goose Down
DROP TABLE source_upload_chunks, source_uploads;
