package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Task is a periodic job. Either Every (run when that long has passed since the last
// start) or DailyAtUTC (run once per day after that hour:minute) must be set.
type Task struct {
	Name       string
	Every      time.Duration
	DailyAtUTC *time.Duration // offset from midnight UTC
	Timeout    time.Duration
	Run        func(ctx context.Context) error
}

// Daily returns an offset for DailyAtUTC.
func Daily(hour, minute int) *time.Duration {
	d := time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute
	return &d
}

// Scheduler runs periodic tasks on exactly one replica at a time: claiming a run is an
// atomic UPDATE of worker_task_runs, so it doubles as leader election.
type Scheduler struct {
	pool   *pgxpool.Pool
	tasks  []Task
	logger *slog.Logger
	now    func() time.Time
}

func NewScheduler(pool *pgxpool.Pool, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{pool: pool, logger: logger, now: time.Now}
}

func (s *Scheduler) Add(t Task) {
	if t.Timeout <= 0 {
		t.Timeout = 30 * time.Minute
	}
	s.tasks = append(s.tasks, t)
}

// threshold is the instant a task's last start must be before for it to be due.
func (s *Scheduler) threshold(t Task, now time.Time) (time.Time, bool) {
	if t.DailyAtUTC != nil {
		slot := now.UTC().Truncate(24 * time.Hour).Add(*t.DailyAtUTC)
		if now.Before(slot) {
			slot = slot.Add(-24 * time.Hour)
		}
		return slot, true
	}
	return now.Add(-t.Every), t.Every > 0
}

// Run ticks every 5 s until ctx ends.
func (s *Scheduler) Run(ctx context.Context) error {
	for _, t := range s.tasks {
		if _, err := s.pool.Exec(ctx, `INSERT INTO worker_task_runs (name) VALUES ($1) ON CONFLICT DO NOTHING`, t.Name); err != nil {
			return err
		}
	}
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		s.RunDue(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

// RunDue runs every due task once (sequentially). Returns the names that ran.
func (s *Scheduler) RunDue(ctx context.Context) []string {
	var ran []string
	for _, t := range s.tasks {
		if ctx.Err() != nil {
			return ran
		}
		if s.runIfDue(ctx, t) {
			ran = append(ran, t.Name)
		}
	}
	return ran
}

// RunNow runs a task regardless of schedule (still exclusive across replicas).
func (s *Scheduler) RunNow(ctx context.Context, name string) error {
	for _, t := range s.tasks {
		if t.Name == name {
			return s.execute(ctx, t, time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC))
		}
	}
	return fmt.Errorf("unknown task %q", name)
}

func (s *Scheduler) runIfDue(ctx context.Context, t Task) bool {
	th, ok := s.threshold(t, s.now())
	if !ok {
		return false
	}
	return s.execute(ctx, t, th) != errNotClaimed
}

var errNotClaimed = fmt.Errorf("not claimed")

func (s *Scheduler) execute(ctx context.Context, t Task, threshold time.Time) error {
	_, _ = s.pool.Exec(ctx, `INSERT INTO worker_task_runs (name) VALUES ($1) ON CONFLICT DO NOTHING`, t.Name)
	tag, err := s.pool.Exec(ctx, `
		UPDATE worker_task_runs SET last_started_at = now(), locked_until = now() + $3::interval, runs = runs + 1
		WHERE name = $1 AND (locked_until IS NULL OR locked_until < now())
		  AND (last_started_at IS NULL OR last_started_at < $2)`,
		t.Name, threshold, fmt.Sprintf("%d seconds", int(t.Timeout.Seconds())))
	if err != nil || tag.RowsAffected() == 0 {
		return errNotClaimed
	}
	start := time.Now()
	tctx, cancel := context.WithTimeout(ctx, t.Timeout)
	runErr := func() (err error) {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("panic: %v", p)
			}
		}()
		return t.Run(tctx)
	}()
	cancel()
	var msg *string
	if runErr != nil {
		m := truncate(runErr.Error(), 2000)
		msg = &m
		s.logger.Error("periodic task failed", "task", t.Name, "error", runErr)
	} else {
		s.logger.Info("periodic task done", "task", t.Name, "ms", time.Since(start).Milliseconds())
	}
	_, err = s.pool.Exec(context.WithoutCancel(ctx), `
		UPDATE worker_task_runs SET last_finished_at = now(), last_duration_ms = $2, last_error = $3, locked_until = NULL,
		       failures = failures + CASE WHEN $3::text IS NULL THEN 0 ELSE 1 END
		WHERE name = $1`, t.Name, int(time.Since(start).Milliseconds()), msg)
	if err != nil {
		s.logger.Error("record task run", "task", t.Name, "error", err)
	}
	return runErr
}
