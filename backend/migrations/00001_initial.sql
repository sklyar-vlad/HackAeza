-- +goose Up
CREATE TABLE app_users (
    id text PRIMARY KEY DEFAULT gen_random_uuid()::text CHECK (id <> ''),
    username text NOT NULL UNIQUE CHECK (username <> ''),
    password_hash text CHECK (password_hash <> ''),
    oidc_issuer text,
    oidc_subject text,
    created_at timestamptz NOT NULL DEFAULT now(),
    disabled_at timestamptz,
    UNIQUE (oidc_issuer, oidc_subject),
    CHECK ((oidc_issuer IS NULL) = (oidc_subject IS NULL)),
    CHECK (password_hash IS NOT NULL OR oidc_issuer IS NOT NULL)
);

CREATE TABLE auth_sessions (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id text NOT NULL REFERENCES app_users(id),
    auth_provider text NOT NULL CHECK (auth_provider IN ('password', 'oidc')),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL CHECK (expires_at > created_at),
    revoked_at timestamptz CHECK (revoked_at >= created_at)
);

CREATE INDEX auth_sessions_user_idx ON auth_sessions (user_id);
CREATE INDEX auth_sessions_expiry_idx ON auth_sessions (expires_at);

CREATE TABLE source_snapshots (
    version bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY CHECK (version > 0),
    received_at timestamptz NOT NULL DEFAULT now(),
    captured_at timestamptz,
    raw_payload jsonb CHECK (jsonb_typeof(raw_payload) = 'object'),
    accounts_count bigint CHECK (accounts_count >= 0),
    services_count bigint CHECK (services_count >= 0),
    transactions_count bigint CHECK (transactions_count >= 0),
    warnings jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(warnings) = 'array'),
    CHECK (
        (accounts_count IS NULL AND services_count IS NULL AND transactions_count IS NULL AND raw_payload IS NOT NULL)
        OR (accounts_count IS NOT NULL AND services_count IS NOT NULL AND transactions_count IS NOT NULL)
    )
);

CREATE TABLE accounts (
    snapshot_version bigint NOT NULL REFERENCES source_snapshots(version),
    id bigint NOT NULL CHECK (id > 0),
    email text COLLATE "C" NOT NULL CHECK (email <> ''),
    balance bigint NOT NULL CHECK (balance >= 0),
    withdraw_balance bigint NOT NULL CHECK (withdraw_balance >= 0),
    bonus_balance bigint NOT NULL CHECK (bonus_balance >= 0),
    bonus_state text NOT NULL CHECK (bonus_state IN ('locked', 'unlocked')),
    total_replenished bigint NOT NULL CHECK (total_replenished >= 0),
    PRIMARY KEY (snapshot_version, id),
    UNIQUE (snapshot_version, email)
);

CREATE INDEX accounts_balance_idx ON accounts (snapshot_version, balance, id);
CREATE INDEX accounts_replenished_idx ON accounts (snapshot_version, total_replenished, id);

CREATE TABLE services (
    snapshot_version bigint NOT NULL,
    id bigint NOT NULL CHECK (id > 0),
    owner_id bigint NOT NULL,
    name text NOT NULL CHECK (name ~ '^[A-Za-z0-9-]+$'),
    payment_term text NOT NULL CHECK (payment_term IN ('hour', 'week', 'month', 'quarter_year', 'half_year', 'year')),
    auto_prolong boolean NOT NULL,
    created_at timestamptz(3) NOT NULL,
    expires_at timestamptz(3) NOT NULL CHECK (expires_at >= created_at),
    status text NOT NULL CHECK (status IN ('active', 'suspended', 'prolong_wait', 'deleted', 'blocked')),
    price bigint NOT NULL CHECK (price >= 0),
    PRIMARY KEY (snapshot_version, id),
    FOREIGN KEY (snapshot_version, owner_id) REFERENCES accounts(snapshot_version, id)
);

CREATE INDEX services_owner_idx ON services (snapshot_version, owner_id, id);

CREATE TABLE transactions (
    snapshot_version bigint NOT NULL,
    id bigint NOT NULL CHECK (id > 0),
    service_id bigint NOT NULL,
    amount integer NOT NULL CHECK (amount BETWEEN 0 AND 1000),
    status text NOT NULL CHECK (status IN ('performed', 'not_performed')),
    type text NOT NULL CHECK (type IN ('order', 'prolong')),
    created_at timestamptz(3) NOT NULL,
    performed_at timestamptz(3),
    PRIMARY KEY (snapshot_version, id),
    FOREIGN KEY (snapshot_version, service_id) REFERENCES services(snapshot_version, id),
    CHECK (
        (status = 'performed' AND performed_at IS NOT NULL AND performed_at >= created_at)
        OR (status = 'not_performed' AND performed_at IS NULL)
    )
);

CREATE INDEX transactions_service_idx ON transactions (snapshot_version, service_id, created_at DESC, id DESC);

CREATE TABLE model_versions (
    version bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY CHECK (version > 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    coefficients jsonb NOT NULL CHECK (
        jsonb_typeof(coefficients) = 'object'
        AND NOT jsonb_path_exists(coefficients, 'strict $.* ? (@.type() != "number" || @ < 0)')
        AND jsonb_path_exists(coefficients, 'strict $.* ? (@ > 0)')
    ),
    medium_from double precision NOT NULL CHECK (medium_from >= 0),
    high_from double precision NOT NULL CHECK (high_from > medium_from AND high_from <= 100),
    horizon_days bigint NOT NULL CHECK (horizon_days > 0),
    refresh_interval_seconds bigint NOT NULL CHECK (refresh_interval_seconds > 0),
    score_formula text NOT NULL,
    loss_formula text NOT NULL
);

CREATE TABLE calculation_versions (
    version bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY CHECK (version > 0),
    source_version bigint REFERENCES source_snapshots(version),
    model_version bigint NOT NULL REFERENCES model_versions(version),
    base_version bigint REFERENCES calculation_versions(version),
    calculated_at timestamptz NOT NULL,
    published_at timestamptz CHECK (published_at >= calculated_at),
    UNIQUE (version, source_version),
    UNIQUE (version, model_version),
    CHECK (base_version IS NULL OR base_version < version)
);

CREATE INDEX calculation_versions_source_idx ON calculation_versions (source_version);
CREATE INDEX calculation_versions_model_idx ON calculation_versions (model_version);
CREATE INDEX calculation_versions_base_idx ON calculation_versions (base_version);

CREATE TABLE calculation_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL CHECK (kind IN ('model_settings', 'source_snapshot')),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    created_by text REFERENCES app_users(id),
    request_id text NOT NULL,
    candidate_source_version bigint REFERENCES source_snapshots(version),
    candidate_model_version bigint REFERENCES model_versions(version),
    base_calculation_version bigint REFERENCES calculation_versions(version),
    result_calculation_version bigint REFERENCES calculation_versions(version),
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz CHECK (started_at >= created_at),
    finished_at timestamptz CHECK (finished_at >= coalesce(started_at, created_at)),
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    worker_id text CHECK (worker_id <> ''),
    lease_token uuid,
    lease_until timestamptz,
    not_before timestamptz NOT NULL DEFAULT now(),
    processed_customers bigint,
    total_customers bigint,
    result jsonb CHECK (jsonb_typeof(result) = 'object'),
    error jsonb CHECK (jsonb_typeof(error) = 'object'),
    UNIQUE (id, created_by),
    CHECK (
        (kind = 'source_snapshot' AND candidate_source_version IS NOT NULL AND candidate_model_version IS NULL)
        OR (kind = 'model_settings' AND candidate_model_version IS NOT NULL AND candidate_source_version IS NULL)
    ),
    CHECK (
        (processed_customers IS NULL AND total_customers IS NULL)
        OR (processed_customers IS NOT NULL AND total_customers IS NOT NULL AND processed_customers BETWEEN 0 AND total_customers)
    ),
    CHECK (
        (status = 'queued' AND started_at IS NULL AND finished_at IS NULL AND worker_id IS NULL AND lease_token IS NULL AND lease_until IS NULL)
        OR (status = 'running' AND started_at IS NOT NULL AND finished_at IS NULL AND attempt > 0 AND worker_id IS NOT NULL AND lease_token IS NOT NULL AND lease_until IS NOT NULL AND lease_until > started_at)
        OR (status IN ('succeeded', 'failed') AND finished_at IS NOT NULL AND worker_id IS NULL AND lease_token IS NULL AND lease_until IS NULL)
    ),
    CHECK (
        (status IN ('queued', 'running') AND result IS NULL AND error IS NULL AND result_calculation_version IS NULL)
        OR (status = 'succeeded' AND result IS NOT NULL AND error IS NULL AND result_calculation_version IS NOT NULL)
        OR (status = 'failed' AND result IS NULL AND error IS NOT NULL AND result_calculation_version IS NULL)
    )
);

CREATE UNIQUE INDEX calculation_jobs_active_idx ON calculation_jobs ((true)) WHERE status IN ('queued', 'running');
CREATE INDEX calculation_jobs_claim_idx ON calculation_jobs (not_before, created_at) WHERE status = 'queued';
CREATE INDEX calculation_jobs_lease_idx ON calculation_jobs (lease_until) WHERE status = 'running';
CREATE INDEX calculation_jobs_actor_idx ON calculation_jobs (created_by, created_at DESC);
CREATE INDEX calculation_jobs_finished_idx ON calculation_jobs (finished_at) WHERE finished_at IS NOT NULL;

CREATE TABLE idempotency_receipts (
    actor_id text NOT NULL REFERENCES app_users(id),
    idempotency_key text NOT NULL CHECK (idempotency_key ~ '^[A-Za-z0-9._:-]{1,128}$'),
    operation text NOT NULL CHECK (operation IN ('model_settings', 'source_snapshot')),
    payload_hash bytea NOT NULL CHECK (octet_length(payload_hash) = 32),
    job_id uuid NOT NULL,
    accepted jsonb NOT NULL CHECK (jsonb_typeof(accepted) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL CHECK (expires_at >= created_at + interval '24 hours'),
    PRIMARY KEY (actor_id, idempotency_key),
    FOREIGN KEY (job_id, actor_id) REFERENCES calculation_jobs(id, created_by)
);

CREATE INDEX idempotency_receipts_job_idx ON idempotency_receipts (job_id, actor_id);
CREATE INDEX idempotency_receipts_expiry_idx ON idempotency_receipts (expires_at);

CREATE TABLE customer_results (
    calculation_version bigint NOT NULL,
    customer_id bigint NOT NULL,
    source_version bigint NOT NULL,
    services_count bigint NOT NULL CHECK (services_count >= 0),
    assessment_status text NOT NULL,
    risk_level text NOT NULL,
    risk_score double precision,
    loss_amount_cents bigint CHECK (loss_amount_cents >= 0),
    renewal_revenue_cents bigint NOT NULL CHECK (renewal_revenue_cents >= 0),
    unassessed_revenue_cents bigint NOT NULL CHECK (unassessed_revenue_cents BETWEEN 0 AND renewal_revenue_cents),
    is_partial boolean NOT NULL,
    loss_explanation text NOT NULL,
    features jsonb NOT NULL CHECK (jsonb_typeof(features) = 'array'),
    reasons jsonb NOT NULL CHECK (jsonb_typeof(reasons) = 'array'),
    PRIMARY KEY (calculation_version, customer_id),
    FOREIGN KEY (calculation_version, source_version) REFERENCES calculation_versions(version, source_version),
    FOREIGN KEY (source_version, customer_id) REFERENCES accounts(snapshot_version, id),
    CHECK (
        (assessment_status = 'assessed' AND risk_score IS NOT NULL AND risk_score BETWEEN 0 AND 100 AND risk_level IN ('low', 'medium', 'high'))
        OR (assessment_status = 'insufficient_data' AND risk_score IS NULL AND risk_level = 'unknown')
    ),
    CHECK (loss_amount_cents <= renewal_revenue_cents),
    CHECK (is_partial OR unassessed_revenue_cents = 0)
);

CREATE INDEX customer_results_source_idx ON customer_results (source_version, customer_id);
CREATE INDEX customer_results_score_idx ON customer_results (calculation_version, risk_score DESC NULLS LAST, customer_id);
CREATE INDEX customer_results_level_idx ON customer_results (calculation_version, risk_level, customer_id);
CREATE INDEX customer_results_loss_idx ON customer_results (calculation_version, loss_amount_cents DESC NULLS LAST, customer_id);

CREATE TABLE service_results (
    calculation_version bigint NOT NULL,
    service_id bigint NOT NULL,
    source_version bigint NOT NULL,
    assessment_status text NOT NULL,
    risk_level text NOT NULL,
    risk_score double precision,
    loss_amount_cents bigint CHECK (loss_amount_cents >= 0),
    renewal_revenue_cents bigint NOT NULL CHECK (renewal_revenue_cents >= 0),
    unassessed_revenue_cents bigint NOT NULL CHECK (unassessed_revenue_cents BETWEEN 0 AND renewal_revenue_cents),
    is_partial boolean NOT NULL,
    loss_explanation text NOT NULL,
    features jsonb NOT NULL CHECK (jsonb_typeof(features) = 'array'),
    reasons jsonb NOT NULL CHECK (jsonb_typeof(reasons) = 'array'),
    PRIMARY KEY (calculation_version, service_id),
    FOREIGN KEY (calculation_version, source_version) REFERENCES calculation_versions(version, source_version),
    FOREIGN KEY (source_version, service_id) REFERENCES services(snapshot_version, id),
    CHECK (
        (assessment_status = 'assessed' AND risk_score IS NOT NULL AND risk_score BETWEEN 0 AND 100 AND risk_level IN ('low', 'medium', 'high'))
        OR (assessment_status = 'insufficient_data' AND risk_score IS NULL AND risk_level = 'unknown')
    ),
    CHECK (loss_amount_cents <= renewal_revenue_cents),
    CHECK (is_partial OR unassessed_revenue_cents = 0)
);

CREATE INDEX service_results_source_idx ON service_results (source_version, service_id);

CREATE TABLE analytics_results (
    calculation_version bigint PRIMARY KEY REFERENCES calculation_versions(version),
    total_customers bigint NOT NULL CHECK (total_customers >= 0),
    assessed_customers bigint NOT NULL CHECK (assessed_customers >= 0),
    low_count bigint NOT NULL CHECK (low_count >= 0),
    medium_count bigint NOT NULL CHECK (medium_count >= 0),
    high_count bigint NOT NULL CHECK (high_count >= 0),
    unknown_count bigint NOT NULL CHECK (unknown_count >= 0),
    total_services bigint NOT NULL CHECK (total_services >= 0),
    at_risk_services bigint NOT NULL CHECK (at_risk_services BETWEEN 0 AND total_services),
    loss_amount_cents bigint CHECK (loss_amount_cents >= 0),
    renewal_revenue_cents bigint NOT NULL CHECK (renewal_revenue_cents >= 0),
    unassessed_revenue_cents bigint NOT NULL CHECK (unassessed_revenue_cents BETWEEN 0 AND renewal_revenue_cents),
    is_partial boolean NOT NULL,
    loss_explanation text NOT NULL,
    CHECK (total_customers = low_count + medium_count + high_count + unknown_count),
    CHECK (assessed_customers = total_customers - unknown_count),
    CHECK (loss_amount_cents <= renewal_revenue_cents),
    CHECK (is_partial OR unassessed_revenue_cents = 0)
);

CREATE TABLE published_state (
    id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    active_calculation_version bigint,
    active_model_version bigint NOT NULL REFERENCES model_versions(version),
    pending_job_id uuid REFERENCES calculation_jobs(id),
    FOREIGN KEY (active_calculation_version, active_model_version) REFERENCES calculation_versions(version, model_version)
);

INSERT INTO model_versions (coefficients, medium_from, high_from, horizon_days, refresh_interval_seconds, score_formula, loss_formula)
VALUES (
    '{"renewalWithoutAutoProlong": 0.4, "insufficientRenewalFunds": 0.35, "paymentWaiting": 0.25}',
    35, 70, 7, 60,
    '100 * sum(coefficient * applicableFeatureValue) / sum(applicableCoefficient)',
    'round(sum(nextRenewalPriceCents * serviceScore / 100))'
);

INSERT INTO published_state (active_model_version) SELECT version FROM model_versions;

-- +goose Down
DROP TABLE published_state, idempotency_receipts, analytics_results,
    service_results, customer_results, calculation_jobs, calculation_versions,
    transactions, services, accounts, source_snapshots, model_versions,
    auth_sessions, app_users;
