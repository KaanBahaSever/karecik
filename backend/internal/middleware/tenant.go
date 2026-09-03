package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
)

// Tenant is the business (and optionally branch + menu) a public request
// targets. ExtractSubdomain / SubdomainOf only produce the slug; the lookup
// that turns it into records lives here.
type Tenant struct {
	Business *models.Business
	Branch   *models.Branch // nil when the slug matched a business directly
	Menu     *models.Menu   // the menu the request is served from
}

// ResolveTenant maps a slug (from a subdomain or a path) plus an optional menu
// slug onto a Tenant.
//
// Order: branch slug first, then business slug. Branch slugs are globally
// unique and the 003 migration gave every existing business a branch carrying
// its own slug, so old <business>.karecik.com addresses keep resolving to the
// very same menu.
//
// It returns repository.ErrNotFound when no branch and no business match, so
// the handler can map it to a Turkish 404.
func ResolveTenant(ctx context.Context, db repository.DB, slug, menuSlug string) (*Tenant, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return nil, repository.ErrNotFound
	}

	tenant := &Tenant{}

	branch, err := repository.GetBranchBySlug(ctx, db, slug)
	switch {
	case err == nil:
		// GetBranchBySlug already refuses an unpublished business, but
		// GetBusinessByID does not filter, so the guard is repeated here.
		business, err := repository.GetBusinessByID(ctx, db, branch.BusinessID)
		if err != nil {
			return nil, err
		}
		if !business.IsActive {
			return nil, repository.ErrNotFound
		}
		tenant.Branch = branch
		tenant.Business = business

	case errors.Is(err, repository.ErrNotFound):
		// No branch owns this slug — fall back to the business itself.
		// GetBusinessBySlug keeps an inactive business invisible.
		business, err := repository.GetBusinessBySlug(ctx, db, slug)
		if err != nil {
			return nil, err
		}
		tenant.Business = business

	default:
		return nil, err
	}

	menu, err := resolveMenu(ctx, db, tenant, menuSlug)
	if err != nil {
		return nil, err
	}
	tenant.Menu = menu

	return tenant, nil
}

// resolveMenu picks the menu a resolved tenant is served from: the requested
// slug when it is reachable, otherwise the default menu of the branch, and
// finally the default menu of the business.
//
// Every step only ever returns a published menu: is_active = false means the
// menu is offline, exactly like it does for a branch or a business.
//
// A business that has no menu at all still resolves, with a nil menu — the
// customer menu then falls back to every category of the business, which is
// exactly what it showed before menus existed.
func resolveMenu(ctx context.Context, db repository.DB, tenant *Tenant, menuSlug string) (*models.Menu, error) {
	if slug := strings.TrimSpace(menuSlug); slug != "" {
		menu, err := repository.GetMenuBySlug(ctx, db, tenant.Business.ID, slug)
		switch {
		case err == nil:
			reachable, err := menuReachable(ctx, db, tenant.Branch, menu)
			if err != nil {
				return nil, err
			}
			if reachable && menu.IsActive {
				return menu, nil
			}
		case errors.Is(err, repository.ErrNotFound):
			// An unknown menu slug falls back to the default menu instead of
			// hiding the whole business behind a 404.
		default:
			return nil, err
		}
	}

	if tenant.Branch != nil {
		menu, err := repository.DefaultMenuOfBranch(ctx, db, tenant.Branch.ID)
		if err == nil {
			return menu, nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
	}

	return defaultPublishedMenu(ctx, db, tenant.Business.ID)
}

// defaultPublishedMenu returns the menu a business serves when nothing more
// specific was asked for: its default menu when that one is published, else the
// first published menu by position.
//
// repository.GetDefaultMenu is deliberately not used here — it ignores
// is_active, which is what the dashboard preview wants and what a public
// request must never get.
func defaultPublishedMenu(ctx context.Context, db repository.DB,
	businessID uuid.UUID) (*models.Menu, error) {

	menus, err := repository.ListMenus(ctx, db, businessID)
	if err != nil {
		return nil, err
	}
	if len(menus) == 0 {
		return nil, nil
	}

	var first *models.Menu
	for i := range menus {
		if !menus[i].IsActive {
			continue
		}
		if menus[i].IsDefault {
			return &menus[i], nil
		}
		if first == nil {
			first = &menus[i]
		}
	}
	if first == nil {
		// Every menu of the business is offline, so the address stays dark
		// just like an inactive branch or business does.
		return nil, repository.ErrNotFound
	}
	return first, nil
}

// menuReachable reports whether a menu may be served through the resolved
// branch. Without a branch every menu of the business is reachable; with one,
// only the menus linked to it are — otherwise a branch address could serve a
// menu that branch does not carry.
func menuReachable(ctx context.Context, db repository.DB, branch *models.Branch, menu *models.Menu) (bool, error) {
	if branch == nil {
		return true, nil
	}

	menus, err := repository.ListBranchMenus(ctx, db, branch.ID)
	if err != nil {
		return false, err
	}
	for _, linked := range menus {
		if linked.ID == menu.ID {
			return true, nil
		}
	}
	return false, nil
}
