package redirect

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/scan"
)

const (
	// StreamName is the Redis stream the ingest service consumes.
	StreamName     = "scans"
	streamMaxLen   = 5_000_000
	flushInterval  = 50 * time.Millisecond
	flushBatchSize = 500
)

// Emitter decouples redirects from Redis: events go into a bounded channel and a single
// goroutine pipelines them into the stream every 50 ms or 500 events. A redirect never
// waits on analytics; if the buffer is full the event is dropped and counted.
type Emitter struct {
	rdb     *redis.Client
	ch      chan *scan.ScanEvent
	done    chan struct{}
	Dropped atomic.Int64
	Sent    atomic.Int64
	Failed  atomic.Int64
}

func NewEmitter(rdb *redis.Client, buffer int) *Emitter {
	if buffer <= 0 {
		buffer = 100_000
	}
	return &Emitter{rdb: rdb, ch: make(chan *scan.ScanEvent, buffer), done: make(chan struct{})}
}

// Emit enqueues without blocking.
func (e *Emitter) Emit(ev *scan.ScanEvent) {
	select {
	case e.ch <- ev:
	default:
		e.Dropped.Add(1)
	}
}

// Backlog is the number of buffered events.
func (e *Emitter) Backlog() int { return len(e.ch) }

// Run drains the buffer until ctx is cancelled, then flushes what is left for up to 10 s.
func (e *Emitter) Run(ctx context.Context) {
	defer close(e.done)
	t := time.NewTicker(flushInterval)
	defer t.Stop()
	batch := make([]*scan.ScanEvent, 0, flushBatchSize)
	flush := func(fctx context.Context) {
		if len(batch) == 0 {
			return
		}
		backoff := 100 * time.Millisecond
		for {
			if err := e.write(fctx, batch); err == nil {
				e.Sent.Add(int64(len(batch)))
				break
			} else if fctx.Err() != nil {
				e.Failed.Add(int64(len(batch)))
				slog.Error("scan events lost on shutdown", "count", len(batch), "error", err)
				break
			} else {
				slog.Warn("scan stream write failed; retrying", "error", err, "backlog", len(e.ch))
				select {
				case <-time.After(backoff):
				case <-fctx.Done():
				}
				if backoff < 5*time.Second {
					backoff *= 2
				}
			}
		}
		batch = batch[:0]
	}
	for {
		select {
		case ev := <-e.ch:
			batch = append(batch, ev)
			if len(batch) >= flushBatchSize {
				flush(ctx)
			}
		case <-t.C:
			flush(ctx)
		case <-ctx.Done():
			dctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			for {
				select {
				case ev := <-e.ch:
					batch = append(batch, ev)
					if len(batch) >= flushBatchSize {
						flush(dctx)
					}
					continue
				default:
				}
				break
			}
			flush(dctx)
			cancel()
			return
		}
	}
}

// Wait blocks until Run has returned.
func (e *Emitter) Wait() { <-e.done }

func (e *Emitter) write(ctx context.Context, batch []*scan.ScanEvent) error {
	if e.rdb == nil {
		return nil
	}
	pipe := e.rdb.Pipeline()
	for _, ev := range batch {
		b, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		pipe.XAdd(ctx, &redis.XAddArgs{Stream: StreamName, MaxLen: streamMaxLen, Approx: true, Values: []any{"e", string(b)}})
	}
	_, err := pipe.Exec(ctx)
	return err
}
