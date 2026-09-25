// Package eventgate decides which visitor events the public analytics endpoint
// actually stores.
//
// POST /api/public/events is open to the whole internet and writes a row per
// request, so the per-client rate limiter in front of it is not enough on its
// own to keep menu_events from growing without bound: a limit a real visitor
// never reaches still lets one client write hundreds of thousands of rows a
// day, and the database behind it has a small storage quota. The gate adds two
// cheaper guards, both in memory:
//
//   - a repeat of the same event — the same visitor, type, menu, category and
//     product — within a short window is dropped. A double tap, a dialog
//     opened twice or a page that reports the same view on every re-render
//     describes one look at the menu, not several;
//   - each business stores at most a fixed number of events per calendar day.
//     Beyond it events are still accepted — the page is never told anything
//     that would make it retry — but not stored, and the first refusal of the
//     day is reported once so that an operator can tell a flood from a quiet
//     day.
//
// Both are per process. The deployment runs a single instance (sessions live
// in memory too), and a restart only forgets the repeats of the last few
// seconds; the daily count is not forgotten, because the first event of a
// business after a start-up is counted against what the database already holds
// for that day.
package eventgate

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Defaults of Config.
const (
	// DefaultRepeatWindow is how long an event counts as a repeat of the last
	// identical one. Ten seconds covers a double tap and a re-render without
	// merging two real visits: somebody who comes back to a product after
	// reading the rest of the menu has been away longer than that.
	DefaultRepeatWindow = 10 * time.Second

	// DefaultMaxRepeatKeys bounds the repeat set. Each entry is one event of
	// the last window, so the bound is reached only at thousands of distinct
	// events a second — far beyond what the rate limiter lets in. When it is
	// reached anyway, new events are simply not remembered as repeats (they are
	// stored, and still count against the daily cap) rather than growing the
	// set any further.
	DefaultMaxRepeatKeys = 100_000
)

// Config configures a Gate. The zero value of every field but Zone takes the
// default named on it.
type Config struct {
	// DailyCap is how many events one business may store per calendar day.
	// 0 disables the cap.
	DailyCap int

	// Zone is the calendar the day is counted in — the one the dashboard
	// draws its daily series in, so "today" means the same thing to both.
	Zone *time.Location

	// RepeatWindow is DefaultRepeatWindow when zero.
	RepeatWindow time.Duration

	// MaxRepeatKeys is DefaultMaxRepeatKeys when zero.
	MaxRepeatKeys int

	// Now is time.Now when nil; a test sets it to move the clock.
	Now func() time.Time
}

// Gate is safe for concurrent use.
type Gate struct {
	cap     int
	zone    *time.Location
	window  time.Duration
	maxKeys int
	now     func() time.Time

	mu sync.Mutex

	// repeats maps an event key to the moment it stops being a repeat.
	repeats   map[string]time.Time
	nextSweep time.Time

	// day is the calendar day the counts below belong to; they are dropped
	// together the first time the gate is asked about a later day.
	day    string
	counts map[uuid.UUID]*dailyCount
}

// dailyCount is one business's stored events of the current day.
type dailyCount struct {
	stored   int
	reported bool
}

// New builds a Gate.
func New(cfg Config) *Gate {
	g := &Gate{
		cap:     cfg.DailyCap,
		zone:    cfg.Zone,
		window:  cfg.RepeatWindow,
		maxKeys: cfg.MaxRepeatKeys,
		now:     cfg.Now,
		repeats: make(map[string]time.Time),
		counts:  make(map[uuid.UUID]*dailyCount),
	}
	if g.cap < 0 {
		g.cap = 0
	}
	if g.zone == nil {
		g.zone = time.UTC
	}
	if g.window <= 0 {
		g.window = DefaultRepeatWindow
	}
	if g.maxKeys <= 0 {
		g.maxKeys = DefaultMaxRepeatKeys
	}
	if g.now == nil {
		g.now = time.Now
	}
	return g
}

// DailyCap is the cap the gate enforces; 0 means none.
func (g *Gate) DailyCap() int { return g.cap }

// ----------------------------------------------------------------- repeats

// Claim reports whether an event is new — false when the same key was claimed
// less than the repeat window ago — and remembers it when it is. The event
// claimed is the one about to be stored: a caller whose write then fails hands
// the key back with Release, so that a refused event is not mistaken for a
// stored one.
func (g *Gate) Claim(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	if until, seen := g.repeats[key]; seen && now.Before(until) {
		return false
	}
	if now.After(g.nextSweep) || len(g.repeats) >= g.maxKeys {
		g.sweep(now)
	}
	if len(g.repeats) < g.maxKeys {
		g.repeats[key] = now.Add(g.window)
	}
	return true
}

// Release forgets a claim whose event was not stored after all.
func (g *Gate) Release(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.repeats, key)
}

// sweep drops every expired claim. It runs at most once per window, or when
// the set is full, so its cost is spread over a window's worth of events.
func (g *Gate) sweep(now time.Time) {
	for key, until := range g.repeats {
		if !now.Before(until) {
			delete(g.repeats, key)
		}
	}
	g.nextSweep = now.Add(g.window)
}

// --------------------------------------------------------------- daily cap

// Counter counts the events a business has already stored since the given
// instant — the database's answer, asked once per business per day.
type Counter func(ctx context.Context, businessID uuid.UUID, since time.Time) (int, error)

// Admit decides whether one more event of the business may be stored today.
// When it may, the event is counted, and a caller whose write then fails hands
// it back with Return. When it may not, first reports whether this is the
// business's first refusal of the day — the one worth a log line.
//
// The first time a business is seen on a day its count is read through count,
// outside the gate's lock, so a restart does not hand every business a fresh
// allowance. Two requests of the same business may both read it; the first to
// finish wins and the other's reading is dropped, which can at most let the
// events stored in between go uncounted.
func (g *Gate) Admit(ctx context.Context, businessID uuid.UUID, count Counter) (ok, first bool, err error) {
	if g.cap == 0 {
		return true, false, nil
	}

	now := g.now().In(g.zone)
	day := now.Format("2006-01-02")

	g.mu.Lock()
	g.rollDay(day)
	entry := g.counts[businessID]
	g.mu.Unlock()

	if entry == nil {
		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, g.zone)
		stored, err := count(ctx, businessID, midnight)
		if err != nil {
			return false, false, err
		}
		g.mu.Lock()
		g.rollDay(day)
		if entry = g.counts[businessID]; entry == nil {
			entry = &dailyCount{stored: stored}
			g.counts[businessID] = entry
		}
		g.mu.Unlock()
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if entry.stored >= g.cap {
		first = !entry.reported
		entry.reported = true
		return false, first, nil
	}
	entry.stored++
	return true, false, nil
}

// Return hands back an event Admit counted but the caller did not store.
func (g *Gate) Return(businessID uuid.UUID) {
	if g.cap == 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if entry := g.counts[businessID]; entry != nil && entry.stored > 0 {
		entry.stored--
	}
}

// rollDay drops the counts of an earlier day. The caller holds g.mu.
func (g *Gate) rollDay(day string) {
	if g.day != day {
		g.day = day
		g.counts = make(map[uuid.UUID]*dailyCount)
	}
}
