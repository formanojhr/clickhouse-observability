package batcher

import (
	"context"
	"sync"
	"time"

	"go-log-service/internal/db"
)

type Batcher struct {
	store      *db.DB
	in         chan db.Log
	flushSize  int
	flushEvery time.Duration
	wg         sync.WaitGroup
}

func New(store *db.DB, flushSize int, flushEvery time.Duration) *Batcher {
	if flushSize <= 0 {
		flushSize = 500
	}
	if flushEvery <= 0 {
		flushEvery = 100 * time.Millisecond
	}
	return &Batcher{
		store:      store,
		in:         make(chan db.Log, flushSize*4),
		flushSize:  flushSize,
		flushEvery: flushEvery,
	}
}

func (b *Batcher) Submit(l db.Log) { b.in <- l }

func (b *Batcher) SubmitMany(ls []db.Log) {
	for _, l := range ls {
		b.in <- l
	}
}

// Run blocks until ctx is cancelled; flushes by size or time.
func (b *Batcher) Run(ctx context.Context) {
	b.wg.Add(1)
	defer b.wg.Done()

	buf := make([]db.Log, 0, b.flushSize)
	tick := time.NewTicker(b.flushEvery)
	defer tick.Stop()

	flush := func() {
		if len(buf) == 0 {
			return
		}
		// write in background so producers aren’t blocked by DB latency
		tmp := make([]db.Log, len(buf))
		copy(tmp, buf)
		buf = buf[:0]
		go b.store.InsertLogs(context.Background(), tmp) // best-effort
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case l := <-b.in:
			buf = append(buf, l)
			if len(buf) >= b.flushSize {
				flush()
			}
		case <-tick.C:
			flush()
		}
	}
}

func (b *Batcher) Close() {
	close(b.in)
	b.wg.Wait()
}

