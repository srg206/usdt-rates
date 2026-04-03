-- +goose Up
CREATE TABLE rate_snapshots (
    id              BIGSERIAL PRIMARY KEY,
    exchange_time   TIMESTAMPTZ NOT NULL,
    bid             DOUBLE PRECISION NOT NULL,
    ask             DOUBLE PRECISION NOT NULL,
    bid_top_n       DOUBLE PRECISION NOT NULL,
    ask_top_n       DOUBLE PRECISION NOT NULL,
    bid_avg_nm      DOUBLE PRECISION NOT NULL,
    ask_avg_nm      DOUBLE PRECISION NOT NULL,
    top_n           INT NOT NULL,
    avg_n           INT NOT NULL,
    avg_m           INT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX rate_snapshots_created_at_idx ON rate_snapshots (created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS rate_snapshots_created_at_idx;
DROP TABLE IF EXISTS rate_snapshots;
