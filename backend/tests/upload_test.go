package tests

// Uploads over HTTP: what is accepted, what is refused before it ever reaches
// the disk, and the headers a stored file is served with.
//
// The rule-by-rule SVG coverage lives in tests/svgsafe; this suite proves the
// wiring — that the handler runs the check, stores what it returns, verifies a
// raster file by its bytes instead of its label, and that /uploads serves an
// SVG so that opening it directly can never run script.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karecik/backend/internal/config"
	"karecik/backend/internal/router"
	"karecik/backend/internal/utils"
)

// A 1x1 transparent PNG and the smallest JPEG header a sniffer recognises.
var (
	tinyPNG, _ = base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")
	tinyJPEG = append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}, make([]byte, 32)...)
)

const cleanLogo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 40"><text x="4" y="28">Melly Coffee Co</text></svg>`

// upload posts one file as multipart/form-data with the given part type.
func upload(t *testing.T, h *harness, session, filename, contentType string, content []byte) (*http.Response, []byte) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if session != "" {
		req.AddCookie(&http.Cookie{Name: utils.SessionCookieName, Value: session})
	}
	resp, err := h.app.Test(req, requestTimeoutMS)
	if err != nil {
		t.Fatalf("POST /api/uploads never completed: %v", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	return resp, payload
}

func uploadedFiles(t *testing.T, h *harness) []string {
	t.Helper()
	entries, err := os.ReadDir(h.cfg.UploadDir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// uploadDir is a scratch upload directory for a suite that SERVES what it
// uploads. t.TempDir cannot be used for it on Windows: fasthttp's file server
// keeps a served file open in its handle cache for up to ten seconds, and
// Windows refuses to delete an open file, so TempDir's own cleanup would fail
// the test. This one retries until the cache has let go — immediately on any
// other system — and only logs if it never does.
func uploadDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "karecik-uploads-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		deadline := time.Now().Add(15 * time.Second)
		for {
			err := os.RemoveAll(dir)
			if err == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Logf("could not remove the scratch upload directory %s: %v", dir, err)
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
	})
	return dir
}

func TestUploads(t *testing.T) {
	dir := uploadDir(t)
	h := newHarnessConfigured(t, nil, func(cfg *config.Config) { cfg.UploadDir = dir })
	owner := h.register("owner", "Yükleme Kafe", "upload-owner@example.test")

	accept := func(t *testing.T, filename, contentType string, content []byte) (string, []byte) {
		t.Helper()
		resp, payload := upload(t, h, owner.session, filename, contentType, content)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("%s (%s) answered %d: %s", filename, contentType, resp.StatusCode, payload)
		}
		var body struct {
			URL  string `json:"url"`
			Size int64  `json:"size"`
		}
		decodeInto(t, "upload", payload, &body)
		stored, err := os.ReadFile(filepath.Join(h.cfg.UploadDir, strings.TrimPrefix(body.URL, "/uploads/")))
		if err != nil {
			t.Fatalf("the upload answered %s but no such file is stored: %v", body.URL, err)
		}
		if body.Size != int64(len(stored)) {
			t.Errorf("size %d, but %d bytes are stored", body.Size, len(stored))
		}
		return body.URL, stored
	}

	t.Run("a_clean_svg_is_stored_as_it_is_and_served_sandboxed", func(t *testing.T) {
		url, stored := accept(t, "logo.svg", "image/svg+xml", []byte(cleanLogo))
		if !strings.HasSuffix(url, ".svg") || string(stored) != cleanLogo {
			t.Fatalf("stored %s as %q", url, stored)
		}

		resp, payload := h.do(http.MethodGet, url, "", nil)
		if resp.StatusCode != http.StatusOK || string(payload) != cleanLogo {
			t.Fatalf("GET %s answered %d: %s", url, resp.StatusCode, payload)
		}
		if got := resp.Header.Get("Content-Type"); got != "image/svg+xml" {
			t.Errorf("Content-Type %q, want image/svg+xml", got)
		}
		if got := resp.Header.Get("Content-Security-Policy"); got != router.UploadPolicy {
			t.Errorf("Content-Security-Policy %q, want %q", got, router.UploadPolicy)
		}
		for _, directive := range []string{"default-src 'none'", "sandbox"} {
			if !strings.Contains(resp.Header.Get("Content-Security-Policy"), directive) {
				t.Errorf("the policy lacks %s", directive)
			}
		}
		if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("X-Content-Type-Options %q, want nosniff", got)
		}
	})

	t.Run("an_svg_without_a_viewbox_gets_one", func(t *testing.T) {
		_, stored := accept(t, "logo.svg", "image/svg+xml",
			[]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="300" height="100"><rect width="300" height="100"/></svg>`))
		if !strings.Contains(string(stored), `viewBox="0 0 300 100"`) {
			t.Errorf("stored %s, want a viewBox added", stored)
		}
	})

	t.Run("an_svg_without_a_namespace_gets_one", func(t *testing.T) {
		// Markup copied out of an HTML page: without xmlns a browser draws
		// nothing from the stored file, so the logo would silently vanish.
		_, stored := accept(t, "logo.svg", "image/svg+xml",
			[]byte(`<svg viewBox="0 0 10 10"><rect width="1" height="1"/></svg>`))
		want := `<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"/></svg>`
		if string(stored) != want {
			t.Errorf("stored %s, want %s", stored, want)
		}
	})

	t.Run("an_svg_named_by_its_extension_is_checked_too", func(t *testing.T) {
		accept(t, "logo.svg", "application/octet-stream", []byte(cleanLogo))
		before := len(uploadedFiles(t, h))
		resp, _ := upload(t, h, owner.session, "logo.svg", "application/octet-stream",
			[]byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`))
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("a script-carrying SVG named .svg answered %d", resp.StatusCode)
		}
		if after := len(uploadedFiles(t, h)); after != before {
			t.Error("a refused SVG was written to the upload directory")
		}
	})

	t.Run("raster_images_are_verified_by_their_bytes", func(t *testing.T) {
		url, _ := accept(t, "photo.png", "image/png", tinyPNG)
		resp, _ := h.do(http.MethodGet, url, "", nil)
		// The policy is on every upload, a raster image included: telling an
		// SVG apart by its address is what cannot be trusted.
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
			resp.Header.Get("Content-Security-Policy") != router.UploadPolicy {
			t.Errorf("a PNG is served with nosniff %q and CSP %q, want nosniff and the upload policy",
				resp.Header.Get("X-Content-Type-Options"), resp.Header.Get("Content-Security-Policy"))
		}
		if !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/png") {
			t.Errorf("a PNG is served as %q", resp.Header.Get("Content-Type"))
		}

		// A real JPEG sent as PNG is stored as what it is.
		if url, _ := accept(t, "photo.png", "image/png", tinyJPEG); !strings.HasSuffix(url, ".jpg") {
			t.Errorf("a JPEG labelled PNG was stored as %s", url)
		}
	})

	t.Run("refusals_write_nothing", func(t *testing.T) {
		before := len(uploadedFiles(t, h))
		for _, tc := range []struct {
			name, filename, contentType string
			content                     []byte
			status                      int
		}{
			{"html_labelled_png", "x.png", "image/png", []byte("<html><script>alert(1)</script></html>"), 422},
			{"svg_labelled_png", "x.png", "image/png", []byte(cleanLogo), 422},
			{"png_labelled_svg", "x.svg", "image/svg+xml", tinyPNG, 422},
			{"script_svg", "x.svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), 422},
			{"external_reference_svg", "x.svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><image href="https://tracker.example/p.png"/></svg>`), 422},
			{"text_file", "notes.txt", "text/plain", []byte("hello"), 422},
			{"empty_png", "x.png", "image/png", []byte{}, 422},
			// Over MAX_UPLOAD_BYTES (5 MB) but inside the app's body limit,
			// so it is the handler's own check that answers.
			{"too_large", "big.png", "image/png", append(append([]byte{}, tinyPNG...), make([]byte, 5*1024*1024+1024)...), 413},
		} {
			t.Run(tc.name, func(t *testing.T) {
				resp, payload := upload(t, h, owner.session, tc.filename, tc.contentType, tc.content)
				if resp.StatusCode != tc.status {
					t.Errorf("answered %d, want %d: %s", resp.StatusCode, tc.status, payload)
				}
			})
		}
		if after := len(uploadedFiles(t, h)); after != before {
			t.Errorf("refused uploads wrote %d file(s)", after-before)
		}
	})

	t.Run("an_upload_is_audited", func(t *testing.T) {
		var rows int
		var changes []byte
		if err := h.pool.QueryRow(context.Background(), `
			SELECT COUNT(*) OVER (), changes::text FROM audit_logs
			WHERE action = 'upload.create' ORDER BY created_at LIMIT 1`).Scan(&rows, &changes); err != nil {
			t.Fatalf("no upload.create row: %v", err)
		}
		if rows != 6 {
			t.Errorf("%d upload.create rows, want one per accepted upload (6)", rows)
		}
		var decoded map[string]struct{ New any }
		if err := json.Unmarshal(changes, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded["content_type"].New != "image/svg+xml" || decoded["url"].New == nil || decoded["size"].New == nil {
			t.Errorf("the first upload row records %s", changes)
		}
	})

	t.Run("anonymous_uploads_are_refused", func(t *testing.T) {
		resp, _ := upload(t, h, "", "logo.svg", "image/svg+xml", []byte(cleanLogo))
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("an anonymous upload answered %d", resp.StatusCode)
		}
	})
}

// Every response /uploads gives for a stored file carries the policy and
// nosniff, however the address was spelled and whatever form the response
// takes — and an SVG is always served as one.
//
// The headers used to be chosen by the extension of the address exactly as
// the client spelled it, while the static handler finds the file from the
// decoded and normalised path. Every spelling below reaches the same stored
// file; the ones that do on this system must all be answered with the same
// headers. The suite also insists that the spellings that decode to the stored
// name really are served, so it cannot pass by exercising nothing.
func TestUploadHeadersDoNotDependOnHowTheAddressIsSpelled(t *testing.T) {
	dir := uploadDir(t)
	h := newHarnessConfigured(t, nil, func(cfg *config.Config) { cfg.UploadDir = dir })
	owner := h.register("owner", "Adres Kafe", "upload-spelling@example.test")

	stored := func(filename, contentType string, content []byte) string {
		t.Helper()
		resp, payload := upload(t, h, owner.session, filename, contentType, content)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("fixture upload %s answered %d: %s", filename, resp.StatusCode, payload)
		}
		var body struct {
			URL string `json:"url"`
		}
		decodeInto(t, "upload", payload, &body)
		return strings.TrimPrefix(body.URL, "/uploads/")
	}
	svgName := stored("logo.svg", "image/svg+xml", []byte(cleanLogo))
	pngName := stored("photo.png", "image/png", tinyPNG)

	send := func(t *testing.T, method, target string, headers map[string]string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(method, target, nil)
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := h.app.Test(req, requestTimeoutMS)
		if err != nil {
			t.Fatalf("%s %s never completed: %v", method, target, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp
	}

	// requireGuarded checks one answer: nothing at all for a miss, the full set
	// of headers for anything else.
	requireGuarded := func(t *testing.T, what string, resp *http.Response, svg bool) {
		t.Helper()
		if resp.StatusCode == http.StatusNotFound {
			return
		}
		if got := resp.Header.Get("Content-Security-Policy"); got != router.UploadPolicy {
			t.Errorf("%s answered %d with Content-Security-Policy %q, want %q",
				what, resp.StatusCode, got, router.UploadPolicy)
		}
		if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s answered %d with X-Content-Type-Options %q, want nosniff", what, resp.StatusCode, got)
		}
		if svg && resp.StatusCode != http.StatusNotModified {
			if got := resp.Header.Get("Content-Type"); got != "image/svg+xml" {
				t.Errorf("%s answered %d with Content-Type %q, want image/svg+xml", what, resp.StatusCode, got)
			}
		}
	}

	// spellings turns a stored name into addresses of the same file. served
	// marks the ones that decode to exactly the stored name under /uploads,
	// which every file server has to find.
	type spelling struct {
		name   string
		target string
		served bool
	}
	spellings := func(file string) []spelling {
		base, ext := strings.TrimSuffix(file, filepath.Ext(file)), strings.TrimPrefix(filepath.Ext(file), ".")
		encodedExt := ""
		for _, r := range ext {
			encodedExt += "%" + strings.ToUpper(hex2(byte(r)))
		}
		return []spelling{
			{"canonical", "/uploads/" + file, true},
			{"a_query_string", "/uploads/" + file + "?v=2", true},
			{"an_encoded_dot", "/uploads/" + base + "%2E" + ext, true},
			{"an_encoded_lowercase_dot", "/uploads/" + base + "%2e" + ext, true},
			{"an_encoded_extension", "/uploads/" + base + "." + encodedExt, true},
			{"a_repeated_slash", "/uploads//" + file, true},
			{"a_dot_segment", "/uploads/./" + file, true},
			{"a_dot_dot_segment", "/uploads/x/../" + file, true},
			{"an_uppercase_prefix", "/UPLOADS/" + file, true},
			{"a_trailing_slash", "/uploads/" + file + "/", false},
			{"an_uppercase_extension", "/uploads/" + base + "." + strings.ToUpper(ext), false},
			{"an_encoded_uppercase_extension", "/uploads/" + base + "." + strings.ToUpper(encodedExt), false},
		}
	}

	for _, file := range []struct {
		name string
		svg  bool
	}{{svgName, true}, {pngName, false}} {
		for _, s := range spellings(file.name) {
			t.Run(strings.TrimPrefix(filepath.Ext(file.name), ".")+"/"+s.name, func(t *testing.T) {
				resp := send(t, http.MethodGet, s.target, nil)
				if s.served && resp.StatusCode != http.StatusOK {
					t.Fatalf("GET %s answered %d, want the stored file", s.target, resp.StatusCode)
				}
				requireGuarded(t, "GET "+s.target, resp, file.svg)

				head := send(t, http.MethodHead, s.target, nil)
				requireGuarded(t, "HEAD "+s.target, head, file.svg)

				partial := send(t, http.MethodGet, s.target, map[string]string{"Range": "bytes=0-9"})
				if s.served && partial.StatusCode != http.StatusPartialContent {
					t.Errorf("a range request for %s answered %d, want 206", s.target, partial.StatusCode)
				}
				requireGuarded(t, "a range GET "+s.target, partial, file.svg)

				if modified := resp.Header.Get("Last-Modified"); modified != "" {
					again := send(t, http.MethodGet, s.target, map[string]string{"If-Modified-Since": modified})
					if s.served && again.StatusCode != http.StatusNotModified {
						t.Errorf("a revalidation of %s answered %d, want 304", s.target, again.StatusCode)
					}
					requireGuarded(t, "a revalidation of "+s.target, again, file.svg)
				} else if s.served {
					t.Errorf("GET %s carried no Last-Modified to revalidate against", s.target)
				}
			})
		}
	}
}

// hex2 is a byte in two hexadecimal digits.
func hex2(b byte) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[b>>4], digits[b&0x0f]})
}
