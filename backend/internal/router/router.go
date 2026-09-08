package router

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
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
	if cfg.ServeStatic {
		app.Static("/", cfg.StaticDir, fiber.Static{Browse: false, MaxAge: 3600})
		app.Get("/*", func(c *fiber.Ctx) error {
			return c.SendFile(filepath.Join(cfg.StaticDir, "index.html"))
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

	roots := []string{cfg.AppDomain}
	if !cfg.IsProduction() {
		roots = append(roots, cfg.DevDomain)
	}

	for _, root := range roots {
		root = strings.ToLower(strings.TrimSpace(root))
		if root == "" {
			continue
		}
		if host == root || strings.HasSuffix(host, "."+root) {
			return true
		}
	}
	return false
}
