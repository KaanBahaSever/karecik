package ipexclude_test

// The exclusion rules on their own, without a database: how an address or a
// range is read and normalised, how broad a range may be, how a list from the
// environment is split, what matches a visitor's address — and the Set that
// caches each business's list, with a clock the test moves.
//
// The properties that matter:
//
//   - one address written several ways is one entry (mapped, masked, cased);
//   - a range too broad to be "my own network" is refused on every path;
//   - an address the resolver could not establish never matches anything;
//   - a change to a business's list takes effect at once after Invalidate, and
//     a list read before the change is never cached after it.

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"karecik/backend/internal/ipexclude"
)

func TestParseNormalisesEveryWayOfWritingAnAddress(t *testing.T) {
	for _, tc := range []struct {
		raw, cidr, display string
	}{
		{"198.18.139.87", "198.18.139.87/32", "198.18.139.87"},
		{"  198.18.139.87\t", "198.18.139.87/32", "198.18.139.87"},
		{"198.18.139.87/32", "198.18.139.87/32", "198.18.139.87"},
		// Host bits are masked: the same /24 however it is written.
		{"198.18.139.87/24", "198.18.139.0/24", "198.18.139.0/24"},
		{"198.18.139.0/24", "198.18.139.0/24", "198.18.139.0/24"},
		{"10.20.0.0/16", "10.20.0.0/16", "10.20.0.0/16"},
		// IPv4-mapped IPv6 is the IPv4 address, single or ranged.
		{"::ffff:198.18.139.87", "198.18.139.87/32", "198.18.139.87"},
		{"::ffff:198.18.139.87/120", "198.18.139.0/24", "198.18.139.0/24"},
		// IPv6 is lowercased and compressed.
		{"2001:DB8:0:0::1", "2001:db8::1/128", "2001:db8::1"},
		{"2001:db8::1/128", "2001:db8::1/128", "2001:db8::1"},
		{"2001:db8:abcd:12::1/64", "2001:db8:abcd:12::/64", "2001:db8:abcd:12::/64"},
		{"2001:db8:abcd::/48", "2001:db8:abcd::/48", "2001:db8:abcd::/48"},
	} {
		prefix, err := ipexclude.Parse(tc.raw)
		if err != nil {
			t.Errorf("Parse(%q) failed: %v", tc.raw, err)
			continue
		}
		if prefix.String() != tc.cidr {
			t.Errorf("Parse(%q) = %s, want %s", tc.raw, prefix, tc.cidr)
		}
		if got := ipexclude.Display(prefix); got != tc.display {
			t.Errorf("Display(Parse(%q)) = %q, want %q", tc.raw, got, tc.display)
		}
	}
}

func TestParseRefusesWhatIsNotAnAddressOrARange(t *testing.T) {
	for _, raw := range []string{
		"", "   ", "abc", "198.18.139", "198.18.139.256", "198.18.139.87/33",
		"198.18.139.87/", "/24", "2001:db8::1/129", "2001:db8:::1",
		"fe80::1%eth0",      // a zone never appears in a visitor's address
		"198.18.139.87:443", // nor does a port
		"[2001:db8::1]", "198.18.139.87 10.0.0.1", "198.18.139.87\x00", "1.2.3.4/24/8",
	} {
		if prefix, err := ipexclude.Parse(raw); !errors.Is(err, ipexclude.ErrInvalid) {
			t.Errorf("Parse(%q) = %v, %v — want ErrInvalid", raw, prefix, err)
		}
	}
}

func TestParseRefusesRangesBroaderThanOneSite(t *testing.T) {
	accepted := []string{"198.18.0.0/16", "198.18.255.255/16", "2001:db8:1::/48", "2001:db8:1:2::/64"}
	for _, raw := range accepted {
		if _, err := ipexclude.Parse(raw); err != nil {
			t.Errorf("Parse(%q) failed at the limit: %v", raw, err)
		}
	}
	refused := []string{
		"198.18.0.0/15", "176.0.0.0/8", "0.0.0.0/0",
		"2001:db8::/47", "2001:db8::/32", "::/0",
		// A mapped range is judged as the IPv4 range it is.
		"::ffff:198.18.0.0/111", "::ffff:0:0/96",
	}
	for _, raw := range refused {
		if prefix, err := ipexclude.Parse(raw); !errors.Is(err, ipexclude.ErrTooBroad) {
			t.Errorf("Parse(%q) = %v, %v — want ErrTooBroad", raw, prefix, err)
		}
	}
}

func TestParseListSplitsOnEverySeparatorAndReportsWhatItSkips(t *testing.T) {
	raw := " 198.18.139.87, 203.0.113.0/24;2001:db8:77::/48\n bogus\t10.0.0.0/8 ;; 198.18.139.87 ,::ffff:198.18.139.87, 1.2.3.4/33 "
	prefixes, rejected := ipexclude.ParseList(raw)

	want := []string{"198.18.139.87/32", "203.0.113.0/24", "2001:db8:77::/48"}
	if len(prefixes) != len(want) {
		t.Fatalf("ParseList kept %v, want %v", prefixes, want)
	}
	for i, prefix := range prefixes {
		if prefix.String() != want[i] {
			t.Errorf("entry %d = %s, want %s", i, prefix, want[i])
		}
	}

	if len(rejected) != 3 {
		t.Fatalf("ParseList rejected %v, want bogus, 10.0.0.0/8 and 1.2.3.4/33", rejected)
	}
	for i, tc := range []struct {
		value string
		err   error
	}{{"bogus", ipexclude.ErrInvalid}, {"10.0.0.0/8", ipexclude.ErrTooBroad}, {"1.2.3.4/33", ipexclude.ErrInvalid}} {
		if rejected[i].Value != tc.value || !errors.Is(rejected[i].Err, tc.err) {
			t.Errorf("rejected %d = %q (%v), want %q (%v)", i, rejected[i].Value, rejected[i].Err, tc.value, tc.err)
		}
	}

	if prefixes, rejected := ipexclude.ParseList(""); len(prefixes) != 0 || len(rejected) != 0 {
		t.Errorf("an empty list parsed to %v / %v", prefixes, rejected)
	}
	if prefixes, rejected := ipexclude.ParseList(" ,; \n"); len(prefixes) != 0 || len(rejected) != 0 {
		t.Errorf("a list of separators parsed to %v / %v", prefixes, rejected)
	}
}

func mustParse(t *testing.T, raws ...string) []netip.Prefix {
	t.Helper()
	prefixes := make([]netip.Prefix, 0, len(raws))
	for _, raw := range raws {
		prefix, err := ipexclude.Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q): %v", raw, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

func TestContainsMatchesTheVisitorsResolvedAddress(t *testing.T) {
	list := mustParse(t, "198.18.139.87", "203.0.113.0/24", "2001:db8:1::/48")

	for _, ip := range []string{
		"198.18.139.87", "::ffff:198.18.139.87", "203.0.113.0", "203.0.113.255",
		"2001:db8:1::1", "2001:db8:1:ffff::abcd", "2001:DB8:1::2",
	} {
		if !ipexclude.Contains(list, ip) {
			t.Errorf("Contains(%q) = false", ip)
		}
	}
	for _, ip := range []string{
		"198.18.139.86", "198.18.139.88", "203.0.114.1", "2001:db8:2::1",
		// The resolver's "nothing could be established", and anything else
		// that is not an address, never match.
		"-", "", "unknown", "0.0.0.0/0", "198.18.139.87/32",
	} {
		if ipexclude.Contains(list, ip) {
			t.Errorf("Contains(%q) = true", ip)
		}
	}
	if ipexclude.Contains(nil, "198.18.139.87") {
		t.Error("an empty list matched")
	}
}

// ------------------------------------------------------------------ the Set

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

// fakeStore is a Loader over a map, counting its reads.
type fakeStore struct {
	mu    sync.Mutex
	lists map[uuid.UUID][]netip.Prefix
	reads atomic.Int64
	err   error
	// during runs inside a read, after the list was taken: what a write that
	// commits while the read is in flight looks like.
	during func()
}

func (s *fakeStore) set(id uuid.UUID, list []netip.Prefix) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lists[id] = list
}

func (s *fakeStore) load(_ context.Context, id uuid.UUID) ([]netip.Prefix, error) {
	s.reads.Add(1)
	s.mu.Lock()
	list, err, during := s.lists[id], s.err, s.during
	s.during = nil
	s.mu.Unlock()
	if during != nil {
		during()
	}
	return list, err
}

func newStore() *fakeStore { return &fakeStore{lists: make(map[uuid.UUID][]netip.Prefix)} }

func mustExcludes(t *testing.T, set *ipexclude.Set, id uuid.UUID, ip string) bool {
	t.Helper()
	excluded, err := set.Excludes(context.Background(), id, ip)
	if err != nil {
		t.Fatalf("Excludes(%s, %q): %v", id, ip, err)
	}
	return excluded
}

func TestTheSetCachesEachBusinessForItsTTL(t *testing.T) {
	store := newStore()
	clk := &clock{now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	set := ipexclude.New(ipexclude.Config{Load: store.load, TTL: time.Minute, Now: clk.Now})
	shop, other := uuid.New(), uuid.New()
	store.set(shop, mustParse(t, "198.18.139.87"))

	if !mustExcludes(t, set, shop, "198.18.139.87") {
		t.Fatal("the business's own address was not excluded")
	}
	if mustExcludes(t, set, other, "198.18.139.87") {
		t.Error("one business's list excluded another business's visitor")
	}
	for i := 0; i < 10; i++ {
		mustExcludes(t, set, shop, "198.51.100.1")
	}
	if n := store.reads.Load(); n != 2 {
		t.Errorf("twelve lookups of two businesses read the database %d times, want 2", n)
	}

	// A change made behind the Set's back is seen once the TTL has passed, and
	// not a moment before.
	store.set(shop, mustParse(t, "198.51.100.1"))
	clk.Advance(time.Minute - time.Second)
	if mustExcludes(t, set, shop, "198.51.100.1") {
		t.Error("an entry added behind the Set's back was seen before the TTL passed")
	}
	clk.Advance(time.Second)
	if !mustExcludes(t, set, shop, "198.51.100.1") {
		t.Error("an entry added behind the Set's back was not seen after the TTL")
	}
	if mustExcludes(t, set, shop, "198.18.139.87") {
		t.Error("a removed entry still matched after the TTL")
	}
}

func TestInvalidateTakesEffectAtOnce(t *testing.T) {
	store := newStore()
	clk := &clock{now: time.Now()}
	set := ipexclude.New(ipexclude.Config{Load: store.load, TTL: time.Hour, Now: clk.Now})
	shop, other := uuid.New(), uuid.New()
	store.set(other, mustParse(t, "203.0.113.9"))

	if mustExcludes(t, set, shop, "198.18.139.87") || !mustExcludes(t, set, other, "203.0.113.9") {
		t.Fatal("the lists were not read as stored")
	}
	store.set(shop, mustParse(t, "198.18.139.87"))
	set.Invalidate(shop)
	if !mustExcludes(t, set, shop, "198.18.139.87") {
		t.Error("an added entry was not seen right after Invalidate")
	}
	store.set(shop, nil)
	set.Invalidate(shop)
	if mustExcludes(t, set, shop, "198.18.139.87") {
		t.Error("a removed entry still matched right after Invalidate")
	}

	// Another business's cached list is untouched: invalidating one does not
	// send every business back to the database.
	before := store.reads.Load()
	mustExcludes(t, set, other, "203.0.113.9")
	if store.reads.Load() != before {
		t.Error("invalidating one business dropped another's cached list")
	}
}

// A list read just before a change commits, and handed back just after the
// change invalidated the business, must not be cached: it would hide the change
// for a whole TTL, which is exactly what Invalidate is there to prevent.
func TestAListReadBeforeAChangeIsNotCachedAfterIt(t *testing.T) {
	store := newStore()
	clk := &clock{now: time.Now()}
	set := ipexclude.New(ipexclude.Config{Load: store.load, TTL: time.Hour, Now: clk.Now})
	shop := uuid.New()

	store.during = func() {
		// The add commits and invalidates while the read above is in flight.
		store.set(shop, mustParse(t, "198.18.139.87"))
		set.Invalidate(shop)
	}
	if mustExcludes(t, set, shop, "198.18.139.87") {
		t.Fatal("the read that raced the add saw the add — the race was not staged")
	}
	if !mustExcludes(t, set, shop, "198.18.139.87") {
		t.Error("the stale list read before the add was cached after it")
	}
}

func TestThePlatformListAppliesToEveryBusinessWithoutADatabaseRead(t *testing.T) {
	store := newStore()
	set := ipexclude.New(ipexclude.Config{
		Platform: mustParse(t, "192.0.2.77", "2001:db8:77::/48"),
		Load:     store.load,
	})
	if set.PlatformCount() != 2 {
		t.Errorf("PlatformCount() = %d, want 2", set.PlatformCount())
	}
	for _, id := range []uuid.UUID{uuid.New(), uuid.New()} {
		if !mustExcludes(t, set, id, "192.0.2.77") || !mustExcludes(t, set, id, "2001:db8:77:1::5") {
			t.Error("a platform entry did not apply to a business")
		}
	}
	if n := store.reads.Load(); n != 0 {
		t.Errorf("platform matches read the database %d times, want 0", n)
	}
	// An unknown address never matches, and costs no read either.
	if mustExcludes(t, set, uuid.New(), "-") || store.reads.Load() != 0 {
		t.Error("an unknown address matched, or was looked up")
	}
}

func TestALoadErrorIsReportedAndNotCached(t *testing.T) {
	store := newStore()
	set := ipexclude.New(ipexclude.Config{Load: store.load})
	shop := uuid.New()
	store.set(shop, mustParse(t, "198.18.139.87"))

	store.err = errors.New("database unreachable")
	if _, err := set.Excludes(context.Background(), shop, "198.18.139.87"); err == nil {
		t.Fatal("a failed read was reported as an answer")
	}
	store.err = nil
	if !mustExcludes(t, set, shop, "198.18.139.87") {
		t.Error("the failed read was cached as an empty list")
	}

	// Without a loader, no business has a list.
	bare := ipexclude.New(ipexclude.Config{})
	if mustExcludes(t, bare, shop, "198.18.139.87") {
		t.Error("a Set without a loader excluded a visitor")
	}
}

// Many events of many businesses at once, with lists changing underneath:
// run with -race, this is what proves the Set's locking.
func TestTheSetIsSafeForConcurrentUse(t *testing.T) {
	store := newStore()
	set := ipexclude.New(ipexclude.Config{Load: store.load, TTL: time.Millisecond})
	ids := make([]uuid.UUID, 8)
	for i := range ids {
		ids[i] = uuid.New()
		store.set(ids[i], mustParse(t, fmt.Sprintf("198.51.100.%d", i+1)))
	}

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				id := ids[(g+i)%len(ids)]
				if i%50 == 0 {
					set.Invalidate(id)
				}
				want := (g+i)%len(ids) + 1
				excluded, err := set.Excludes(context.Background(), id, fmt.Sprintf("198.51.100.%d", want))
				if err != nil || !excluded {
					errs <- fmt.Errorf("business %d: excluded=%v err=%v", want, excluded, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
