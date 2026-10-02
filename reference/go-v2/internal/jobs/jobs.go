// Package jobs is a small durable job queue on Postgres (see migration 00003).
//
// Enqueue inside the business transaction (pass the tx) so jobs are exactly as durable as
// the change that caused them. Runner claims jobs with FOR UPDATE SKIP LOCKED, runs the
// handler registered for the job's kind, and retries failures with exponential backoff.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is satisfied by *pgxpool.Pool, *pgxpool.Conn and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Options tune a single enqueue.
type Options struct {
	Queue       string
	Priority    int16
	RunAt       time.Time
	MaxAttempts int
	// UniqueKey makes enqueue a no-op while an identical (kind, key) job is pending or running.
	UniqueKey   string
	WorkspaceID *uuid.UUID
}

// Enqueue inserts a job. It returns 0 (and no error) when UniqueKey deduplicated it.
func Enqueue(ctx context.Context, db DBTX, kind string, payload any, o Options) (int64, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal %s payload: %w", kind, err)
	}
	if o.Queue == "" {
		o.Queue = "default"
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 8
	}
	var runAt *time.Time
	if !o.RunAt.IsZero() {
		runAt = &o.RunAt
	}
	var unique *string
	if o.UniqueKey != "" {
		unique = &o.UniqueKey
	}
	var id int64
	err = db.QueryRow(ctx, `
		INSERT INTO job_queue (kind, queue, payload, priority, run_at, max_attempts, unique_key, workspace_id)
		VALUES ($1, $2, $3, $4, COALESCE($5, now()), $6, $7, $8)
		ON CONFLICT (kind, unique_key) WHERE unique_key IS NOT NULL AND state IN ('available','running') DO NOTHING
		RETURNING id`, kind, o.Queue, b, o.Priority, runAt, o.MaxAttempts, unique, o.WorkspaceID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// Job is a claimed job handed to a handler.
type Job struct {
	ID          int64
	Kind        string
	Payload     json.RawMessage
	Attempt     int
	MaxAttempts int
	WorkspaceID *uuid.UUID
}

// Decode unmarshals the payload.
func (j *Job) Decode(v any) error { return json.Unmarshal(j.Payload, v) }

// Handler runs a job. Returning an error retries it; wrap with Permanent to fail at once;
// return Snooze to reschedule without consuming an attempt.
type Handler func(ctx context.Context, j *Job) error

type permanentErr struct{ err error }

func (p permanentErr) Error() string { return p.err.Error() }
func (p permanentErr) Unwrap() error { return p.err }

// Permanent marks an error that must not be retried.
func Permanent(err error) error { return permanentErr{err} }

type snoozeErr struct{ d time.Duration }

func (s snoozeErr) Error() string { return fmt.Sprintf("snoozed for %s", s.d) }

// Snooze reschedules the job after d without counting the attempt.
func Snooze(d time.Duration) error { return snoozeErr{d} }

// Backoff returns the delay before retry n (1-based): 15s, 1m, 5m, 20m ... capped at 12h, ±10 % jitter.
func Backoff(attempt int) time.Duration {
	d := time.Duration(15*math.Pow(4, float64(attempt-1))) * time.Second
	if d > 12*time.Hour || d <= 0 {
		d = 12 * time.Hour
	}
	j := time.Duration(rand.Int64N(int64(d) / 5))
	return d - d/10 + j
}

// Runner claims and executes jobs.
type Runner struct {
	pool        *pgxpool.Pool
	id          string
	queues      []string
	concurrency int
	timeout     time.Duration
	mu          sync.RWMutex
	handlers    map[string]Handler
	backoff     func(int) time.Duration
	logger      *slog.Logger
}

func NewRunner(pool *pgxpool.Pool, concurrency int, logger *slog.Logger) *Runner {
	if concurrency <= 0 {
		concurrency = 8
	}
	if logger == nil {
		logger = slog.Default()
	}
	host, _ := os.Hostname()
	return &Runner{pool: pool, id: fmt.Sprintf("%s-%d-%s", host, os.Getpid(), uuid.NewString()[:8]),
		queues: []string{"default"}, concurrency: concurrency, timeout: 10 * time.Minute,
		handlers: map[string]Handler{}, backoff: Backoff, logger: logger}
}

// Handle registers the handler for kind.
func (r *Runner) Handle(kind string, h Handler) {
	r.mu.Lock()
	r.handlers[kind] = h
	r.mu.Unlock()
}

// Queues sets which queues this runner serves.
func (r *Runner) Queues(q ...string) { r.queues = q }

// SetBackoff overrides the retry schedule (tests).
func (r *Runner) SetBackoff(f func(int) time.Duration) { r.backoff = f }

// Run polls until ctx ends, then waits for in-flight jobs.
func (r *Runner) Run(ctx context.Context) {
	sem := make(chan struct{}, r.concurrency)
	var wg sync.WaitGroup
	lastRescue := time.Time{}
	for ctx.Err() == nil {
		if time.Since(lastRescue) > time.Minute {
			r.rescue(ctx)
			lastRescue = time.Now()
		}
		free := r.concurrency - len(sem)
		if free == 0 {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		claimed, err := r.claim(ctx, free)
		if err != nil {
			if ctx.Err() == nil {
				r.logger.Warn("claim jobs", "error", err)
				sleep(ctx, 2*time.Second)
			}
			continue
		}
		if len(claimed) == 0 {
			sleep(ctx, 500*time.Millisecond)
			continue
		}
		for _, j := range claimed {
			sem <- struct{}{}
			wg.Add(1)
			go func(j *Job) {
				defer func() { <-sem; wg.Done() }()
				r.execute(context.WithoutCancel(ctx), j)
			}(j)
		}
	}
	wg.Wait()
}

// RunOnce claims and runs available jobs until none are due (tests, CLI).
func (r *Runner) RunOnce(ctx context.Context) (int, error) {
	total := 0
	for {
		claimed, err := r.claim(ctx, r.concurrency)
		if err != nil || len(claimed) == 0 {
			return total, err
		}
		for _, j := range claimed {
			r.execute(ctx, j)
			total++
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// kinds lists registered job kinds: a runner only claims work it can do, so jobs of a
// kind served by a newer deployment wait instead of failing.
func (r *Runner) kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.handlers))
	for k := range r.handlers {
		out = append(out, k)
	}
	return out
}

func (r *Runner) claim(ctx context.Context, n int) ([]*Job, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE job_queue SET state = 'running', attempt = attempt + 1, locked_by = $1, locked_at = now()
		WHERE id IN (
			SELECT id FROM job_queue
			WHERE state = 'available' AND queue = ANY($2) AND run_at <= now() AND kind = ANY($4)
			ORDER BY priority DESC, run_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $3)
		RETURNING id, kind, payload, attempt, max_attempts, workspace_id`, r.id, r.queues, n, r.kinds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		j := &Job{}
		if err := rows.Scan(&j.ID, &j.Kind, &j.Payload, &j.Attempt, &j.MaxAttempts, &j.WorkspaceID); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *Runner) execute(ctx context.Context, j *Job) {
	r.mu.RLock()
	h, ok := r.handlers[j.Kind]
	r.mu.RUnlock()
	var err error
	start := time.Now()
	if !ok {
		err = Permanent(fmt.Errorf("no handler registered for %q", j.Kind))
	} else {
		func() {
			defer func() {
				if p := recover(); p != nil {
					err = fmt.Errorf("panic: %v\n%s", p, debug.Stack())
				}
			}()
			jctx, cancel := context.WithTimeout(ctx, r.timeout)
			defer cancel()
			err = h(jctx, j)
		}()
	}
	var sn snoozeErr
	var perm permanentErr
	switch {
	case err == nil:
		_, err = r.pool.Exec(ctx, `UPDATE job_queue SET state = 'completed', finished_at = now(), locked_by = NULL, last_error = NULL WHERE id = $1`, j.ID)
	case errors.As(err, &sn):
		_, err = r.pool.Exec(ctx, `UPDATE job_queue SET state = 'available', attempt = attempt - 1, run_at = now() + $2::interval, locked_by = NULL WHERE id = $1`,
			j.ID, fmt.Sprintf("%d milliseconds", sn.d.Milliseconds()))
	case errors.As(err, &perm) || j.Attempt >= j.MaxAttempts:
		r.logger.Error("job failed permanently", "id", j.ID, "kind", j.Kind, "attempt", j.Attempt, "error", err)
		_, err = r.pool.Exec(ctx, `UPDATE job_queue SET state = 'failed', finished_at = now(), locked_by = NULL, last_error = $2 WHERE id = $1`, j.ID, truncate(err.Error(), 2000))
	default:
		delay := r.backoff(j.Attempt)
		r.logger.Warn("job failed; will retry", "id", j.ID, "kind", j.Kind, "attempt", j.Attempt, "retry_in", delay, "error", err)
		_, err = r.pool.Exec(ctx, `UPDATE job_queue SET state = 'available', run_at = now() + $2::interval, locked_by = NULL, last_error = $3 WHERE id = $1`,
			j.ID, fmt.Sprintf("%d milliseconds", delay.Milliseconds()), truncate(err.Error(), 2000))
	}
	if err != nil {
		r.logger.Error("record job result", "id", j.ID, "error", err)
	}
	r.logger.Debug("job done", "id", j.ID, "kind", j.Kind, "ms", time.Since(start).Milliseconds())
}

// rescue returns jobs whose worker died mid-run to the queue.
func (r *Runner) rescue(ctx context.Context) {
	tag, err := r.pool.Exec(ctx, `UPDATE job_queue SET state = 'available', locked_by = NULL, run_at = now()
		WHERE state = 'running' AND locked_at < now() - $1::interval`, fmt.Sprintf("%d seconds", int((r.timeout+5*time.Minute).Seconds())))
	if err == nil && tag.RowsAffected() > 0 {
		r.logger.Warn("rescued stuck jobs", "count", tag.RowsAffected())
	}
}

func truncate(s string, n int) string {
	s = strings.ToValidUTF8(s, "")
	if len(s) > n {
		return s[:n]
	}
	return s
}

// Prune deletes finished jobs older than keep.
func Prune(ctx context.Context, pool *pgxpool.Pool, keep time.Duration) (int64, error) {
	tag, err := pool.Exec(ctx, `DELETE FROM job_queue WHERE state IN ('completed','cancelled') AND finished_at < now() - $1::interval`,
		fmt.Sprintf("%d seconds", int(keep.Seconds())))
	return tag.RowsAffected(), err
}
