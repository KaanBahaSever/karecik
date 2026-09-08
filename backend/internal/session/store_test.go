package session

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The store is the authentication boundary now that no database row backs it,
// so these tests are the equivalent of the SQL predicates they replaced. The
// one that matters most is indexConsistency: byUser is what revocation walks,
// and if it drifts out of step with byHash a password change silently leaves
// sessions alive — which looks exactly like success.

// newTestStore returns a store with a clock the test controls, so a session can
// be aged past its expiry without sleeping.
func newTestStore(clock *time.Time) *Store {
	s := New()
	s.now = func() time.Time { return *clock }
	return s
}

// put adds a session that expires after ttl and returns its hash.
func put(s *Store, userID uuid.UUID, at time.Time, ttl time.Duration, hash string) string {
	s.Create(hash, Entry{
		UserID:     userID,
		BusinessID: uuid.New(),
		CreatedAt:  at,
		ExpiresAt:  at.Add(ttl),
	})
	return hash
}

// indexConsistency fails when byUser and byHash disagree. Every test calls it
// at the end: a leaked index entry is invisible until the day revocation needs
// it, which is the day it must not fail.
func indexConsistency(t *testing.T, s *Store) {
	t.Helper()

	s.mu.RLock()
	defer s.mu.RUnlock()

	counted := 0
	for userID, hashes := range s.byUser {
		if len(hashes) == 0 {
			t.Errorf("byUser holds an empty set for %s — it should have been dropped", userID)
		}
		for hash := range hashes {
			entry, ok := s.byHash[hash]
			if !ok {
				t.Errorf("byUser[%s] points at %q, which is not in byHash", userID, hash)
				continue
			}
			if entry.UserID != userID {
				t.Errorf("byUser[%s] holds %q, but that entry belongs to %s",
					userID, hash, entry.UserID)
			}
			counted++
		}
	}
	if counted != len(s.byHash) {
		t.Errorf("byUser indexes %d session(s), byHash holds %d", counted, len(s.byHash))
	}
}

func TestLookup(t *testing.T) {
	now := time.Now()
	s := newTestStore(&now)
	user := uuid.New()
	business := uuid.New()

	s.Create("hash-a", Entry{
		UserID:     user,
		BusinessID: business,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Hour),
	})

	entry, ok := s.Lookup("hash-a")
	if !ok {
		t.Fatal("a live session was not found")
	}
	if entry.UserID != user {
		t.Errorf("user id = %s, want %s", entry.UserID, user)
	}
	// The business travels with the session precisely so the middleware never
	// has to ask the database for it.
	if entry.BusinessID != business {
		t.Errorf("business id = %s, want %s", entry.BusinessID, business)
	}

	if _, ok := s.Lookup("hash-that-was-never-issued"); ok {
		t.Error("an unknown hash was accepted")
	}
	if _, ok := s.Lookup(""); ok {
		t.Error("an empty hash was accepted")
	}

	indexConsistency(t, s)
}

func TestLookupRefusesExpired(t *testing.T) {
	now := time.Now()
	s := newTestStore(&now)

	put(s, uuid.New(), now, time.Hour, "hash-a")

	now = now.Add(time.Hour + time.Second)

	if _, ok := s.Lookup("hash-a"); ok {
		t.Error("an expired session was accepted")
	}
	// Refused but still resident: Lookup deliberately does not take the write
	// lock on the hot path, so the janitor is what frees it.
	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1 — Lookup should not delete", s.Len())
	}

	if removed := s.Sweep(); removed != 1 {
		t.Errorf("Sweep() removed %d, want 1", removed)
	}
	if s.Len() != 0 {
		t.Errorf("Len() = %d after the sweep, want 0", s.Len())
	}

	indexConsistency(t, s)
}

func TestDelete(t *testing.T) {
	now := time.Now()
	s := newTestStore(&now)
	user := uuid.New()

	put(s, user, now, time.Hour, "hash-a")
	put(s, user, now, time.Hour, "hash-b")

	if !s.Delete("hash-a") {
		t.Error("Delete reported that a live session was absent")
	}
	if _, ok := s.Lookup("hash-a"); ok {
		t.Error("a deleted session still authenticates")
	}
	if _, ok := s.Lookup("hash-b"); !ok {
		t.Error("deleting one session took the other with it")
	}

	// Logging out twice is a no-op, not an error.
	if s.Delete("hash-a") {
		t.Error("Delete reported removing a session that was already gone")
	}

	indexConsistency(t, s)
}

func TestDeleteUserSparesTheCaller(t *testing.T) {
	now := time.Now()
	s := newTestStore(&now)
	owner, stranger := uuid.New(), uuid.New()

	current := put(s, owner, now, time.Hour, "owner-current")
	put(s, owner, now, time.Hour, "owner-phone")
	put(s, owner, now, time.Hour, "owner-stolen")
	put(s, stranger, now, time.Hour, "stranger-laptop")

	removed := s.DeleteUser(owner, current)
	if removed != 2 {
		t.Errorf("DeleteUser removed %d session(s), want 2", removed)
	}

	if _, ok := s.Lookup(current); !ok {
		t.Error("the session that changed the password was signed out")
	}
	for _, hash := range []string{"owner-phone", "owner-stolen"} {
		if _, ok := s.Lookup(hash); ok {
			t.Errorf("%s survived the revocation — a leaked password would still work", hash)
		}
	}
	// Cross-tenant: one account's password change must not touch another's.
	if _, ok := s.Lookup("stranger-laptop"); !ok {
		t.Error("another user's session was revoked")
	}

	indexConsistency(t, s)
}

func TestDeleteUserWithoutExceptionClearsEverything(t *testing.T) {
	now := time.Now()
	s := newTestStore(&now)
	owner := uuid.New()

	put(s, owner, now, time.Hour, "a")
	put(s, owner, now, time.Hour, "b")

	// The empty exception is what an administrative reset passes.
	if removed := s.DeleteUser(owner, ""); removed != 2 {
		t.Errorf("DeleteUser removed %d, want 2", removed)
	}
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
	if removed := s.DeleteUser(uuid.New(), ""); removed != 0 {
		t.Errorf("DeleteUser on an unknown user removed %d, want 0", removed)
	}

	indexConsistency(t, s)
}

func TestSweepKeepsLiveSessions(t *testing.T) {
	now := time.Now()
	s := newTestStore(&now)
	user := uuid.New()

	put(s, user, now, time.Minute, "short")
	put(s, user, now, 48*time.Hour, "long")

	now = now.Add(time.Hour)

	if removed := s.Sweep(); removed != 1 {
		t.Errorf("Sweep() removed %d, want 1", removed)
	}
	if _, ok := s.Lookup("long"); !ok {
		t.Error("the sweep took a session that had not expired")
	}
	// A sweep with nothing to do must not disturb anything either.
	if removed := s.Sweep(); removed != 0 {
		t.Errorf("the second Sweep() removed %d, want 0", removed)
	}

	indexConsistency(t, s)
}

func TestEvictsOldestBeyondTheCap(t *testing.T) {
	now := time.Now()
	s := newTestStore(&now)
	user := uuid.New()

	// One more than the cap, each a second older than the next.
	for i := 0; i <= MaxSessionsPerUser; i++ {
		put(s, user, now.Add(time.Duration(i)*time.Second), time.Hour, fmt.Sprintf("hash-%02d", i))
	}

	if s.Len() != MaxSessionsPerUser {
		t.Errorf("Len() = %d, want the cap of %d", s.Len(), MaxSessionsPerUser)
	}
	if _, ok := s.Lookup("hash-00"); ok {
		t.Error("the oldest session survived the cap")
	}
	if _, ok := s.Lookup(fmt.Sprintf("hash-%02d", MaxSessionsPerUser)); !ok {
		t.Error("the newest session was evicted instead of the oldest")
	}

	indexConsistency(t, s)
}

// TestConcurrentAccess is meaningful under `go test -race`: the store is read
// on every authenticated request and written by logins, logouts and the
// janitor, all at once.
func TestConcurrentAccess(t *testing.T) {
	s := New()
	users := make([]uuid.UUID, 8)
	for i := range users {
		users[i] = uuid.New()
	}

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			user := users[worker]
			for i := 0; i < 200; i++ {
				hash := fmt.Sprintf("w%d-%d", worker, i)
				s.Create(hash, Entry{
					UserID:     user,
					BusinessID: uuid.New(),
					CreatedAt:  time.Now(),
					// Half of them are born expired, so the sweeps below have
					// something to race the readers over.
					ExpiresAt: time.Now().Add(time.Duration(i%2)*time.Hour - time.Second),
				})
				s.Lookup(hash)
				if i%3 == 0 {
					s.Delete(hash)
				}
				if i%50 == 0 {
					s.DeleteUser(user, hash)
				}
			}
		}(w)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			s.Sweep()
			s.Len()
		}
	}()

	wg.Wait()
	indexConsistency(t, s)
}

// TestStopIsIdempotent guards the shutdown path: Stop closes a channel, and
// closing a closed channel panics.
func TestStopIsIdempotent(t *testing.T) {
	s := New()
	s.StartJanitor(time.Millisecond)
	s.Stop()
	s.Stop()
}
