// Package tests holds the black-box suites that drive the Karecik API the way
// a real client does: over HTTP, through the router that main.go builds,
// against a real PostgreSQL database.
//
// Nothing in here calls a repository function directly. That is the whole
// point: an isolation hole that lives in a handler — a missing ownership check
// before a write, a menu id taken from the body and trusted — has to be able to
// turn this suite red. A test that talked to the repository would prove only
// that the SQL carries a business_id predicate, which is the half already
// covered by the unit tests.
package tests

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"karecik/backend/internal/config"
	"karecik/backend/internal/database"
	"karecik/backend/internal/handlers"
	"karecik/backend/internal/mailer"
	"karecik/backend/internal/router"
	"karecik/backend/internal/session"
	"karecik/backend/internal/utils"
)

const (
	// adminURLEnv names the connection the suite borrows in order to CREATE its
	// own scratch database. It is never the database under test, and the suite
	// never writes a single row through it.
	adminURLEnv = "KARECIK_TEST_DATABASE_URL"

	// The development machine's PostgreSQL uses trust authentication, so this
	// fallback needs no password and no configuration at all. It points at the
	// stock "postgres" maintenance database — again, only to create and drop the
	// scratch one.
	defaultAdminURL = "postgres://postgres@localhost:5432/postgres?sslmode=disable"

	// app.Test takes its timeout in milliseconds. The first request through a
	// fresh pool pays for the connection handshake, so this is deliberately
	// generous: a slow machine must not flake.
	requestTimeoutMS = 10000

	// The price productA is created with. Case 8 doubles it if the bulk-price
	// endpoint ever stops checking who owns the menu.
	fixturePrice = 10.0
)

// ---------------------------------------------------------------- harness

// harness owns the scratch database and the Fiber app under test.
type harness struct {
	t      *testing.T
	app    *fiber.App
	dbName string
	// pool is the scratch database, exposed so a suite can assert on rows the
	// API is not supposed to expose over HTTP — password reset tokens, for one.
	pool *pgxpool.Pool
}

// newHarness brings up an isolated copy of the whole backend.
//
// Gating rule, and the reason this file is not simply skipped when it is
// inconvenient: `go test ./...` must stay green on a machine with no
// PostgreSQL. A server that cannot be reached is therefore a t.Skip naming the
// environment variable, never a failure. Everything after a successful
// connection — a scratch database that cannot be created, a migration that will
// not apply — is a real problem on a machine that does have PostgreSQL, but it
// is still reported as a skip rather than a failure so that a restricted role
// (one that may connect but not CREATE DATABASE) does not paint the whole
// repository red. The skip message says exactly what went wrong.
func newHarness(t *testing.T) *harness {
	t.Helper()
	// mailer.Disabled refuses rather than pretending to deliver, so a suite that
	// wanders into the reset flow fails loudly instead of quietly passing.
	return newHarnessWith(t, mailer.Disabled{})
}

// newHarnessWith is newHarness with the e-mail transport chosen by the caller.
// The reset suite passes a recorder; everything else wants Disabled.
func newHarnessWith(t *testing.T, mail mailer.Mailer) *harness {
	t.Helper()

	adminURL := strings.TrimSpace(os.Getenv(adminURLEnv))
	source := adminURLEnv
	if adminURL == "" {
		adminURL = defaultAdminURL
		source = "the default local connection"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	adminCfg, err := poolConfig(adminURL, "")
	if err != nil {
		t.Skipf("tenant isolation suite skipped: %s is not a usable connection string "+
			"(set %s to a reachable PostgreSQL server to run this suite): %v",
			source, adminURLEnv, err)
	}
	// CREATE DATABASE and DROP DATABASE are utility statements; the simple
	// protocol keeps them clear of any prepared-statement handling. Only the
	// admin pool gets it — the pool the app runs on must use the default mode,
	// so that the suite exercises the same query path production does.
	adminCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	adminPool, err := openPool(ctx, adminCfg)
	if err != nil {
		t.Skipf("tenant isolation suite skipped: could not connect to PostgreSQL using %s "+
			"(set %s to a reachable server to run this suite): %v", source, adminURLEnv, err)
	}
	// Registered FIRST so that it runs LAST: t.Cleanup is LIFO, and the DROP
	// below needs this pool to still be open. Closing it with `defer` instead —
	// the obvious-looking move — would run before every cleanup and leave a
	// karecik_test_* database behind on every run.
	t.Cleanup(adminPool.Close)

	dbName := scratchDatabaseName(t)

	// dbName is "karecik_test_" plus hex from crypto/rand, so quoting the
	// identifier here cannot be an injection.
	if _, err := adminPool.Exec(ctx, `CREATE DATABASE "`+dbName+`"`); err != nil {
		t.Skipf("tenant isolation suite skipped: could not create the scratch database %q "+
			"(the role behind %s has to be allowed to CREATE DATABASE): %v",
			dbName, adminURLEnv, err)
	}
	// Registered second, so it runs second: after the scratch pool is closed and
	// before the admin pool is.
	t.Cleanup(func() { dropScratchDatabase(t, adminPool, dbName) })

	scratchCfg, err := poolConfig(adminURL, dbName)
	if err != nil {
		t.Fatalf("could not build the scratch pool configuration: %v", err)
	}
	scratchPool, err := openPool(ctx, scratchCfg)
	if err != nil {
		t.Fatalf("could not connect to the scratch database %q: %v", dbName, err)
	}
	// Registered last, so it runs first — nothing may hold a connection to the
	// database the next cleanup drops.
	t.Cleanup(scratchPool.Close)

	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer migrateCancel()
	if err := database.Migrate(migrateCtx, scratchPool); err != nil {
		t.Fatalf("could not migrate the scratch database %q: %v", dbName, err)
	}

	// The configuration is built by hand rather than through config.Load():
	// that helper walks up to backend/.env, which points at the LIVE karecik
	// database. That does not belong in a test.
	//
	// The cookie is deliberately NOT Secure here: app.Test speaks plain HTTP, so
	// a Secure cookie would be set by the server and then dropped by the client,
	// and every authenticated assertion would fail for a reason that has nothing
	// to do with tenant isolation.
	cfg := &config.Config{
		DatabaseURL:    "",
		CookieDomain:   "",
		CookieSameSite: "Lax",
		CookieSecure:   false,
		Host:           "127.0.0.1",
		Port:           "0",
		AppDomain:      "karecik.com",
		DevDomain:      "localhost",
		CORSOrigins:    []string{"http://localhost:5173"},
		UploadDir:      t.TempDir(),
		MaxUploadBytes: 5 * 1024 * 1024,
		ServeStatic:    false,
		StaticDir:      "",
		Env:            "development",
	}

	// Mirrors cmd/api/main.go, error handler included: the status a handler
	// returns is the thing under test, so the app that returns it has to be
	// assembled the same way.
	app := fiber.New(fiber.Config{
		AppName:               "Karecik API (test)",
		BodyLimit:             int(cfg.MaxUploadBytes) + 1024*1024,
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				return utils.Fail(c, fiberErr.Code, "HTTP_ERROR", fiberErr.Message)
			}
			return utils.Internal(c, err)
		},
	})
	// Sessions are in memory now, so each harness gets its own store the same
	// way it gets its own scratch database — two suites running in parallel
	// must not be able to see each other's logins.
	//
	// No janitor: nothing in this suite outlives its expiry, and a background
	// goroutine sweeping a store the test is asserting against would only add
	// timing to a suite that has none.
	sessions := session.New()
	t.Cleanup(sessions.Stop)

	router.Setup(app, handlers.New(scratchPool, cfg, sessions, mail), cfg)

	return &harness{t: t, app: app, dbName: dbName, pool: scratchPool}
}

// poolConfig parses a connection string and optionally re-points it at another
// database on the same server. Swapping the field rather than rewriting the URL
// keeps every other parameter — host, port, user, sslmode — exactly as the
// caller wrote them, and works for the keyword/value DSN form too.
func poolConfig(dsn, database string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if database != "" {
		cfg.ConnConfig.Database = database
	}
	cfg.MaxConns = 5
	cfg.MinConns = 0
	return cfg, nil
}

func openPool(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// scratchDatabaseName builds a name this run owns exclusively. The suite drops
// what this returns and nothing else — never a database it did not create, and
// never the live "karecik".
func scratchDatabaseName(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("could not generate a scratch database name: %v", err)
	}
	return "karecik_test_" + hex.EncodeToString(buf)
}

// dropScratchDatabase removes the scratch database, and says so loudly when it
// cannot: a silently swallowed error here leaves karecik_test_* databases piling
// up on the developer's server.
func dropScratchDatabase(t *testing.T, admin *pgxpool.Pool, name string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// The scratch pool was closed by the previous cleanup, but a backend that
	// has not finished shutting down still blocks the drop. FORCE handles that
	// in one statement on PostgreSQL 13 and newer.
	if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS "`+name+`" WITH (FORCE)`); err == nil {
		return
	}

	// Older server: terminate the leftover backends by hand and try again.
	if _, err := admin.Exec(ctx,
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity
		  WHERE datname = $1 AND pid <> pg_backend_pid()`, name); err != nil {
		t.Logf("could not terminate the backends of %q: %v", name, err)
	}
	if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS "`+name+`"`); err != nil {
		t.Errorf("could not drop the scratch database %q: %v — drop it by hand", name, err)
	}
}

// ------------------------------------------------------------ HTTP helpers

// do performs one request against the app under test and returns the response
// together with its already-drained body, because every assertion here wants
// both the status and the payload.
func (h *harness) do(method, path, session string, body any) (*http.Response, []byte) {
	h.t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("could not encode the request body for %s %s: %v", method, path, err)
		}
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// The session is a cookie now, not a bearer header. Sending it the way a
	// browser would is the point: it exercises the same middleware path a real
	// request takes, including the cookie name.
	if session != "" {
		req.AddCookie(&http.Cookie{Name: utils.SessionCookieName, Value: session})
	}

	resp, err := h.app.Test(req, requestTimeoutMS)
	if err != nil {
		h.t.Fatalf("%s %s: the request never completed: %v", method, path, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatalf("%s %s: could not read the response body: %v", method, path, err)
	}
	return resp, payload
}

func decodeInto(t *testing.T, what string, payload []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatalf("%s: could not decode the response %s: %v", what, payload, err)
	}
}

// ------------------------------------------------------------- payloads

type authPayload struct {
	Business struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	} `json:"business"`
}

type menuPayload struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type categoryPayload struct {
	ID     string  `json:"id"`
	MenuID *string `json:"menu_id"`
}

type productPayload struct {
	ID         string  `json:"id"`
	CategoryID string  `json:"category_id"`
	Price      float64 `json:"price"`
}

// tenant is one registered account and the session cookie that speaks for it.
type tenant struct {
	label      string
	session    string
	businessID string
}

// ------------------------------------------------------------- fixtures

// requireSuccess is what separates a fixture failure from a false pass. If the
// product was never created, B cannot steal it and every assertion below would
// come back green while proving nothing — so a fixture that did not return 2xx
// stops the test right here, named as a fixture.
func (h *harness) requireSuccess(what string, resp *http.Response, payload []byte) {
	h.t.Helper()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		h.t.Fatalf("fixture %s: expected a 2xx, got %d: %s", what, resp.StatusCode, payload)
	}
}

// register creates a tenant through the real sign-up endpoint. Sign-up creates
// the user and the business and nothing else — no menu — which is why the menu,
// category and product below are created over HTTP as well.
func (h *harness) register(label, businessName, email string) *tenant {
	h.t.Helper()

	resp, payload := h.do(http.MethodPost, "/api/auth/register", "", map[string]any{
		"business_name": businessName,
		"email":         email,
		"password":      "karecik-test-password",
	})
	h.requireSuccess("register "+label, resp, payload)

	var auth authPayload
	decodeInto(h.t, "fixture register "+label, payload, &auth)
	if auth.Business.ID == "" {
		h.t.Fatalf("fixture register %s: the response carried no business id: %s", label, payload)
	}

	// Sign-up must hand back a session cookie; the body no longer carries a
	// token, so a missing cookie means nothing downstream could authenticate.
	session := sessionCookie(resp)
	if session == "" {
		h.t.Fatalf("fixture register %s: no %s cookie was set", label, utils.SessionCookieName)
	}
	return &tenant{label: label, session: session, businessID: auth.Business.ID}
}

func (h *harness) createMenu(owner *tenant, name string) menuPayload {
	h.t.Helper()

	resp, payload := h.do(http.MethodPost, "/api/menus", owner.session, map[string]any{
		"name": name,
	})
	h.requireSuccess(fmt.Sprintf("create menu %q for %s", name, owner.label), resp, payload)

	var menu menuPayload
	decodeInto(h.t, "fixture create menu", payload, &menu)
	if menu.ID == "" {
		h.t.Fatalf("fixture create menu %q: no id in the response: %s", name, payload)
	}
	return menu
}

func (h *harness) createCategory(owner *tenant, menuID, name string) categoryPayload {
	h.t.Helper()

	resp, payload := h.do(http.MethodPost, "/api/categories", owner.session, map[string]any{
		"menu_id":      menuID,
		"translations": map[string]any{"tr": map[string]any{"name": name}},
	})
	h.requireSuccess(fmt.Sprintf("create category %q for %s", name, owner.label), resp, payload)

	var category categoryPayload
	decodeInto(h.t, "fixture create category", payload, &category)
	if category.ID == "" {
		h.t.Fatalf("fixture create category %q: no id in the response: %s", name, payload)
	}
	return category
}

func (h *harness) createProduct(owner *tenant, categoryID, name string, price float64) productPayload {
	h.t.Helper()

	resp, payload := h.do(http.MethodPost, "/api/products", owner.session, map[string]any{
		"category_id":  categoryID,
		"translations": map[string]any{"tr": map[string]any{"name": name}},
		"price":        price,
	})
	h.requireSuccess(fmt.Sprintf("create product %q for %s", name, owner.label), resp, payload)

	var product productPayload
	decodeInto(h.t, "fixture create product", payload, &product)
	if product.ID == "" {
		h.t.Fatalf("fixture create product %q: no id in the response: %s", name, payload)
	}
	return product
}

// --------------------------------------------------------- owner re-reads
// The negative half of every mutating case. A handler that answers 404 and
// performs the write anyway sails through a status-only check, so each case
// comes back as the owner and looks at the row itself.

func (h *harness) readMenuAsOwner(t *testing.T, what string, owner *tenant, menuID string) menuPayload {
	t.Helper()
	resp, payload := h.do(http.MethodGet, "/api/menus/"+menuID, owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: %s can no longer read its own menu %s (got %d: %s)",
			what, owner.label, menuID, resp.StatusCode, payload)
	}
	var menu menuPayload
	decodeInto(t, what, payload, &menu)
	return menu
}

func (h *harness) listCategoriesAsOwner(t *testing.T, what string, owner *tenant, menuID string) []categoryPayload {
	t.Helper()
	resp, payload := h.do(http.MethodGet, "/api/categories?menu_id="+menuID, owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: listing %s's own categories failed (got %d: %s)",
			what, owner.label, resp.StatusCode, payload)
	}
	var categories []categoryPayload
	decodeInto(t, what, payload, &categories)
	return categories
}

func (h *harness) listProductsAsOwner(t *testing.T, what string, owner *tenant, categoryID string) []productPayload {
	t.Helper()
	resp, payload := h.do(http.MethodGet, "/api/products?category_id="+categoryID, owner.session, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: listing %s's own products failed (got %d: %s)",
			what, owner.label, resp.StatusCode, payload)
	}
	var products []productPayload
	decodeInto(t, what, payload, &products)
	return products
}

// findProduct returns the product with the given id, or nil when the list does
// not contain it.
func findProduct(products []productPayload, id string) *productPayload {
	for i := range products {
		if products[i].ID == id {
			return &products[i]
		}
	}
	return nil
}

func findCategory(categories []categoryPayload, id string) *categoryPayload {
	for i := range categories {
		if categories[i].ID == id {
			return &categories[i]
		}
	}
	return nil
}

// -------------------------------------------------------- the assertion

// assertDenied is the positive half of every case: the request has to come back
// as a client error. A 2xx means the isolation hole is open; a 5xx means the
// handler crashed instead of refusing, which is not a defence either.
//
// Both 403 and 404 are permitted where the brief permits both — the codebase
// uses 403 when the request named someone else's PARENT entity and 404 when the
// target itself is not yours, and pinning one here would make the suite brittle
// against a deliberate choice. The status that actually came back is always
// recorded, so a regression is diagnosable from the log alone.
func assertDenied(t *testing.T, what string, resp *http.Response, payload []byte, allowed ...int) {
	t.Helper()

	got := resp.StatusCode
	if got >= 200 && got < 300 {
		t.Errorf("%s: THE REQUEST SUCCEEDED (%d) — cross-tenant access is open: %s",
			what, got, payload)
		return
	}
	if got < 400 || got >= 500 {
		t.Errorf("%s: expected a 4xx refusal, got %d: %s", what, got, payload)
		return
	}
	for _, code := range allowed {
		if got == code {
			t.Logf("%s: refused with %d", what, got)
			return
		}
	}
	t.Errorf("%s: refused, but with %d — the documented answers are %v: %s",
		what, got, allowed, payload)
}

// --------------------------------------------------------------- the suite

// TestTenantIsolation is the guarantee itself: an authenticated user of
// business B must never read, mutate, delete or attach anything that belongs to
// business A.
func TestTenantIsolation(t *testing.T) {
	h := newHarness(t)

	// --- fixtures. Every one of these is asserted 2xx before a single
	// isolation assertion runs.
	tenantA := h.register("A", "Tenant A", "a@example.test")
	tenantB := h.register("B", "Tenant B", "b@example.test")

	menuA := h.createMenu(tenantA, "Tenant A Menüsü")
	categoryA := h.createCategory(tenantA, menuA.ID, "A Kategorisi")
	productA := h.createProduct(tenantA, categoryA.ID, "A Ürünü", fixturePrice)

	// B owns a menu of its own, so that case 5 can try to re-parent A's
	// category into it — the theft only means something when the destination is
	// legitimately B's.
	menuB := h.createMenu(tenantB, "Tenant B Menüsü")

	if menuA.ID == menuB.ID || tenantA.businessID == tenantB.businessID {
		t.Fatalf("fixtures collided: the two tenants are not distinct (menus %s / %s, businesses %s / %s)",
			menuA.ID, menuB.ID, tenantA.businessID, tenantB.businessID)
	}

	t.Run("1_read_menu", func(t *testing.T) {
		resp, payload := h.do(http.MethodGet, "/api/menus/"+menuA.ID, tenantB.session, nil)
		assertDenied(t, "B reading A's menu", resp, payload,
			http.StatusForbidden, http.StatusNotFound)

		if strings.Contains(string(payload), menuA.Name) {
			t.Errorf("B reading A's menu: the refusal leaked A's menu name: %s", payload)
		}
	})

	t.Run("2_update_menu", func(t *testing.T) {
		resp, payload := h.do(http.MethodPut, "/api/menus/"+menuA.ID, tenantB.session,
			map[string]any{"name": "Hacked"})
		assertDenied(t, "B renaming A's menu", resp, payload,
			http.StatusForbidden, http.StatusNotFound)

		// The point of the case: refused AND not written.
		menu := h.readMenuAsOwner(t, "B renaming A's menu", tenantA, menuA.ID)
		if menu.Name != "Tenant A Menüsü" {
			t.Errorf("B renaming A's menu: the write went through despite the refusal — "+
				"A's menu is now named %q", menu.Name)
		}
	})

	t.Run("3_delete_product", func(t *testing.T) {
		resp, payload := h.do(http.MethodDelete, "/api/products/"+productA.ID, tenantB.session, nil)
		assertDenied(t, "B deleting A's product", resp, payload,
			http.StatusForbidden, http.StatusNotFound)

		products := h.listProductsAsOwner(t, "B deleting A's product", tenantA, categoryA.ID)
		if findProduct(products, productA.ID) == nil {
			t.Errorf("B deleting A's product: the delete went through despite the refusal — "+
				"product %s is gone from A's own list (%d products left)",
				productA.ID, len(products))
		}
	})

	t.Run("4_create_product_in_foreign_category", func(t *testing.T) {
		before := h.listProductsAsOwner(t, "B adding a product to A's category",
			tenantA, categoryA.ID)

		resp, payload := h.do(http.MethodPost, "/api/products", tenantB.session, map[string]any{
			"category_id":  categoryA.ID,
			"translations": map[string]any{"tr": map[string]any{"name": "B'nin ürünü"}},
			"price":        99,
		})
		assertDenied(t, "B adding a product to A's category", resp, payload,
			http.StatusForbidden, http.StatusUnprocessableEntity)

		after := h.listProductsAsOwner(t, "B adding a product to A's category",
			tenantA, categoryA.ID)
		if len(after) != len(before) {
			t.Errorf("B adding a product to A's category: the insert went through despite the "+
				"refusal — A's category held %d products and now holds %d",
				len(before), len(after))
		}
	})

	t.Run("5_steal_category_by_reparenting", func(t *testing.T) {
		resp, payload := h.do(http.MethodPut, "/api/categories/"+categoryA.ID, tenantB.session,
			map[string]any{"menu_id": menuB.ID})
		assertDenied(t, "B re-parenting A's category into its own menu", resp, payload,
			http.StatusForbidden, http.StatusNotFound)

		categories := h.listCategoriesAsOwner(t, "B re-parenting A's category", tenantA, menuA.ID)
		stolen := findCategory(categories, categoryA.ID)
		if stolen == nil {
			t.Fatalf("B re-parenting A's category: the move went through despite the refusal — "+
				"category %s has left A's menu entirely", categoryA.ID)
		}
		if stolen.MenuID == nil || *stolen.MenuID != menuA.ID {
			t.Errorf("B re-parenting A's category: the move went through despite the refusal — "+
				"category %s now points at menu %v instead of A's %s",
				categoryA.ID, stolen.MenuID, menuA.ID)
		}
	})

	t.Run("6_list_categories_of_foreign_menu", func(t *testing.T) {
		resp, payload := h.do(http.MethodGet, "/api/categories?menu_id="+menuA.ID,
			tenantB.session, nil)
		assertDenied(t, "B listing the categories of A's menu", resp, payload,
			http.StatusForbidden, http.StatusNotFound)

		// A refusal that still ships the rows would be the same leak with a
		// different status code on it.
		if strings.Contains(string(payload), categoryA.ID) {
			t.Errorf("B listing the categories of A's menu: the response carried A's "+
				"category %s: %s", categoryA.ID, payload)
		}
	})

	t.Run("7_delete_menu", func(t *testing.T) {
		resp, payload := h.do(http.MethodDelete, "/api/menus/"+menuA.ID, tenantB.session, nil)
		assertDenied(t, "B deleting A's menu", resp, payload,
			http.StatusForbidden, http.StatusNotFound)

		// DeleteMenu cascades into the categories and the products, so a hole
		// here would take the rest of A's data with it.
		menu := h.readMenuAsOwner(t, "B deleting A's menu", tenantA, menuA.ID)
		if menu.ID != menuA.ID {
			t.Errorf("B deleting A's menu: re-reading it as A returned a different menu (%s)",
				menu.ID)
		}
	})

	t.Run("8_bulk_price_on_foreign_menu", func(t *testing.T) {
		resp, payload := h.do(http.MethodPost, "/api/products/bulk-price", tenantB.session,
			map[string]any{
				"menu_id":    menuA.ID,
				"apply":      true,
				"percentage": 100,
			})
		assertDenied(t, "B doubling the prices of A's menu", resp, payload,
			http.StatusForbidden, http.StatusNotFound)

		// price_updated_at is NOT NULL DEFAULT now(), so it can never be used as
		// a "nothing happened" probe. The price itself can: +100% would turn
		// fixturePrice into twice fixturePrice.
		products := h.listProductsAsOwner(t, "B doubling the prices of A's menu",
			tenantA, categoryA.ID)
		product := findProduct(products, productA.ID)
		if product == nil {
			t.Fatalf("B doubling the prices of A's menu: A's product %s disappeared", productA.ID)
		}
		if product.Price != fixturePrice {
			t.Errorf("B doubling the prices of A's menu: the update went through despite the "+
				"refusal — A's product price is %.2f, expected %.2f",
				product.Price, fixturePrice)
		}
	})
}

// sessionCookie pulls the session value out of a response's Set-Cookie headers.
// It returns "" when the header is absent, which the caller treats as a fixture
// failure rather than an isolation failure.
func sessionCookie(resp *http.Response) string {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == utils.SessionCookieName && cookie.Value != "" {
			return cookie.Value
		}
	}
	return ""
}
