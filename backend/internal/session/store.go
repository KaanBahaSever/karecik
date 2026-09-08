// Package session keeps the live sessions of the API in this process's memory.
//
// It replaced a `sessions` table. The trade-off is deliberate and worth stating
// plainly, because it is not a pure win:
//
//   - GAINED: an authenticated request costs one map lookup instead of a
//     database round trip. That was the whole point.
//   - LOST: sessions live and die with the process. A restart, a crash or a
//     deploy signs everybody out. This was accepted knowingly.
//   - LOST: nothing outside this process can see or revoke a session. The
//     password-reset CLI in cmd/resetpw can no longer sign anyone out — see the
//     note there.
//
// ONE PROCESS ONLY. Two replicas behind a load balancer do not share this map,
// so a session created on one is unknown to the other and the user is thrown
// back to the login screen on roughly half of their requests. The API has to
// run as a single instance until sessions move to something shared.
//
// The raw cookie value never reaches this package. Keys are the SHA-256 hex of
// it (utils.HashSessionToken), exactly what the database column used to hold,
// so a heap dump yields no replayable credential either.
package session

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultJanitorInterval is how often expired entries are swept out. Expired
// entries are already refused by Lookup, so this interval is a memory concern
// rather than a security one and does not need to be tight.
const DefaultJanitorInterval = 10 * time.Minute

// MaxSessionsPerUser caps how many browsers one account may be signed in from
// at once; the oldest is evicted beyond it.
//
// Expiry alone bounds nothing inside the 30-day TTL: a script that logs in in a
// loop adds an entry per call and none of them are expired, so the janitor
// cannot touch them. On a database that grew a table; here it grows the
// container's heap until it is killed. Twenty devices is far more than a café
// owner uses, and it turns an unbounded leak into a bounded one.
const MaxSessionsPerUser = 20

// sweepBatch is how many entries one sweep deletes per acquisition of the write
// lock. See Sweep.
const sweepBatch = 512

// Entry is one signed-in browser.
//
// BusinessID is stored alongside UserID on purpose. The database lookup this
// package replaced resolved both in a single JOIN, and every dashboard endpoint
// scopes itself by the business. Keeping only the user here would mean asking
// the database "which business?" on every authenticated request — putting back
// the exact round trip the move to memory was meant to remove.
//
// Caching it is safe because it cannot change: a business row's id is assigned
// once and a user owns exactly one of them.
type Entry struct {
	UserID     uuid.UUID
	BusinessID uuid.UUID
	CreatedAt  time.Time
	ExpiresAt  time.Time
	IPAddress  string
	UserAgent  string
}

// Store is a thread-safe set of live sessions.
//
// Two maps rather than one:
//
//	byHash   token hash -> entry               the hot path, read on every request
//	byUser   user id    -> that user's hashes
//
// byUser exists for one operation: a password change revokes every other
// session of the user, and without an index that means scanning every session
// on the server. The cost of the index is that it has to be kept in step with
// the primary map, which is why every removal path in this file goes through
// removeLocked and nothing deletes from byHash directly.
type Store struct {
	mu     sync.RWMutex
	byHash map[string]Entry
	byUser map[uuid.UUID]map[string]struct{}

	// now is time.Now everywhere but the tests, which need to age a session
	// without sleeping. It is assigned at construction and never afterwards.
	now func() time.Time

	stop     chan struct{}
	stopOnce sync.Once
}

// New builds an empty store. The janitor is NOT started; call StartJanitor.
func New() *Store {
	return &Store{
		byHash: make(map[string]Entry),
		byUser: make(map[uuid.UUID]map[string]struct{}),
		now:    time.Now,
		stop:   make(chan struct{}),
	}
}

// Create records a signed-in browser under the hash of its cookie.
func (s *Store) Create(tokenHash string, entry Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// A hash that is somehow already present is replaced rather than merged, so
	// the two maps can never disagree about who owns it.
	s.removeLocked(tokenHash)

	s.byHash[tokenHash] = entry

	hashes := s.byUser[entry.UserID]
	if hashes == nil {
		hashes = make(map[string]struct{}, 1)
		s.byUser[entry.UserID] = hashes
	}
	hashes[tokenHash] = struct{}{}

	s.evictOldestLocked(entry.UserID)
}

// Lookup resolves a token hash to its session. The second result is false for a
// hash that is unknown, revoked or expired — the caller has one case to handle,
// which is the contract the database lookup had.
//
// An expired entry is reported as absent but NOT deleted here. Deleting would
// mean taking the write lock on the hot path, where a read lock lets every
// concurrent request through at once; the janitor collects it shortly after.
// Being late to free memory is cheap. Being late to refuse a session would not
// be — and this never returns one.
func (s *Store) Lookup(tokenHash string) (Entry, bool) {
	s.mu.RLock()
	entry, ok := s.byHash[tokenHash]
	s.mu.RUnlock()

	if !ok || !entry.ExpiresAt.After(s.now()) {
		return Entry{}, false
	}
	return entry, true
}

// Delete signs out exactly one browser. A hash that matches nothing is not an
// error: logging out twice, or with a stale cookie, is a no-op.
func (s *Store) Delete(tokenHash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeLocked(tokenHash)
}

// DeleteUser signs out every browser of one user, optionally sparing the one
// making the request, and returns how many it removed.
//
// This is the password-change path. Whoever changed the password keeps their
// own session — being logged out of the page you are standing on is a hostile
// way to confirm success — and every other session dies, which is the entire
// point when the reason for the change is that the old password leaked. Pass an
// empty exceptTokenHash to sign out everything.
func (s *Store) DeleteUser(userID uuid.UUID, exceptTokenHash string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	// Deleting from a map while ranging over it is defined behaviour in Go, and
	// removeLocked only ever deletes the key in hand.
	for hash := range s.byUser[userID] {
		if hash == exceptTokenHash {
			continue
		}
		if s.removeLocked(hash) {
			removed++
		}
	}
	return removed
}

// Sweep removes expired entries and returns how many went.
//
// It runs in two phases so that it cannot stall the API while it works:
//
//  1. find, under the READ lock — concurrent requests keep authenticating
//     throughout, because a read lock is shared.
//  2. delete, under the WRITE lock, in batches, releasing it between them. One
//     write lock held across the whole deletion would block every login and
//     every logout for the duration; a batch bounds that to a handful of map
//     deletes.
//
// Each candidate is re-checked under the write lock, because a hash can be
// deleted (a logout) or reissued in the gap between the two phases, and this
// must not throw out a session that became live meanwhile.
func (s *Store) Sweep() int {
	now := s.now()

	s.mu.RLock()
	expired := make([]string, 0, 64)
	for hash, entry := range s.byHash {
		if !entry.ExpiresAt.After(now) {
			expired = append(expired, hash)
		}
	}
	s.mu.RUnlock()

	removed := 0
	for start := 0; start < len(expired); start += sweepBatch {
		end := start + sweepBatch
		if end > len(expired) {
			end = len(expired)
		}

		s.mu.Lock()
		for _, hash := range expired[start:end] {
			entry, ok := s.byHash[hash]
			if !ok || entry.ExpiresAt.After(now) {
				continue
			}
			if s.removeLocked(hash) {
				removed++
			}
		}
		s.mu.Unlock()
	}
	return removed
}

// StartJanitor runs Sweep on a ticker until Stop is called. A non-positive
// interval falls back to DefaultJanitorInterval.
//
// Without it the map holds every session ever issued for the life of the
// process: an expired entry is refused, but refusing one does not free it.
func (s *Store) StartJanitor(interval time.Duration) {
	if interval <= 0 {
		interval = DefaultJanitorInterval
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if removed := s.Sweep(); removed > 0 {
					log.Printf("[karecik] purged %d expired session(s) from memory", removed)
				}
			case <-s.stop:
				return
			}
		}
	}()
}

// Stop ends the janitor. It is safe to call more than once, and safe to call
// when no janitor was ever started.
func (s *Store) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

// Len is the number of sessions held, expired ones included until the next
// sweep. It exists for the tests and for a health or metrics line.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byHash)
}

// removeLocked deletes one hash from BOTH maps. Every deletion in this file
// goes through it; that is what keeps the user index from drifting out of step
// with the primary map and stranding hashes that revocation would then miss.
//
// The caller must hold the write lock.
func (s *Store) removeLocked(tokenHash string) bool {
	entry, ok := s.byHash[tokenHash]
	if !ok {
		return false
	}

	delete(s.byHash, tokenHash)

	if hashes, ok := s.byUser[entry.UserID]; ok {
		delete(hashes, tokenHash)
		// Drop the empty set too, or byUser grows by one entry per account that
		// ever signed in and never shrinks.
		if len(hashes) == 0 {
			delete(s.byUser, entry.UserID)
		}
	}
	return true
}

// evictOldestLocked enforces MaxSessionsPerUser by dropping the user's oldest
// sessions. The caller must hold the write lock.
//
// The scan is linear in that one user's session count, which the cap itself
// holds at twenty-ish; it never touches the other accounts on the server.
//
// The iteration count is worked out ONCE, up front, and the loop counts down.
// The obvious alternative — `for len(s.byUser[userID]) > MaxSessionsPerUser` —
// spins for ever if a removal ever fails to shrink the index, and it does so
// while holding the write lock, which freezes every request on the server. That
// is not hypothetical: breaking removeLocked's index bookkeeping produces
// exactly that hang. A bound that does not depend on the removal succeeding
// turns a whole-API deadlock into, at worst, one session over the cap.
func (s *Store) evictOldestLocked(userID uuid.UUID) {
	for excess := len(s.byUser[userID]) - MaxSessionsPerUser; excess > 0; excess-- {
		var (
			oldestHash string
			oldestAt   time.Time
		)
		for hash := range s.byUser[userID] {
			at := s.byHash[hash].CreatedAt
			if oldestHash == "" || at.Before(oldestAt) {
				oldestHash, oldestAt = hash, at
			}
		}
		if oldestHash == "" || !s.removeLocked(oldestHash) {
			return
		}
	}
}
