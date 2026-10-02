-- 00003_worker.sql: durable background work.
--
-- job_queue: a transactional outbox + work queue on Postgres (FOR UPDATE SKIP LOCKED).
-- Producers enqueue in the same transaction as the business change, so a job exists
-- if and only if the change committed. Workers claim, run, and complete or retry with
-- exponential backoff; jobs that exhaust their attempts stay as 'failed' for inspection.
--
-- worker_task_runs: one row per periodic task; the atomic UPDATE that claims a run
-- doubles as leader election, so any number of worker replicas can run safely.

-- +goose Up
CREATE TABLE job_queue (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind          text        NOT NULL,
    queue         text        NOT NULL DEFAULT 'default',
    payload       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    priority      smallint    NOT NULL DEFAULT 0,
    state         text        NOT NULL DEFAULT 'available'
                  CHECK (state IN ('available','running','completed','failed','cancelled')),
    attempt       integer     NOT NULL DEFAULT 0,
    max_attempts  integer     NOT NULL DEFAULT 8 CHECK (max_attempts > 0),
    run_at        timestamptz NOT NULL DEFAULT now(),
    locked_by     text,
    locked_at     timestamptz,
    last_error    text,
    unique_key    text,
    workspace_id  uuid,
    created_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz
);
CREATE INDEX job_queue_fetch_idx   ON job_queue (queue, priority DESC, run_at, id) WHERE state = 'available';
CREATE INDEX job_queue_running_idx ON job_queue (locked_at) WHERE state = 'running';
CREATE INDEX job_queue_finished_idx ON job_queue (finished_at) WHERE state IN ('completed','cancelled');
CREATE INDEX job_queue_ws_idx      ON job_queue (workspace_id, created_at DESC) WHERE workspace_id IS NOT NULL;
-- At most one pending/running job per (kind, unique_key): enqueue is idempotent.
CREATE UNIQUE INDEX job_queue_unique_idx ON job_queue (kind, unique_key)
    WHERE unique_key IS NOT NULL AND state IN ('available','running');

CREATE TABLE worker_task_runs (
    name              text PRIMARY KEY,
    last_started_at   timestamptz,
    last_finished_at  timestamptz,
    last_duration_ms  integer,
    last_error        text,
    locked_until      timestamptz,
    runs              bigint NOT NULL DEFAULT 0,
    failures          bigint NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE IF EXISTS worker_task_runs;
DROP TABLE IF EXISTS job_queue;
