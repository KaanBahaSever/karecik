package middleware

import (
	"context"
	"strings"

	"karecik/backend/internal/models"
	"karecik/backend/internal/repository"
)

// ResolveBusiness maps a slug onto the tenant it names.
//
// The slug is a BUSINESS slug: the subdomain of
// {business-slug}.karecik.com/{menu-slug}, whether it arrived in the Host
// header or in the path form used locally and by the QR fallback. Which menu is
// served under it is a separate, business-scoped lookup — see
// repository.GetMenuBySlug.
//
// It returns repository.ErrNotFound when the slug matches no business, so the
// handler can map it to a Turkish 404. A business that exists but publishes
// nothing is NOT a 404: it resolves here and is answered with an empty payload.
func ResolveBusiness(ctx context.Context, db repository.DB, slug string) (*models.Business, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return nil, repository.ErrNotFound
	}
	return repository.GetBusinessBySlug(ctx, db, slug)
}
