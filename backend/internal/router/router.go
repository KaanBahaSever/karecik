package router

import (
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"karecik/backend/internal/config"
	"karecik/backend/internal/handlers"
	"karecik/backend/internal/middleware"
	"karecik/backend/internal/utils"
)

// Setup registers every middleware and route.
//
// The registration order matters:
//  1. public /api endpoints
//  2. the protected /api group -> every /api path added after this line
//     requires a token
//  3. static files and the SPA fallback (last)
func Setup(app *fiber.App, h *handlers.Handler, cfg *config.Config) {
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "[karecik] ${time} ${status} ${method} ${path} (${latency})\n",
		TimeFormat: "15:04:05",
	}))

	// Only AllowOriginsFunc is passed: isAllowedOrigin checks cfg.CORSOrigins
	// itself, so defining AllowOrigins as well would just make Fiber log
	// "Both 'AllowOrigins' and 'AllowOriginsFunc' have been defined" on every
	// start-up. Fiber falls back to "*" only when neither of the two is set.
	// AllowCredentials is what lets the session cookie cross an origin at all:
	// without it the browser refuses to SEND the cookie on a cross-origin fetch
	// and refuses to STORE what comes back, however correct the Set-Cookie is.
	// It also rules out the "*" wildcard — the spec forbids the pair — which is
	// exactly why the origin is decided by a function that echoes one specific
	// allowed origin back.
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool { return isAllowedOrigin(origin, cfg) },
		AllowCredentials: true,
		AllowHeaders:     "Origin, Content-Type, Accept, X-Requested-With",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		MaxAge:           3600,
	}))

	// Uploaded images
	app.Static("/uploads", cfg.UploadDir, fiber.Static{
		Browse:    false,
		MaxAge:    86400,
		ByteRange: true,
	})

	// ------------------------------------------------------ public endpoints
	app.Get("/api/health", h.Health)
	app.Get("/api/meta", h.Meta)

	app.Post("/api/auth/register", h.Register)
	app.Post("/api/auth/login", h.Login)

	// Password reset. These two are the only unauthenticated endpoints that DO
	// something on behalf of a named account, so both are limited per IP — but
	// NOT with the same budget, because they cost entirely different things.
	//
	// They deliberately get separate limiter instances rather than one shared
	// middleware. Sharing it would spend the mail budget on form submissions:
	// somebody who asks for a link and then mistypes their new password twice
	// would be locked out of finishing the reset they are in the middle of.
	tooManyRequests := func(c *fiber.Ctx) error {
		return utils.Fail(c, fiber.StatusTooManyRequests, "RATE_LIMITED",
			"Çok fazla deneme yaptınız. Lütfen bir süre sonra tekrar deneyin.")
	}
	byIP := func(c *fiber.Ctx) string { return c.IP() }

	// /forgot-password SENDS MAIL, and the provider's free tier allows 100
	// messages a day. Two an hour per host is the tightest budget a real
	// person still fits inside: ask once, notice nothing arrived, ask again.
	// A third attempt in the same hour is not someone recovering an account.
	//
	// This is only the per-host half. Requests spread across many hosts walk
	// straight past it, which is what handlers.resetCooldown is for.
	forgotLimiter := limiter.New(limiter.Config{
		Max:          2,
		Expiration:   time.Hour,
		KeyGenerator: byIP,
		LimitReached: tooManyRequests,
	})

	// /reset-password sends nothing: it accepts a token guess and a new
	// password. The token is 256 bits, so the limiter is defence in depth
	// rather than the real barrier — the budget only has to be small enough to
	// make automated probing pointless and large enough to survive a person
	// fixing "the two passwords do not match" a few times.
	submitLimiter := limiter.New(limiter.Config{
		Max:          10,
		Expiration:   15 * time.Minute,
		KeyGenerator: byIP,
		LimitReached: tooManyRequests,
	})

	app.Post("/api/auth/forgot-password", forgotLimiter, h.ForgotPassword)
	app.Post("/api/auth/reset-password", submitLimiter, h.ResetPassword)

	// Logout is public on purpose: an expired or already-revoked cookie must
	// still be able to clear itself, and requiring a valid session to log out
	// would strand exactly the people who most need to.
	app.Post("/api/auth/logout", h.Logout)

	// Customer menu — no token required. The address is
	// {business-slug}.karecik.com/{menu-slug}: the host form reads the tenant
	// from the subdomain and the menu from an optional "?menu=", the path form
	// reads both from the path. The menu segment is optional in both, because
	// the bare tenant address is a directory page, not an error.
	app.Get("/api/public/menu", h.PublicMenuByHost)
	app.Get("/api/public/menu/:businessSlug", h.PublicMenuByPath)
	app.Get("/api/public/menu/:businessSlug/:menuSlug", h.PublicMenuByPath)

	// --------------------------------------------------- protected endpoints
	api := app.Group("/api", middleware.Protected(h.Sessions, cfg))

	api.Get("/auth/me", h.Me)
	api.Post("/auth/change-password", h.ChangePassword)

	// The account owns exactly two fields — its name and the subdomain slug.
	// Every setting a customer sees lives on a menu and is written through
	// /api/menus/:id.
	api.Get("/business", h.GetBusiness)
	api.Put("/business", h.UpdateBusiness)

	// Categories — the "reorder" path must come BEFORE the ":id" pattern
	api.Get("/categories", h.ListCategories)
	api.Post("/categories", h.CreateCategory)
	api.Put("/categories/reorder", h.ReorderCategories)
	api.Put("/categories/:id", h.UpdateCategory)
	api.Delete("/categories/:id", h.DeleteCategory)

	// Products — the fixed paths must come BEFORE the ":id" pattern
	api.Get("/products", h.ListProducts)
	api.Post("/products", h.CreateProduct)
	api.Put("/products/reorder", h.ReorderProducts)
	api.Post("/products/bulk-price", h.BulkPrice)
	api.Put("/products/:id", h.UpdateProduct)
	api.Patch("/products/:id/price", h.PatchProductPrice)
	api.Delete("/products/:id", h.DeleteProduct)

	// Menus — the primary entity: a business may publish several of them
	// (kahvaltı, akşam, bar...) and each one owns its address and its settings.
	api.Get("/menus", h.ListMenus)
	api.Post("/menus", h.CreateMenu)
	api.Get("/menus/:id", h.GetMenu)
	api.Put("/menus/:id", h.UpdateMenu)
	api.Delete("/menus/:id", h.DeleteMenu)

	api.Post("/uploads", h.Upload)

	// Dashboard live preview — "?menu=<slug>" is optional
	api.Get("/preview/menu", h.PreviewMenu)

	// ------------------------------------------------- unknown /api requests
	app.All("/api/*", func(c *fiber.Ctx) error {
		return utils.NotFound(c, "Böyle bir API ucu yok: "+c.Path())
	})

	// ------------------------------ production: serve the built frontend (SPA)
	//
	// One container serves both halves: the Go binary answers /api and hands out
	// the React bundle for everything else. app.Static serves a file when one
	// exists on disk and calls Next() when it does not, so the catch-all below
	// is what turns an address that exists only inside the browser —
	// /panel/ayarlar, or a tenant's /kahvalti — into index.html.
	if cfg.ServeStatic {
		// The HTML shell, served for "/" and for every client-side route below.
		//
		// no-cache, not a max-age: this file names the hashed bundle files by
		// their exact filenames, and those names change on every deploy. Letting
		// a browser hold it for an hour means an hour of requests for assets that
		// no longer exist. no-cache still lets the browser keep a copy — it just
		// has to revalidate, which is one cheap 304 when nothing changed.
		serveIndex := func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderCacheControl, "no-cache")
			return c.SendFile(filepath.Join(cfg.StaticDir, "index.html"))
		}

		// Registered before the static handler below so it wins for exactly "/".
		app.Get("/", serveIndex)

		// The hashed bundle is the opposite case: the filename changes whenever
		// the content does, so it can be cached for a year and never revalidated.
		// This is what makes a repeat visit almost free.
		app.Static("/assets", filepath.Join(cfg.StaticDir, "assets"), fiber.Static{
			Browse: false,
			MaxAge: 31536000,
		})

		// Everything else Vite copies out of public/ — logo.svg and friends.
		// These keep their names across deploys, so an hour is fine.
		app.Static("/", cfg.StaticDir, fiber.Static{Browse: false, MaxAge: 3600})

		app.Get("/*", func(c *fiber.Ctx) error {
			// A miss under these two prefixes is a MISSING FILE, not a deep
			// link: no route in the SPA lives under /assets or /uploads. Left
			// to the catch-all they answer 200 with the HTML page, and the
			// browser is handed markup where it asked for a script or an image.
			//
			// The case that actually bites: a browser still holding the
			// previous deploy's index.html asks for /assets/index-OLDHASH.js.
			// Answering with HTML gives "Unexpected token '<'" and a white
			// screen, and because it is a cacheable 200 a reload does not
			// reliably clear it. A 404 fails cleanly and fixes itself.
			//
			// This only matters with SERVE_STATIC on — without the catch-all
			// these paths already 404 — which is why it came back with the
			// move to a single container.
			//
			// The trailing slash in the prefixes is load-bearing. Menu slugs are
			// the first path segment under a tenant subdomain and are NOT
			// reserved-checked, so a business may own a menu addressed /assets.
			// "/assets/" does not match "/assets", so that menu still opens.
			path := c.Path()
			if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/uploads/") {
				return utils.NotFound(c, "Dosya bulunamadı: "+path)
			}
			return serveIndex(c)
		})
	}
}

// isAllowedOrigin decides whether an origin may call the API.
// Besides the configured list it allows every subdomain of the root domain,
// because customer menus are served from <business>.karecik.com.
func isAllowedOrigin(origin string, cfg *config.Config) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return false
	}

	for _, allowed := range cfg.CORSOrigins {
		if strings.EqualFold(strings.TrimSpace(allowed), origin) {
			return true
		}
	}

	// The root domain itself — and, deliberately, NOT *.karecik.com any more.
	//
	// That wildcard belonged to the split deployment, where a menu served from
	// {slug}.karecik.com had to call api.karecik.com and really was cross-origin.
	// One container means one origin per host: the panel calls karecik.com/api,
	// a customer menu calls {slug}.karecik.com/api, and a same-origin request
	// never consults Access-Control-Allow-Origin at all. Nothing legitimate
	// needs the allowance.
	//
	// Keeping it would cost something real. AllowCredentials is on, so any script
	// that got a foothold on ANY tenant subdomain could call the panel API on
	// karecik.com with the owner's session cookie and READ THE ANSWER — the
	// cookie is sent because the request targets karecik.com, and SameSite=Lax
	// does not object because the two hosts are same-site. Uploaded SVGs are
	// served inline from those same tenant hosts, so scanSVG would be the only
	// thing standing between one tenant and every other tenant's dashboard. That
	// is too much weight for one regular expression to carry.
	//
	// In production the scheme is checked too, not just the hostname. Comparing
	// only the host would accept "http://karecik.com" — and a plaintext origin
	// is not the panel, because the panel is served over HTTPS.
	if appDomain := strings.ToLower(strings.TrimSpace(cfg.AppDomain)); appDomain != "" {
		if cfg.IsProduction() {
			if strings.EqualFold(origin, "https://"+appDomain) {
				return true
			}
		} else if host == appDomain {
			return true
		}
	}

	// Development keeps the wildcard: *.localhost is how subdomain routing is
	// exercised locally, and nothing there is worth stealing.
	if !cfg.IsProduction() {
		if dev := strings.ToLower(strings.TrimSpace(cfg.DevDomain)); dev != "" {
			if host == dev || strings.HasSuffix(host, "."+dev) {
				return true
			}
		}
	}

	return false
}
