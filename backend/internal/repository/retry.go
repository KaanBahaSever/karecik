package repository

import (
	"context"
	"errors"
	"log"
	"math/rand/v2"
	"time"
)

// Running a write again after it lost a lock conflict.
//
// PostgreSQL never leaves two writers waiting on each other for good: after
// deadlock_timeout the deadlock detector aborts one side of the cycle with
// 40P01, and a REPEATABLE READ or SERIALIZABLE transaction that loses a
// serialization conflict is aborted with 40001. Either way the aborted
// statement or transaction wrote nothing, and the other side is usually done a
// moment later — so the same write, run again, almost always succeeds.
//
// The writers already avoid the lock cycles this codebase is known to produce:
// every multi-row writer locks its rows in ascending id order, product rows
// before category rows and a menu row, when it takes one, last (see
// DeleteMenu), and a create, a move or a reorder into a menu or a category
// waits for a delete of that parent at an advisory lock taken before any row
// lock (see locks.go). This is the net under those rules, for a cycle nobody
// has spotted yet, so a rare race costs the owner a pause of a few tens of
// milliseconds instead of a 500.
//
// A retry that succeeds hides the conflict from the response — the request
// answers as if nothing had happened — so every retry writes one log line
// naming the operation, the SQLSTATE it lost with and the run it starts. That
// line is how a cycle nobody has spotted yet gets noticed.
//
// What may be retried is a single self-contained statement, or a function that
// opens and finishes its own transaction. Never a statement inside a
// transaction the caller keeps open: PostgreSQL has already aborted that
// transaction, and every further statement in it fails with 25P02.

const (
	// ConflictAttempts is the total number of runs, the first one included.
	ConflictAttempts = 3

	// The pause before a retry is drawn from this range. It is short because
	// the other side of the conflict is usually finished already, and jittered
	// so that writers aborted at the same moment do not all come back at the
	// same moment.
	conflictBackoffMin = 25 * time.Millisecond
	conflictBackoffMax = 100 * time.Millisecond
)

// RetryOnConflict runs write and, while it fails with a retryable conflict
// (see IsRetryableConflict), runs it again after a short jittered pause — at
// most ConflictAttempts runs in all. Any other error is returned at once, and
// so is the conflict of the last run.
//
// operation names the write in the line every retry logs, right before the run
// it announces:
//
//	[karecik] retrying DeleteMenu after 40P01 (attempt 2/3)
//
// ctx is honoured between runs: once it is cancelled no further run starts,
// and the error returned wraps both the conflict and ctx.Err().
//
// For a Fiber request the handlers pass c.Context(), the fasthttp RequestCtx,
// and that context is cancelled only when the server shuts down — not when the
// client disconnects. In fasthttp v1.51.0 RequestCtx.Done returns the server's
// done channel, which Server.ShutdownWithContext closes and nothing else does.
// A request whose client has gone therefore still runs its retries to success
// or to the last run.
func RetryOnConflict(ctx context.Context, operation string, write func() error) error {
	return RetryOnConflictWith(ctx, operation, ConflictAttempts, ConflictBackoff, log.Printf, write)
}

// RetryOnConflictValue is RetryOnConflict for a write that also returns a
// value. The value of the last run comes back with that run's error.
func RetryOnConflictValue[T any](ctx context.Context, operation string,
	write func() (T, error)) (T, error) {

	var result T
	err := RetryOnConflict(ctx, operation, func() error {
		var err error
		result, err = write()
		return err
	})
	return result, err
}

// RetryOnConflictWith is RetryOnConflict with the number of runs, the pause and the
// logger passed in, so a test can hold a pause open long enough to cancel
// inside it and can read the lines a retry logs.
func RetryOnConflictWith(ctx context.Context, operation string, attempts int,
	pause func() time.Duration, logf func(format string, args ...any), write func() error) error {

	if attempts < 1 {
		attempts = 1
	}
	for attempt := 1; ; attempt++ {
		err := write()
		if err == nil || !IsRetryableConflict(err) || attempt >= attempts {
			return err
		}

		// Checked before the timer as well as during it: a context that is
		// already done must not buy one more run just because the select below
		// happened to see the timer first.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return errors.Join(err, ctxErr)
		}

		timer := time.NewTimer(pause())
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}

		// Logged after the pause, so a line is written only for a run that
		// really starts.
		logf("[karecik] retrying %s after %s (attempt %d/%d)",
			operation, sqlState(err), attempt+1, attempts)
	}
}

// ConflictBackoff draws the pause before the next run, uniformly from
// [conflictBackoffMin, conflictBackoffMax].
func ConflictBackoff() time.Duration {
	return conflictBackoffMin + rand.N(conflictBackoffMax-conflictBackoffMin+1)
}
