// Package ipexclude decides whose visits the analytics never store: the owner
// looking at their own menu from the café's Wi-Fi, a waiter showing the menu to
// a table from the counter, the platform operator opening a tenant's page to
// check a deploy. Counted as visitors, those are the loudest traffic a small
// menu has — dozens of views a day from the one person who least needs them
// counted — and they drown out what the dashboard is for.
//
// Three mechanisms keep them out, and handlers.TrackEvent consults all three
// before anything else looks at the event:
//
//   - a per-business list of addresses and ranges the owner manages in the
//     panel (the analytics_excluded_ips table of migration 015), read through
//     the cache in this package;
//   - a platform-wide list from ANALYTICS_EXCLUDED_IPS, for the operator, that
//     applies to every tenant and is never shown by any endpoint;
//   - a per-browser opt-out cookie (middleware.AnalyticsOptedOut), for the
//     owner whose address changes every time the phone changes networks.
//
// This package holds the first two: the rules for reading an address or a
// range as a person types it (Parse), the matching (Contains), and the Set that
// keeps the per-business lists in memory. It is pure apart from the Loader a
// Set is handed, so everything that decides what counts as "the same address"
// can be tested without a database.
package ipexclude

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MaxPerBusiness is how many entries one business's list may hold. A café has
// one or two addresses worth excluding, a chain a handful per branch; fifty is
// far beyond either, and the bound keeps the per-event match — a linear walk
// over the list — trivially cheap.
const MaxPerBusiness = 50

// MaxLabelRunes bounds an entry's label ("Kasa", "Ev", "Ofis Wi-Fi"). The
// column's CHECK holds the same number.
const MaxLabelRunes = 60

// The broadest ranges an entry may name.
//
// An exclusion is invisible in the dashboard by design — the visits it drops
// simply never appear — so a range that is too broad does its damage silently.
// A /16 is 65 536 IPv4 addresses: a whole carrier's pool in a city, already far
// more than any one owner's networks. A /48 is what an IPv6 provider hands one
// site, the natural unit for "my shop's connection". Anything broader would
// start excluding the very customers the analytics exist to count.
const (
	MinIPv4Bits = 16
	MinIPv6Bits = 48
)

// DefaultTTL is how long a Set trusts a business's list before reading it
// again. The list changes through two endpoints only, and both invalidate the
// entry the moment their write commits, so the TTL is not what makes a change
// visible — it only bounds how long a change made outside this process (by
// hand in psql, say) can go unnoticed.
const DefaultTTL = 60 * time.Second

// The errors of Parse.
var (
	// ErrInvalid is anything that is not an IPv4 or IPv6 address or CIDR range.
	ErrInvalid = errors.New("not an IP address or a CIDR range")

	// ErrTooBroad is a range broader than MinIPv4Bits / MinIPv6Bits.
	ErrTooBroad = errors.New("the range is broader than /16 for IPv4 or /48 for IPv6")
)

// Parse reads one address or CIDR range as a person or an environment variable
// writes it, and returns it in the one form every comparison uses:
//
//	"198.18.139.87"        -> 198.18.139.87/32   a single address is a /32 or a /128
//	"198.18.139.87/24"     -> 198.18.139.0/24    host bits are masked off
//	"::ffff:198.18.139.87" -> 198.18.139.87/32   IPv4-mapped IPv6 is unmapped
//	"2001:DB8::1"          -> 2001:db8::1/128    IPv6 is lowercased and compressed
//
// Unmapping matters because the visitor side is unmapped too
// (clientip.NormalizeIP): the same phone reached over a dual-stack socket must
// not slip past its own entry. Masking matters because PostgreSQL's cidr type
// refuses a value with host bits set, and because "198.18.139.87/24" and
// "198.18.139.0/24" are the same range and must be the same entry.
//
// A zone ("fe80::1%eth0") and a port ("1.2.3.4:80") are refused rather than
// stripped: neither ever appears in a visitor's address, so either one means
// the input is not what the person thought it was.
func Parse(raw string) (netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return netip.Prefix{}, ErrInvalid
	}

	var prefix netip.Prefix
	if strings.Contains(raw, "/") {
		parsed, err := netip.ParsePrefix(raw)
		if err != nil {
			return netip.Prefix{}, ErrInvalid
		}
		prefix = parsed
	} else {
		addr, err := netip.ParseAddr(raw)
		if err != nil || addr.Zone() != "" {
			return netip.Prefix{}, ErrInvalid
		}
		prefix = netip.PrefixFrom(addr, addr.BitLen())
	}

	prefix = unmap(prefix).Masked()
	if !prefix.IsValid() {
		return netip.Prefix{}, ErrInvalid
	}
	if TooBroad(prefix) {
		return netip.Prefix{}, ErrTooBroad
	}
	return prefix, nil
}

// unmap turns an IPv4-mapped IPv6 range into the IPv4 range it maps, when it
// lies entirely inside ::ffff:0:0/96. A mapped range broader than that also
// covers addresses that are not mapped IPv4 at all, so it stays IPv6 — where it
// matches nothing any visitor ever reports, and is harmless.
func unmap(prefix netip.Prefix) netip.Prefix {
	addr := prefix.Addr()
	if !addr.Is4In6() || prefix.Bits() < 96 {
		return prefix
	}
	return netip.PrefixFrom(addr.Unmap(), prefix.Bits()-96)
}

// TooBroad reports whether a range is broader than an entry may be.
func TooBroad(prefix netip.Prefix) bool {
	if prefix.Addr().Is4() {
		return prefix.Bits() < MinIPv4Bits
	}
	return prefix.Bits() < MinIPv6Bits
}

// Display is how an entry is shown: the bare address for a single host, the
// CIDR text for a range — "198.18.139.87", not "198.18.139.87/32", because
// that is how the owner typed it and how the visit log prints it.
func Display(prefix netip.Prefix) string {
	if prefix.IsSingleIP() {
		return prefix.Addr().String()
	}
	return prefix.String()
}

// Rejected is one entry of a list that Parse refused.
type Rejected struct {
	Value string
	Err   error
}

// ParseList reads ANALYTICS_EXCLUDED_IPS: addresses and ranges separated by
// commas, semicolons or any whitespace, in any mix — the forms people paste
// from a spreadsheet, a firewall config or a chat message. Every entry goes
// through Parse, so the operator's list obeys exactly the rules the panel's
// does, the breadth limit included: a stray "0.0.0.0/0" would otherwise
// silently erase every tenant's analytics. An entry that appears twice is kept
// once. What Parse refused comes back in rejected, in input order, for the
// start-up log.
func ParseList(raw string) (prefixes []netip.Prefix, rejected []Rejected) {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	seen := make(map[netip.Prefix]bool, len(fields))
	prefixes = make([]netip.Prefix, 0, len(fields))
	for _, field := range fields {
		prefix, err := Parse(field)
		if err != nil {
			rejected = append(rejected, Rejected{Value: field, Err: err})
			continue
		}
		if seen[prefix] {
			continue
		}
		seen[prefix] = true
		prefixes = append(prefixes, prefix)
	}
	return prefixes, rejected
}

// visitorAddr reads the address a request was resolved to
// (middleware.ClientAddrOf(c).IP). The resolver's "-" — nothing could be
// established — and anything else that does not parse is not an address, and
// an unknown address never matches any entry: excluding "whoever we could not
// identify" would drop real visitors behind a misconfigured edge, not the
// owner.
func visitorAddr(ip string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.WithZone("").Unmap(), true
}

// Contains reports whether any of the ranges covers the address. An IPv4
// range never covers an IPv6 address and the other way round (netip.Prefix
// compares families first).
func Contains(prefixes []netip.Prefix, ip string) bool {
	addr, ok := visitorAddr(ip)
	return ok && containsAddr(prefixes, addr)
}

func containsAddr(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// ------------------------------------------------------------------ the Set

// Loader reads one business's list from the database — the Set's only way to
// learn it. It is called outside the Set's lock.
type Loader func(ctx context.Context, businessID uuid.UUID) ([]netip.Prefix, error)

// Config configures a Set.
type Config struct {
	// Platform is the ANALYTICS_EXCLUDED_IPS list, already parsed
	// (config.Load). It applies to every business.
	Platform []netip.Prefix

	// Load reads a business's own list. Nil means no business has one.
	Load Loader

	// TTL is DefaultTTL when zero or negative.
	TTL time.Duration

	// Now is time.Now when nil; a test sets it to move the clock.
	Now func() time.Time
}

// Set answers "is this visit excluded?" for every event the public endpoint
// accepts, so it is on the hot path of the one endpoint the whole internet
// calls: the platform list is a slice in memory and a business's own list is
// read from the database at most once per TTL — or once after each change to
// it, see Invalidate. It is safe for concurrent use.
//
// Like the event gate and the sessions it is state of this one process; the
// deployment runs a single instance.
type Set struct {
	platform []netip.Prefix
	load     Loader
	ttl      time.Duration
	now      func() time.Time

	mu      sync.Mutex
	entries map[uuid.UUID]cachedList

	// generation moves on every Invalidate. A load remembers it before it goes
	// to the database and caches its answer only when it has not moved since:
	// a list read just before an add committed, and handed back just after the
	// add invalidated the entry, would otherwise be cached for a whole TTL —
	// the one outcome invalidation exists to prevent. One counter for every
	// business rather than one each keeps the map from growing; a load that
	// raced another business's change merely goes uncached once.
	generation uint64

	// nextSweep is when expired entries are next dropped, so that businesses
	// whose menus nobody opens any more do not stay in the map for good.
	nextSweep time.Time
}

// cachedList is one business's list and the moment it stops being trusted.
type cachedList struct {
	prefixes []netip.Prefix
	expires  time.Time
}

// New builds a Set. The platform list is copied.
func New(cfg Config) *Set {
	s := &Set{
		platform: append([]netip.Prefix(nil), cfg.Platform...),
		load:     cfg.Load,
		ttl:      cfg.TTL,
		now:      cfg.Now,
		entries:  make(map[uuid.UUID]cachedList),
	}
	if s.ttl <= 0 {
		s.ttl = DefaultTTL
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

// PlatformCount is how many entries the platform list holds — what the
// start-up log reports. The entries themselves are deliberately not exposed.
func (s *Set) PlatformCount() int { return len(s.platform) }

// Excludes reports whether a visit from ip to the business is never to be
// stored: when the platform list or the business's own list covers the
// address. An address that could not be established never matches. The error
// is the Loader's, when the business's list had to be read and could not be.
func (s *Set) Excludes(ctx context.Context, businessID uuid.UUID, ip string) (bool, error) {
	addr, ok := visitorAddr(ip)
	if !ok {
		return false, nil
	}
	if containsAddr(s.platform, addr) {
		return true, nil
	}
	prefixes, err := s.businessList(ctx, businessID)
	if err != nil {
		return false, err
	}
	return containsAddr(prefixes, addr), nil
}

// Invalidate forgets a business's cached list, so that the next event of the
// business reads it afresh. The endpoints that add and remove entries call it
// as soon as their write has committed: the owner who has just added their
// address expects the very next view of the menu not to be counted, not the
// one after the TTL.
func (s *Set) Invalidate(businessID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, businessID)
	s.generation++
}

// businessList returns the business's list, from memory while it is fresh.
func (s *Set) businessList(ctx context.Context, businessID uuid.UUID) ([]netip.Prefix, error) {
	if s.load == nil {
		return nil, nil
	}

	s.mu.Lock()
	now := s.now()
	if cached, ok := s.entries[businessID]; ok && now.Before(cached.expires) {
		s.mu.Unlock()
		return cached.prefixes, nil
	}
	generation := s.generation
	s.mu.Unlock()

	// Outside the lock: one slow database read must not hold up the events of
	// every other business. Two events of the same business that miss together
	// both read the list — at most once per TTL, bounded by the rate limiter in
	// front of the endpoint, and cheaper than making the second one wait.
	prefixes, err := s.load(ctx, businessID)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now = s.now()
	if s.generation == generation {
		s.entries[businessID] = cachedList{prefixes: prefixes, expires: now.Add(s.ttl)}
	}
	if now.After(s.nextSweep) {
		for id, cached := range s.entries {
			if !now.Before(cached.expires) {
				delete(s.entries, id)
			}
		}
		s.nextSweep = now.Add(s.ttl)
	}
	return prefixes, nil
}
