package repository_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"karecik/backend/internal/repository"
)

// The retry net makes five promises, and each is something a well-meant change
// could quietly break: a lost lock conflict is run again, any other error is
// not, three runs is the ceiling, a cancelled context stops the loop, and every
// retry leaves one log line behind. None of them needs a database — the
// conflicts below are *pgconn.PgError values built by hand, the very type pgx
// returns for a server error.

func deadlockError() error {
	return &pgconn.PgError{Code: "40P01", Message: "deadlock detected"}
}

// lineRecorder collects what a retry logs, formatted exactly as log.Printf
// would format it.
type lineRecorder struct {
	lines []string
}

func (r *lineRecorder) logf(format string, args ...any) {
	r.lines = append(r.lines, fmt.Sprintf(format, args...))
}

func noPause() time.Duration { return 0 }

func TestIsRetryableConflict(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"deadlock_detected 40P01", deadlockError(), true},
		{"serialization_failure 40001", &pgconn.PgError{Code: "40001"}, true},
		{"a wrapped deadlock", fmt.Errorf("could not delete the menu: %w", deadlockError()), true},
		{"unique_violation 23505", &pgconn.PgError{Code: "23505"}, false},
		{"foreign_key_violation 23503", &pgconn.PgError{Code: "23503"}, false},
		{"lock_not_available 55P03", &pgconn.PgError{Code: "55P03"}, false},
		{"in_failed_sql_transaction 25P02", &pgconn.PgError{Code: "25P02"}, false},
		{"ErrNotFound", repository.ErrNotFound, false},
		{"context.Canceled", context.Canceled, false},
		{"nil", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := repository.IsRetryableConflict(tc.err); got != tc.want {
				t.Fatalf("IsRetryableConflict(%v) = %t, want %t", tc.err, got, tc.want)
			}
		})
	}
}

func TestRetryOnConflictRetriesADeadlockThenSucceeds(t *testing.T) {
	runs := 0
	err := repository.RetryOnConflict(context.Background(), "TestWrite", func() error {
		runs++
		if runs == 1 {
			return deadlockError()
		}
		return nil
	})

	if err != nil {
		t.Fatalf("RetryOnConflict returned %v — the second run succeeded, so the deadlock of the "+
			"first one must not reach the caller", err)
	}
	if runs != 2 {
		t.Fatalf("the write ran %d time(s), want 2: one aborted by the deadlock, one that succeeded", runs)
	}
}

func TestRetryOnConflictValueReturnsTheValueOfTheSuccessfulRun(t *testing.T) {
	runs := 0
	got, err := repository.RetryOnConflictValue(context.Background(), "TestWrite", func() (int, error) {
		runs++
		if runs < 3 {
			return -1, &pgconn.PgError{Code: "40001"}
		}
		return 7, nil
	})

	if err != nil || got != 7 || runs != 3 {
		t.Fatalf("RetryOnConflictValue = (%d, %v) after %d run(s), want (7, <nil>) after 3: two "+
			"serialization failures and then a success", got, err, runs)
	}
}

func TestRetryOnConflictDoesNotRetryOtherErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", repository.ErrNotFound},
		{"unique_violation 23505", &pgconn.PgError{Code: "23505"}},
		{"foreign_key_violation 23503", &pgconn.PgError{Code: "23503"}},
		{"lock_not_available 55P03", &pgconn.PgError{Code: "55P03"}},
		{"a plain error", errors.New("connection refused")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var recorder lineRecorder
			runs := 0
			err := repository.RetryOnConflictWith(context.Background(), "TestWrite", repository.ConflictAttempts, noPause,
				recorder.logf, func() error {
					runs++
					return tc.err
				})

			if runs != 1 {
				t.Fatalf("the write ran %d times for %v, want exactly once: only 40P01 and 40001 "+
					"are worth another run", runs, tc.err)
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("RetryOnConflict returned %v, want the write's own error %v", err, tc.err)
			}
			if len(recorder.lines) != 0 {
				t.Fatalf("a write that was not retried logged %q, want nothing", recorder.lines)
			}
		})
	}
}

func TestRetryOnConflictStopsAfterThreeRuns(t *testing.T) {
	var recorder lineRecorder
	runs := 0
	err := repository.RetryOnConflictWith(context.Background(), "DeleteMenu", repository.ConflictAttempts, noPause,
		recorder.logf, func() error {
			runs++
			return deadlockError()
		})

	if runs != 3 {
		t.Fatalf("a write that always deadlocks ran %d time(s), want 3 in all", runs)
	}
	if !repository.IsRetryableConflict(err) {
		t.Fatalf("after the last run RetryOnConflict returned %v, want that run's deadlock error", err)
	}
	want := []string{
		"[karecik] retrying DeleteMenu after 40P01 (attempt 2/3)",
		"[karecik] retrying DeleteMenu after 40P01 (attempt 3/3)",
	}
	if strings.Join(recorder.lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("three runs that all deadlocked logged\n%q\nwant one line per retry and none for the "+
			"run that gave up:\n%q", recorder.lines, want)
	}
}

// Every retry names the operation, the SQLSTATE of the run it follows — taken
// from inside a wrapped error too — and the run it starts.
func TestRetryOnConflictLogsOneLinePerRetry(t *testing.T) {
	var recorder lineRecorder
	runs := 0
	err := repository.RetryOnConflictWith(context.Background(), "UpdateProduct", repository.ConflictAttempts, noPause,
		recorder.logf, func() error {
			runs++
			switch runs {
			case 1:
				return fmt.Errorf("could not update the product: %w", deadlockError())
			case 2:
				return &pgconn.PgError{Code: "40001"}
			}
			return nil
		})

	if err != nil || runs != 3 {
		t.Fatalf("RetryOnConflictWith = %v after %d run(s), want success on the third", err, runs)
	}
	want := []string{
		"[karecik] retrying UpdateProduct after 40P01 (attempt 2/3)",
		"[karecik] retrying UpdateProduct after 40001 (attempt 3/3)",
	}
	if strings.Join(recorder.lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the retries logged\n%q\nwant\n%q", recorder.lines, want)
	}
}

// The exported function has to write through the standard logger, which is
// where the process sends every other [karecik] line.
func TestRetryOnConflictWritesTheLineToTheStandardLogger(t *testing.T) {
	var buf bytes.Buffer
	savedWriter, savedFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(savedWriter)
		log.SetFlags(savedFlags)
	})

	runs := 0
	err := repository.RetryOnConflict(context.Background(), "DeleteCategory", func() error {
		runs++
		if runs == 1 {
			return deadlockError()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RetryOnConflict returned %v, want success on the second run", err)
	}
	if got, want := buf.String(), "[karecik] retrying DeleteCategory after 40P01 (attempt 2/3)\n"; got != want {
		t.Fatalf("the standard logger received %q, want %q", got, want)
	}
}

func TestRetryOnConflictStopsWhenTheContextIsCancelled(t *testing.T) {
	t.Run("cancelled_before_the_pause", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var recorder lineRecorder
		runs := 0
		err := repository.RetryOnConflictWith(ctx, "TestWrite", repository.ConflictAttempts, repository.ConflictBackoff, recorder.logf, func() error {
			runs++
			// The context goes away while the first run is still in the database.
			cancel()
			return deadlockError()
		})

		if runs != 1 {
			t.Fatalf("the write ran %d times after its context was cancelled, want exactly once", runs)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RetryOnConflict returned %v, want an error wrapping context.Canceled", err)
		}
		if !repository.IsRetryableConflict(err) {
			t.Fatalf("RetryOnConflict returned %v, want the deadlock kept alongside the cancellation", err)
		}
		if len(recorder.lines) != 0 {
			t.Fatalf("a retry that never ran logged %q, want nothing", recorder.lines)
		}
	})

	t.Run("cancelled_during_the_pause", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// A pause far longer than the test may take, cancelled shortly after it
		// starts: only a loop that watches ctx.Done() comes back in time.
		const pause = time.Minute
		timer := time.AfterFunc(20*time.Millisecond, cancel)
		defer timer.Stop()

		var recorder lineRecorder
		runs := 0
		started := time.Now()
		err := repository.RetryOnConflictWith(ctx, "TestWrite", repository.ConflictAttempts, func() time.Duration { return pause },
			recorder.logf, func() error {
				runs++
				return deadlockError()
			})
		elapsed := time.Since(started)

		if elapsed > 10*time.Second {
			t.Fatalf("RetryOnConflictWith returned after %s: the cancellation did not interrupt a %s pause",
				elapsed.Round(time.Millisecond), pause)
		}
		if runs != 1 {
			t.Fatalf("the write ran %d times, want exactly once: the context was cancelled during the "+
				"first pause", runs)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RetryOnConflictWith returned %v, want an error wrapping context.Canceled", err)
		}
		if len(recorder.lines) != 0 {
			t.Fatalf("a retry cancelled during its pause logged %q, want nothing", recorder.lines)
		}
	})
}

// The pause is part of the contract: short enough to be invisible to the owner,
// jittered so writers aborted together do not return together.
func TestConflictBackoffStaysBetween25And100Milliseconds(t *testing.T) {
	seen := make(map[time.Duration]bool)
	for i := 0; i < 10000; i++ {
		pause := repository.ConflictBackoff()
		if pause < 25*time.Millisecond || pause > 100*time.Millisecond {
			t.Fatalf("ConflictBackoff() = %s, want a pause between 25ms and 100ms", pause)
		}
		seen[pause] = true
	}
	if len(seen) < 2 {
		t.Fatalf("ConflictBackoff() returned the same pause 10000 times: the jitter is gone")
	}
}
