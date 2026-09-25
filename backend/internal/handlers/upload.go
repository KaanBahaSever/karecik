package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/audit"
	"karecik/backend/internal/svgsafe"
	"karecik/backend/internal/utils"
)

// Accepted image content types and their file extensions — what a request may
// CLAIM a file is. The claim only picks the check the bytes then go through:
// an SVG is parsed and vetted by svgsafe, and a raster image has to prove its
// type by its magic bytes (rasterTypes). The extension the stored file gets is
// always the one its verified content earns.
var allowedImageTypes = map[string]string{
	"image/jpeg":    ".jpg",
	"image/jpg":     ".jpg",
	"image/png":     ".png",
	"image/webp":    ".webp",
	"image/gif":     ".gif",
	"image/svg+xml": ".svg",
}

// allowedExtensions is the fallback claim, for a browser that sends an empty or
// generic Content-Type: the file name's extension.
var allowedExtensions = map[string]string{
	".jpg": ".jpg", ".jpeg": ".jpg", ".png": ".png", ".webp": ".webp",
	".gif": ".gif", ".svg": ".svg",
}

// rasterTypes maps what http.DetectContentType reads from a file's first bytes
// onto the extension it is stored under. Anything the sniffer does not name as
// one of these four is not a raster image this service stores — an HTML page
// renamed to logo.png comes back "text/html" and is refused, which is the
// whole point: a file served from our own origin must be exactly what its
// extension says.
var rasterTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// Upload — POST /api/uploads (multipart/form-data, field name: "file")
// Stores logos and product images and returns the URL to reach them.
//
// The file is read into memory, checked, and only then written to disk — a
// file that fails a check never touches the upload directory, so nothing can
// serve it in the meantime.
//
//   - an SVG (by Content-Type, or by a .svg name when the type is generic) must
//     pass svgsafe.Check; the bytes stored are the ones it returns, which only
//     ever differ from the upload by a namespace declaration or a viewBox
//     added to the root;
//   - anything else must sniff as a JPEG, PNG, GIF or WebP by its magic bytes,
//     and is stored under the extension of what it really is.
func (h *Handler) Upload(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return utils.BadRequest(c, "Yüklenecek dosya bulunamadı (alan adı: file).")
	}

	tooLarge := func() error {
		return utils.TooLarge(c, fmt.Sprintf(
			"Dosya çok büyük. En fazla %d MB yükleyebilirsiniz.",
			h.Cfg.MaxUploadBytes/(1024*1024)))
	}
	if fileHeader.Size > h.Cfg.MaxUploadBytes {
		return tooLarge()
	}

	contentType := strings.ToLower(strings.TrimSpace(
		strings.Split(fileHeader.Header.Get("Content-Type"), ";")[0]))
	claimed, ok := allowedImageTypes[contentType]
	if !ok {
		// Some browsers send an empty or generic type; fall back to the name.
		claimed, ok = allowedExtensions[strings.ToLower(filepath.Ext(fileHeader.Filename))]
		if !ok {
			return utils.Unprocessable(c,
				"Yalnızca JPG, PNG, WEBP, GIF veya SVG formatında görsel yükleyebilirsiniz.")
		}
	}

	file, err := fileHeader.Open()
	if err != nil {
		return utils.Internal(c, fmt.Errorf("could not open the uploaded file: %w", err))
	}
	// One byte more than the limit, so a header that under-reports the size is
	// caught by what was actually read.
	data, err := io.ReadAll(io.LimitReader(file, h.Cfg.MaxUploadBytes+1))
	_ = file.Close()
	if err != nil {
		return utils.Internal(c, fmt.Errorf("could not read the uploaded file: %w", err))
	}
	if int64(len(data)) > h.Cfg.MaxUploadBytes {
		return tooLarge()
	}

	var extension, storedType string
	if claimed == ".svg" {
		// An SVG is markup, not a picture: it is parsed and refused when it
		// carries anything that could run or fetch — see package svgsafe.
		clean, err := svgsafe.Check(data)
		if err != nil {
			// The rule that failed goes to the log, not to the uploader: it is
			// a hint for whoever debugs a rejected logo, and a map for nobody
			// else.
			log.Printf("[karecik] upload refused: %v", err)
			return utils.Unprocessable(c,
				"SVG dosyası betik, dış bağlantı veya gömülü içerik barındıramaz. "+
					"Lütfen sadeleştirilmiş bir SVG yükleyin.")
		}
		data, extension, storedType = clean, ".svg", "image/svg+xml"
	} else {
		detected := http.DetectContentType(data)
		sniffed, ok := rasterTypes[detected]
		if !ok {
			log.Printf("[karecik] upload refused: claimed %q (%s) but the content reads as %q",
				contentType, claimed, detected)
			return utils.Unprocessable(c,
				"Dosyanın içeriği geçerli bir JPG, PNG, WEBP veya GIF görseli değil.")
		}
		extension, storedType = sniffed, detected
	}

	filename := fmt.Sprintf("%d-%s%s", time.Now().Unix(), uuid.NewString()[:8], extension)
	destination := filepath.Join(h.Cfg.UploadDir, filename)
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		_ = os.Remove(destination)
		return utils.Internal(c, fmt.Errorf("could not save the uploaded file: %w", err))
	}

	url := "/uploads/" + filename
	size := int64(len(data))

	// An upload writes no database row, so it has no transaction for its audit
	// row to join: it is recorded now, best-effort — the file is already
	// stored, and a failure here is logged rather than failing the upload.
	h.recordNow(c, auditRecord{
		action:     audit.ActionUploadCreate,
		entityType: audit.EntityUpload,
		entityID:   filename,
		label:      displayFilename(fileHeader.Filename),
		changes: audit.Changes{
			"url":          {Old: nil, New: url},
			"size":         {Old: nil, New: size},
			"content_type": {Old: nil, New: storedType},
		},
	})

	return utils.Created(c, fiber.Map{
		"url":  url,
		"size": size,
	})
}

// displayFilename is the uploader's own name for a file, fit for a label: the
// base name only, with control and format characters removed, and "" when that
// leaves nothing — or leaves something PostgreSQL could not store.
func displayFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	if name == "." || name == "/" {
		return ""
	}
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, name)
	if UnstorableText(cleaned) {
		return ""
	}
	return strings.TrimSpace(cleaned)
}
