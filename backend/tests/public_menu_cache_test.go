package tests

import (
	"net/http"
	"strings"
	"testing"
)

// TestPublicMenuIsRevalidatedOnEveryLoad pins down the cache contract of the
// customer menu endpoints.
//
// They used to answer "Cache-Control: public, max-age=60". An owner who added a
// category in the panel and reopened the menu in the same browser within that
// minute was handed the old response straight from the browser cache — no
// request reached the server — and the new category was missing. The dashboard
// dialog saves no icon and no image unless the owner picks one, so the category
// that went missing was usually a plain one, which made the bug read as
// "categories without an emoji or a picture do not show".
//
// The contract now: no-cache plus an ETag. The browser may keep its copy but has
// to ask first; an unchanged menu is a 304, and a changed one — the category
// added a moment ago — is a full 200 that carries it.
//
// All three public routes are checked: the path form with a menu, the tenant
// address without one, and the host form a {business}.karecik.com page calls.
// The ETag is attached route by route in the router, and a route that lost it
// would still pass a test that only looked at one of them.
func TestPublicMenuIsRevalidatedOnEveryLoad(t *testing.T) {
	h := newHarness(t)

	owner := h.register("owner", "Önbellek Kafe", "cache-owner@example.test")
	menu := h.createMenu(owner, "Ana Menü")
	h.createCategory(owner, menu.ID, "İlk Kategori")

	resp, payload := h.do(http.MethodGet, "/api/business", owner.session, nil)
	h.requireSuccess("read the business slug", resp, payload)
	var business struct {
		Slug string `json:"slug"`
	}
	decodeInto(t, "fixture business", payload, &business)
	if business.Slug == "" {
		t.Fatalf("fixture: the business carried no slug: %s", payload)
	}

	// With a single active menu the tenant address and the host form resolve to
	// that same menu, so every route lists the same categories.
	routes := []struct {
		name    string
		path    string
		headers map[string]string
	}{
		{"path form with a menu", "/api/public/menu/" + business.Slug + "/" + menu.Slug, nil},
		{"tenant address", "/api/public/menu/" + business.Slug, nil},
		{"host form", "/api/public/menu",
			map[string]string{"X-Forwarded-Host": business.Slug + ".karecik.com"}},
	}

	etags := make(map[string]string, len(routes))

	// --- first load, then a revalidation with nothing changed
	for _, route := range routes {
		first, firstBody := h.doWith(http.MethodGet, route.path, "", nil, route.headers)
		if first.StatusCode != http.StatusOK {
			t.Fatalf("%s: GET %s got %d: %s", route.name, route.path, first.StatusCode, firstBody)
		}
		if !strings.Contains(string(firstBody), "İlk Kategori") {
			t.Fatalf("%s: fixture: the first load does not list the first category: %s",
				route.name, firstBody)
		}
		if got := first.Header.Get("Cache-Control"); got != "no-cache" {
			t.Errorf("%s: Cache-Control = %q, want %q: a max-age lets a browser reuse a stale menu "+
				"without asking, which hides a category added moments ago", route.name, got, "no-cache")
		}
		etag := first.Header.Get("ETag")
		if etag == "" {
			t.Errorf("%s: no ETag: with no-cache and no validator, every load downloads the whole "+
				"menu again", route.name)
			continue
		}
		etags[route.name] = etag

		unchanged, _ := h.doWith(http.MethodGet, route.path, "", nil,
			cacheTestHeaders(route.headers, "If-None-Match", etag))
		if unchanged.StatusCode != http.StatusNotModified {
			t.Errorf("%s: revalidating an unchanged menu answered %d, want 304",
				route.name, unchanged.StatusCode)
		}
	}

	// --- the owner adds a category with no icon and no image
	const plainName = "Görselsiz Kategori"
	h.createCategory(owner, menu.ID, plainName)

	for _, route := range routes {
		etag, ok := etags[route.name]
		if !ok {
			continue // already reported above
		}

		changed, changedBody := h.doWith(http.MethodGet, route.path, "", nil,
			cacheTestHeaders(route.headers, "If-None-Match", etag))
		if changed.StatusCode != http.StatusOK {
			t.Errorf("%s: after adding %q, revalidation answered %d instead of 200: the browser "+
				"would keep showing the menu without it", route.name, plainName, changed.StatusCode)
			continue
		}
		if newTag := changed.Header.Get("ETag"); newTag == "" || newTag == etag {
			t.Errorf("%s: the ETag did not change with the menu (before %q, after %q)",
				route.name, etag, newTag)
		}

		var public struct {
			Categories []struct {
				Name     string  `json:"name"`
				Icon     *string `json:"icon"`
				ImageURL *string `json:"image_url"`
			} `json:"categories"`
		}
		decodeInto(t, route.name+": public menu after adding a category", changedBody, &public)

		found := false
		for _, category := range public.Categories {
			if category.Name != plainName {
				continue
			}
			found = true
			if category.Icon != nil || category.ImageURL != nil {
				t.Errorf("%s: %q was created without an icon or an image, but the payload carries "+
					"icon=%v image_url=%v", route.name, plainName, category.Icon, category.ImageURL)
			}
		}
		if !found {
			t.Errorf("%s: the revalidated menu does not list %q: %s", route.name, plainName, changedBody)
		}
	}
}

// cacheTestHeaders returns a copy of headers with one more entry, leaving the
// route's own map untouched for the next request.
func cacheTestHeaders(headers map[string]string, key, value string) map[string]string {
	out := make(map[string]string, len(headers)+1)
	for k, v := range headers {
		out[k] = v
	}
	out[key] = value
	return out
}
