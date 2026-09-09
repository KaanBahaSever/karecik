// Package clientip resolves the visitor's public address and source port from
// an inbound request, for audit logging.
//
// WHY THIS IS NOT ONE LINE
//
// Every candidate header is attacker-controlled unless something proves
// otherwise. The origin behind this app is reachable directly — Railway serves
// it on *.up.railway.app, and karecik.com itself currently resolves straight to
// Railway — so anyone can open a connection and set CF-Connecting-IP to whatever
// they like. A log line that records a forged address is worse than no log
// line: it is evidence that says the wrong thing, which is precisely what an
// audit trail must never do.
//
// So this package does not "get the IP". It answers a narrower question: what
// is the most trustworthy address available, and HOW trustworthy is it? Every
// result carries its Source, and the caller logs that alongside the value.
//
// THE DEPLOYMENT THIS IS BUILT FOR (verified, not assumed)
//
// Railway terminates TLS at its own edge and proxies to the container, so the
// TCP peer the process observes is a Railway internal address — the SAME one
// for every visitor on Earth. Anything keyed on the peer (a rate limiter, an
// audit field) is therefore global, not per-visitor.
//
// Railway's published specification lists the headers its edge sets:
//
//	X-Real-IP             the client's remote IP as the edge saw it
//	X-Forwarded-Proto     always "https"
//	X-Forwarded-Host      the original Host
//	X-Railway-Edge        the POP that handled the request
//	X-Request-Start       receipt time, Unix milliseconds
//	X-Railway-Request-Id  correlation id
//
// X-Forwarded-For is NOT on that list. It is not documented as set, overwritten
// or sanitised by the platform, which means a value arriving in it may simply
// be what the client typed. It is therefore treated here as untrusted input and
// used only as a last resort, clearly marked — never as the primary source, and
// never for anything that makes a security decision.
//
// Cloudflare sits in front of SOME of this domain's hostnames and not others
// (the apex is DNS-only; www and the tenant wildcard are proxied). CF headers
// are consequently present on some requests and absent on others, and the
// package must be correct either way rather than assuming a uniform edge.
//
// THE SOURCE PORT
//
// Under carrier-grade NAT many subscribers share one public address, so the
// address alone does not identify anyone; the source port is what makes an
// entry attributable. Nothing supplies it for free:
//
//   - Railway's edge does not forward the client port in any header.
//   - Cloudflare does not send it by default either. It has to be published
//     with an HTTP Request Header Modification transform rule that sets a
//     header from the cf.edge.client_port field.
//
// Which means the port is available ONLY for requests that came through
// Cloudflare, on a zone where that rule exists. Everything else records
// PortUnknown, honestly, rather than inventing a value.
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

// Source says where a resolved address came from, and therefore how much it is
// worth. It is logged with every line so that an entry can be weighed later
// instead of being taken at face value.
type Source string

const (
	// SourceCloudflare — CF-Connecting-IP on a request PROVEN to have come
	// through Cloudflare by the shared secret. Cloudflare overwrites this
	// header on the way in, so a client cannot dictate it. The strongest
	// answer available, and the only one that can carry a port.
	SourceCloudflare Source = "cloudflare"

	// SourceCloudflareUnverified — CF-Connecting-IP was present but nothing
	// proved the request came through Cloudflare, because no shared secret is
	// configured. Very probably genuine; not evidence.
	SourceCloudflareUnverified Source = "cloudflare-unverified"

	// SourceEdge — X-Real-IP, set by the hosting platform's edge. On a
	// hostname that Cloudflare proxies this is Cloudflare's address rather
	// than the visitor's, which is why it ranks below a verified CF header.
	SourceEdge Source = "edge"

	// SourceForwarded — an X-Forwarded-For entry. UNTRUSTED: the platform does
	// not document setting or sanitising this header, so it may be verbatim
	// client input. Present so that a request through some other proxy is not
	// logged as a total unknown, and marked so nobody mistakes it for proof.
	SourceForwarded Source = "forwarded-untrusted"

	// SourcePeer — the TCP peer. Behind this platform's edge that is an
	// internal address, identical for every visitor, so it identifies nobody.
	// Useful only when the process is reached directly.
	SourcePeer Source = "peer"

	// SourceUnknown — nothing usable at all.
	SourceUnknown Source = "unknown"
)

// PortUnknown is the port value when none could be established. A literal
// rather than "" so it is visible in a log line: an empty column reads as a
// formatting bug, "-" reads as "we looked and there was nothing".
const PortUnknown = "-"

// IPUnknown is the address value when nothing could be resolved.
const IPUnknown = "-"

// Header is the minimal view of a request this package needs.
//
// An interface rather than *http.Request because the server here is fasthttp
// (through Fiber) and never constructs an http.Request. Keeping the core over
// this one method makes it exhaustively testable with a map and no HTTP server
// at all, and lets both worlds share exactly the same logic instead of two
// implementations that drift.
type Header interface {
	Get(key string) string
}

// HeaderFunc adapts a plain function to Header.
type HeaderFunc func(key string) string

// Get implements Header.
func (f HeaderFunc) Get(key string) string { return f(key) }

// Addr is one resolved client address.
//
// It carries TWO addresses on purpose, because the two consumers want opposite
// things. An audit log wants the most INFORMATIVE value even when it cannot be
// proven — a real visitor address marked "unverified" is far more useful than a
// dash. A rate limiter wants the least FORGEABLE value, because a key the caller
// can vary is not a limit at all: every forged value opens a fresh bucket. Those
// are different questions, and answering both with one field means one of them
// gets the wrong answer.
type Addr struct {
	// IP is a normalised textual address, or IPUnknown. IPv4-mapped IPv6 is
	// unmapped and any zone is stripped, so the same client always produces
	// the same string and log lines can be grouped by it.
	//
	// This is the value to LOG. Weigh it together with Source.
	IP string

	// KeyIP is the most trustworthy address available, and never one the caller
	// chose. It is empty when nothing unforgeable could be established.
	//
	// This is the value to key a RATE LIMITER on, and it is deliberately not
	// always equal to IP: behind an unverified Cloudflare, IP is a header and
	// KeyIP is the platform edge's own X-Real-IP. Coarser, but an attacker
	// cannot move between buckets by editing a header.
	KeyIP string

	// Port is the client's source port, or PortUnknown.
	Port string

	// Source is which header the address came from.
	Source Source

	// Verified reports whether the origin of the value was proven rather than
	// assumed. Only SourceCloudflare with a matching shared secret sets it.
	Verified bool
}

// Resolver holds the trust configuration. The zero value is usable and safe:
// with no secret configured it simply never reports Verified.
type Resolver struct {
	// SecretHeader and Secret prove a request came through Cloudflare.
	//
	// This is the only proof available on this platform. The usual method —
	// checking that the TCP peer is in Cloudflare's published ranges — cannot
	// work here, because the peer is always the hosting platform's own proxy.
	// Authenticated Origin Pulls cannot work either: the platform terminates
	// TLS, so the application never sees a client certificate.
	//
	// What remains is a header Cloudflare adds and an attacker cannot guess.
	// Set it with a transform rule on the zone and give the origin the same
	// value. Leave either empty and CF headers are still used — they are the
	// best answer available — but reported as unverified.
	SecretHeader string
	Secret       string

	// PortHeader carries the client source port, published by a Cloudflare
	// transform rule from cf.edge.client_port. Empty disables port lookup.
	PortHeader string

	// TrustForwarded allows X-Forwarded-For as a last-resort source. Off by
	// default: on this platform the header is not set by the edge, so trusting
	// it would mean trusting the client. Turn it on only behind a proxy known
	// to overwrite it.
	TrustForwarded bool
}

// Header names. Canonical MIME form; both adapters look up case-insensitively.
const (
	HeaderCFConnectingIP = "CF-Connecting-IP"
	HeaderXRealIP        = "X-Real-IP"
	HeaderXForwardedFor  = "X-Forwarded-For"
	HeaderXClientPort    = "X-Client-Port"
)

// Resolve works out the best available address for one request.
//
// remoteAddr is the TCP peer, in "host:port" or bare-host form; it may be empty.
// The order below is by trustworthiness, not by convenience:
//
//  1. CF-Connecting-IP, when the shared secret matches. Proven.
//  2. CF-Connecting-IP without proof — the true client address whenever the
//     request really did come through Cloudflare, which is the common case.
//  3. X-Real-IP, set by the platform edge.
//  4. X-Forwarded-For, only if explicitly enabled.
//  5. The TCP peer.
//
// The two Cloudflare branches are the only ones that can carry a source port,
// because Cloudflare's transform rule is the only thing that publishes one.
//
// Addr.KeyIP is worked out separately and never takes a value from branch 1's
// header unless that branch PROVED itself — see the Addr doc for why.
//
// A malformed value at any step does not stop the walk: it is discarded and the
// next source is tried, so one junk header cannot blank the whole line.
func (r Resolver) Resolve(h Header, remoteAddr string) Addr {
	if h == nil {
		h = HeaderFunc(func(string) string { return "" })
	}

	// The keying floor, worked out FIRST and independently of whatever is
	// eventually reported.
	//
	// X-Real-IP is the header the hosting platform documents as "for
	// identifying client's remote IP", so the edge is what writes it. That is
	// the basis for keying on it — but note what is NOT established: the
	// platform does not publish whether it OVERWRITES a client-supplied value
	// or passes one through. If it were passthrough, this would be as forgeable
	// as the header above it. The TCP peer, by contrast, cannot be forged at
	// all, but behind the edge it is one internal address shared by everyone,
	// so keying on it is the global-bucket bug this replaced.
	//
	// So this is the best available floor, not a proof. The only configuration
	// that removes the assumption entirely is EDGE_SECRET plus a verified
	// Cloudflare header, which is why the startup warning asks for it.
	//
	// Neither value is necessarily the visitor — behind Cloudflare, X-Real-IP
	// is a Cloudflare egress address — but neither is CHOSEN by the caller,
	// and not-chosen is the property a rate-limit key actually requires.
	edgeIP, edgeOK := normalizeIP(h.Get(HeaderXRealIP))
	peerIP, peerPort, peerOK := splitAddr(remoteAddr)

	keyIP := ""
	switch {
	case edgeOK:
		keyIP = edgeIP
	case peerOK:
		keyIP = peerIP
	}

	cfIP, cfOK := normalizeIP(h.Get(HeaderCFConnectingIP))

	// 1 & 2: Cloudflare.
	if cfOK {
		if r.cloudflareProven(h) {
			// Proven, so the reported address is ALSO safe to key on. This is
			// the only configuration in which the limiter counts individual
			// visitors rather than whole swathes of them.
			return Addr{
				IP: cfIP, KeyIP: cfIP, Port: r.port(h),
				Source: SourceCloudflare, Verified: true,
			}
		}
		// Unverified: report the address, because it is the real visitor on
		// every honest request and a log of real addresses is worth more than a
		// column of dashes.
		//
		// The PORT is withheld, and the asymmetry is deliberate. One argument
		// says it should be reported too — it is exactly as trustworthy as the
		// address beside it, and Source already says how much that is. The
		// argument that wins is about harm: an address identifies a network,
		// and under carrier-grade NAT that may be thousands of subscribers,
		// whereas an address WITH a port identifies one of them. Precision
		// without provenance is the combination that puts a specific person in
		// an audit record on the strength of a header anyone can set.
		//
		// It also makes the missing configuration visible. The port column
		// reads "-" until EDGE_SECRET is set, which is a standing prompt to
		// finish the setup rather than a silently degraded feature.
		//
		// What also does NOT happen is this value becoming the limiter key. It
		// is a header; a header an attacker varies per request is a limiter
		// with unlimited buckets, which is worse than no limiter at all
		// because it looks like one.
		return Addr{
			IP: cfIP, KeyIP: keyIP, Port: PortUnknown,
			Source: SourceCloudflareUnverified,
		}
	}

	// 3: the platform's own header.
	if edgeOK {
		return Addr{IP: edgeIP, KeyIP: keyIP, Port: PortUnknown, Source: SourceEdge}
	}

	// 4: X-Forwarded-For, opt-in only. Reported, never keyed on: the platform
	// does not document sanitising it, so it may be verbatim client input.
	if r.TrustForwarded {
		if ip, ok := rightmostForwarded(h.Get(HeaderXForwardedFor)); ok {
			return Addr{IP: ip, KeyIP: keyIP, Port: PortUnknown, Source: SourceForwarded}
		}
	}

	// 5: the peer. Its port is the peer's, not the visitor's — behind a proxy
	// it is the proxy's ephemeral port and means nothing — so it is reported
	// only when the peer is also the address being reported.
	if peerOK {
		return Addr{IP: peerIP, KeyIP: peerIP, Port: peerPort, Source: SourcePeer}
	}

	return Addr{IP: IPUnknown, KeyIP: "", Port: PortUnknown, Source: SourceUnknown}
}

// cloudflareProven reports whether the shared secret is configured AND matched.
//
// The comparison is deliberately NOT constant time. The secret is compared
// against a header on every request, but a timing oracle needs the attacker to
// measure their own requests, and here every path returns the same response in
// the same time — the result only changes a log field. Reaching for
// crypto/subtle would imply a threat this does not have. If this value ever
// gates a response, that changes.
func (r Resolver) cloudflareProven(h Header) bool {
	if r.SecretHeader == "" || r.Secret == "" {
		return false
	}
	return h.Get(r.SecretHeader) == r.Secret
}

// port reads and validates the client source port header.
func (r Resolver) port(h Header) string {
	if r.PortHeader == "" {
		return PortUnknown
	}
	if p, ok := normalizePort(h.Get(r.PortHeader)); ok {
		return p
	}
	return PortUnknown
}

// ResolveRequest is the net/http adapter.
//
// Kept even though this server is fasthttp: it is the signature every Go
// example uses, so a future net/http admin endpoint, a test, or a copy of this
// package elsewhere gets the same logic rather than a second implementation.
func (r Resolver) ResolveRequest(req *http.Request) Addr {
	if req == nil {
		return Addr{IP: IPUnknown, Port: PortUnknown, Source: SourceUnknown}
	}
	return r.Resolve(HeaderFunc(req.Header.Get), req.RemoteAddr)
}

// IPPort is the two-value convenience form: clean strings, ready to log.
func (r Resolver) IPPort(h Header, remoteAddr string) (ip, port string) {
	a := r.Resolve(h, remoteAddr)
	return a.IP, a.Port
}

// ------------------------------------------------------------------ parsing

// trimHorizontal removes only spaces and tabs.
//
// NOT strings.TrimSpace, which also strips CR and LF. A header value that
// arrives carrying a newline did not come from a well-behaved client, and
// quietly trimming it into something valid means the suspicious input is
// accepted and the evidence of it discarded. Leaving the control character in
// place makes the value fail parsing, which is the outcome an audit log wants.
// Padding with real spaces does happen — some proxies write ", " chains — so
// that much is still tolerated.
func trimHorizontal(s string) string {
	return strings.Trim(s, " \t")
}

// hasControl reports whether a value contains a C0 control character or DEL.
//
// netip.ParseAddr does NOT validate the contents of an IPv6 zone: everything
// after "%" is taken verbatim, so a value like
// "fe80::1%eth0\n2026-01-01 FAKE ENTRY" parses happily, and the junk
// only disappears because the zone is stripped afterwards. The output is
// therefore already safe — but "accepted, then silently cleaned" is the
// wrong disposition on an audit path. A header carrying a newline did not
// come from a well-behaved client; refusing it and falling through to a
// source that is not lying is the better answer.
func hasControl(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}

// normalizeIP validates and canonicalises one address.
//
// Accepts a bare address or a host:port pair, because these headers are written
// by many different proxies and some include the port. Returns ok=false for
// anything it cannot make sense of, which is what keeps junk out of the log.
//
// Canonicalisation matters for grouping: the SAME client must always produce
// the same string, or counting distinct visitors by log line is wrong.
//
//	"::ffff:1.2.3.4"  -> "1.2.3.4"   IPv4-mapped IPv6 is unmapped
//	"fe80::1%eth0"    -> "fe80::1"   the zone is a local interface name and
//	                                 means nothing outside this host
//	"[2001:db8::1]:9" -> "2001:db8::1"
func normalizeIP(raw string) (string, bool) {
	raw = trimHorizontal(raw)
	if hasControl(raw) {
		return "", false
	}
	if raw == "" {
		return "", false
	}

	// Try host:port first — an unbracketed IPv6 address contains colons and
	// would otherwise be mistaken for one, so a plain address is tried second
	// rather than first.
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	} else {
		// "[::1]" with no port: SplitHostPort rejects it, but the brackets
		// still have to come off before ParseAddr will look at it.
		raw = strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]")
	}

	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return "", false
	}

	// A zone identifies an interface on the machine that wrote it. It is
	// meaningless here and would split one client into several log identities.
	addr = addr.WithZone("")

	// ::ffff:1.2.3.4 and 1.2.3.4 are the same host reached two ways.
	addr = addr.Unmap()

	// The unspecified address is what a broken proxy writes when it has
	// nothing. It is not a client.
	if addr.IsUnspecified() {
		return "", false
	}

	return addr.String(), true
}

// normalizePort validates a port and returns it in canonical decimal form.
//
// Port 0 is rejected: it is what a kernel is asked for when any port will do,
// never what a live connection reports, so its presence means the value was
// invented somewhere upstream.
func normalizePort(raw string) (string, bool) {
	raw = trimHorizontal(raw)
	if hasControl(raw) {
		return "", false
	}
	if raw == "" {
		return "", false
	}
	n, err := strconv.ParseUint(raw, 10, 16)
	if err != nil || n == 0 {
		return "", false
	}
	return strconv.FormatUint(n, 10), true
}

// splitAddr pulls an address and port out of a TCP peer string.
func splitAddr(raw string) (ip, port string, ok bool) {
	raw = trimHorizontal(raw)
	if hasControl(raw) {
		return "", "", false
	}
	if raw == "" {
		return "", "", false
	}

	if host, p, err := net.SplitHostPort(raw); err == nil {
		normIP, ipOK := normalizeIP(host)
		if !ipOK {
			return "", "", false
		}
		if normPort, portOK := normalizePort(p); portOK {
			return normIP, normPort, true
		}
		return normIP, PortUnknown, true
	}

	if normIP, ipOK := normalizeIP(raw); ipOK {
		return normIP, PortUnknown, true
	}
	return "", "", false
}

// rightmostForwarded takes the LAST entry of an X-Forwarded-For chain.
//
// The last entry is the one the nearest proxy appended, and every entry to its
// left was supplied by whoever came before — which, at the far end, is the
// client. Reading the leftmost entry is the common mistake and the reason
// "spoof your IP by setting X-Forwarded-For" works so often: it hands an
// attacker the field directly.
//
// The rightmost entry is only as good as the proxy that appended it, which is
// why the caller has to opt in with TrustForwarded and why the result is still
// labelled untrusted.
func rightmostForwarded(raw string) (string, bool) {
	if trimHorizontal(raw) == "" {
		return "", false
	}
	parts := strings.Split(raw, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		if ip, ok := normalizeIP(parts[i]); ok {
			return ip, true
		}
	}
	return "", false
}

// ------------------------------------------------------------------ logging

// controlReplacer strips what would let a header value forge log structure.
//
// The address and port that reach a log line have been through netip and
// strconv, so they cannot contain any of this. Everything ELSE on the line —
// the method, the path, a Host — is raw client input, and a path containing a
// newline can write a whole fake entry into an audit log. Sanitising is applied
// where those values are formatted, not here, but the helper lives beside the
// values it protects.
var controlReplacer = strings.NewReplacer(
	"\n", "\\n",
	"\r", "\\r",
	"\t", "\\t",
	"\x1b", "\\x1b", // ANSI escapes: a log viewer will happily render them
)

// SanitizeLogValue makes an untrusted string safe to place in a log line.
//
// Truncation is part of the job: a 32 KB header is within what the platform
// edge accepts, and one request must not be able to push everything else out
// of a bounded log buffer.
func SanitizeLogValue(s string, max int) string {
	if max > 0 && len(s) > max {
		s = s[:max] + "…"
	}
	return controlReplacer.Replace(s)
}
