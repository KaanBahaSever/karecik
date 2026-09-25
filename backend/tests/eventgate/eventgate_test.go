package eventgate_test

// The event gate on its own, with a clock the test moves: which events count
// as repeats and for how long, how the daily cap counts and when it forgets,
// and that the repeat set stays bounded.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"karecik/backend/internal/eventgate"
)

// clock is a settable time source.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var istanbul = time.FixedZone("Europe/Istanbul", 3*60*60)

func newClock() *clock {
	return &clock{now: time.Date(2026, 9, 25, 12, 0, 0, 0, istanbul)}
}

// counting is a Counter that answers stored and records how often it was asked.
type counting struct {
	stored int
	calls  int
	since  []time.Time
}

func (c *counting) count(_ context.Context, _ uuid.UUID, since time.Time) (int, error) {
	c.calls++
	c.since = append(c.since, since)
	return c.stored, nil
}

func TestRepeatsWithinTheWindowAreDropped(t *testing.T) {
	clk := newClock()
	gate := eventgate.New(eventgate.Config{Now: clk.Now})

	if !gate.Claim("v:a|menu_view|m|-|-") {
		t.Fatal("the first event was taken for a repeat")
	}
	if gate.Claim("v:a|menu_view|m|-|-") {
		t.Error("the same event at the same moment was not a repeat")
	}
	if !gate.Claim("v:b|menu_view|m|-|-") || !gate.Claim("v:a|product_view|m|-|p") {
		t.Error("another visitor or another target was taken for a repeat")
	}

	clk.Advance(eventgate.DefaultRepeatWindow - time.Millisecond)
	if gate.Claim("v:a|menu_view|m|-|-") {
		t.Error("the same event just inside the window was not a repeat")
	}
	clk.Advance(time.Millisecond)
	if !gate.Claim("v:a|menu_view|m|-|-") {
		t.Error("the same event once the window had passed was still a repeat")
	}
}

func TestAReleasedClaimIsNotARepeat(t *testing.T) {
	gate := eventgate.New(eventgate.Config{Now: newClock().Now})
	if !gate.Claim("k") {
		t.Fatal("the first claim was refused")
	}
	gate.Release("k")
	if !gate.Claim("k") {
		t.Error("an event whose write failed was remembered as stored")
	}
}

func TestTheRepeatSetIsBounded(t *testing.T) {
	clk := newClock()
	gate := eventgate.New(eventgate.Config{Now: clk.Now, MaxRepeatKeys: 3})

	for i := 0; i < 3; i++ {
		if !gate.Claim(fmt.Sprintf("k%d", i)) {
			t.Fatalf("claim %d refused", i)
		}
	}
	// Full: a new key is let through but not remembered.
	if !gate.Claim("k3") || !gate.Claim("k3") {
		t.Error("with the set full, a new event was refused instead of passed through unremembered")
	}
	// The remembered ones still are.
	if gate.Claim("k0") {
		t.Error("a remembered event stopped being a repeat when the set filled up")
	}
	// Once they expire the set makes room again.
	clk.Advance(eventgate.DefaultRepeatWindow)
	if !gate.Claim("k4") || gate.Claim("k4") {
		t.Error("the set did not make room once its entries expired")
	}
}

func TestTheDailyCapCountsPerBusinessAndDay(t *testing.T) {
	clk := newClock()
	gate := eventgate.New(eventgate.Config{Now: clk.Now, DailyCap: 3, Zone: istanbul})
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	db := &counting{}

	admit := func(id uuid.UUID) (bool, bool) {
		t.Helper()
		ok, first, err := gate.Admit(ctx, id, db.count)
		if err != nil {
			t.Fatalf("Admit: %v", err)
		}
		return ok, first
	}

	for i := 1; i <= 3; i++ {
		if ok, _ := admit(a); !ok {
			t.Fatalf("event %d of the day was refused under a cap of 3", i)
		}
	}
	if ok, first := admit(a); ok || !first {
		t.Errorf("the 4th event: admitted %v, first refusal %v — want refused, first", ok, first)
	}
	if ok, first := admit(a); ok || first {
		t.Errorf("the 5th event: admitted %v, first refusal %v — want refused, not first again", ok, first)
	}
	if ok, _ := admit(b); !ok {
		t.Error("another business was refused because the first reached its cap")
	}
	if db.calls != 2 {
		t.Errorf("the database was asked %d times, want once per business", db.calls)
	}
	wantMidnight := time.Date(2026, 9, 25, 0, 0, 0, 0, istanbul)
	if !db.since[0].Equal(wantMidnight) {
		t.Errorf("the count was asked since %s, want the day's midnight %s", db.since[0], wantMidnight)
	}

	// A new day starts from nothing — and asks the database again.
	clk.Advance(12 * time.Hour)
	if ok, _ := admit(a); !ok {
		t.Error("the cap of yesterday still applied today")
	}
	if db.calls != 3 {
		t.Errorf("a new day did not ask the database for its count (%d calls)", db.calls)
	}
}

func TestTheDailyCapStartsFromWhatIsAlreadyStored(t *testing.T) {
	gate := eventgate.New(eventgate.Config{Now: newClock().Now, DailyCap: 3, Zone: istanbul})
	db := &counting{stored: 2}
	id := uuid.New()

	// A restarted process: two events of today are already in the table.
	if ok, _, _ := gate.Admit(context.Background(), id, db.count); !ok {
		t.Fatal("the 3rd event of the day was refused under a cap of 3")
	}
	if ok, first, _ := gate.Admit(context.Background(), id, db.count); ok || !first {
		t.Errorf("the 4th event of the day: admitted %v, first %v — a restart handed out a fresh allowance", ok, first)
	}
}

func TestAReturnedEventFreesItsPlace(t *testing.T) {
	gate := eventgate.New(eventgate.Config{Now: newClock().Now, DailyCap: 1, Zone: istanbul})
	db := &counting{}
	id := uuid.New()
	ctx := context.Background()

	if ok, _, _ := gate.Admit(ctx, id, db.count); !ok {
		t.Fatal("the first event was refused")
	}
	gate.Return(id)
	if ok, _, _ := gate.Admit(ctx, id, db.count); !ok {
		t.Error("an event whose write failed still counted against the cap")
	}
}

func TestNoCapAsksNothing(t *testing.T) {
	gate := eventgate.New(eventgate.Config{Now: newClock().Now})
	failing := func(context.Context, uuid.UUID, time.Time) (int, error) {
		return 0, errors.New("must not be asked")
	}
	for i := 0; i < 100; i++ {
		if ok, _, err := gate.Admit(context.Background(), uuid.New(), failing); !ok || err != nil {
			t.Fatalf("with no cap an event was refused: %v %v", ok, err)
		}
	}
	if gate.DailyCap() != 0 {
		t.Errorf("DailyCap %d, want 0", gate.DailyCap())
	}
}

func TestACountThatFailsAdmitsNothing(t *testing.T) {
	gate := eventgate.New(eventgate.Config{Now: newClock().Now, DailyCap: 5, Zone: istanbul})
	failing := func(context.Context, uuid.UUID, time.Time) (int, error) {
		return 0, errors.New("database down")
	}
	if ok, _, err := gate.Admit(context.Background(), uuid.New(), failing); ok || err == nil {
		t.Errorf("a failed count admitted %v with error %v, want a refusal carrying the error", ok, err)
	}
}

// Many requests of one business at once never store more than the cap.
func TestTheDailyCapHoldsUnderConcurrency(t *testing.T) {
	gate := eventgate.New(eventgate.Config{Now: newClock().Now, DailyCap: 50, Zone: istanbul})
	id := uuid.New()
	var (
		mu       sync.Mutex
		admitted int
		firsts   int
		wg       sync.WaitGroup
	)
	count := func(context.Context, uuid.UUID, time.Time) (int, error) { return 0, nil }
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, first, err := gate.Admit(context.Background(), id, count)
			if err != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if ok {
				admitted++
			}
			if first {
				firsts++
			}
		}()
	}
	wg.Wait()
	if admitted != 50 || firsts != 1 {
		t.Errorf("200 concurrent events under a cap of 50: %d admitted, %d first refusals — want 50 and 1",
			admitted, firsts)
	}
}
