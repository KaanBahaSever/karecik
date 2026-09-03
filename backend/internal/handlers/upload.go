package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/utils"
)

// Accepted image content types and their file extensions.
// SVG is allowed so a wide, rectangular logo keeps its sharpness, but it is
// script-capable and therefore checked by scanSVG before it is served.
var allowedImageTypes = map[string]string{
	"image/jpeg":    ".jpg",
	"image/jpg":     ".jpg",
	"image/png":     ".png",
	"image/webp":    ".webp",
	"image/gif":     ".gif",
	"image/svg+xml": ".svg",
}

// Patterns that make an SVG active content. We serve uploads inline from our
// own origin, so a file carrying any of them would run in the visitor's page.
var svgScriptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<\s*script`),
	regexp.MustCompile(`(?i)javascript\s*:`),
	regexp.MustCompile(`(?i)\son[a-z]+\s*=`),
}

// Upload — POST /api/uploads (multipart/form-data, field name: "file")
// Stores logos and product images and returns the URL to reach them.
func (h *Handler) Upload(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return utils.BadRequest(c, "Yüklenecek dosya bulunamadı (alan adı: file).")
	}

	if fileHeader.Size > h.Cfg.MaxUploadBytes {
		return utils.TooLarge(c, fmt.Sprintf(
			"Dosya çok büyük. En fazla %d MB yükleyebilirsiniz.",
			h.Cfg.MaxUploadBytes/(1024*1024)))
	}

	contentType := strings.ToLower(strings.TrimSpace(
		strings.Split(fileHeader.Header.Get("Content-Type"), ";")[0]))

	extension, ok := allowedImageTypes[contentType]
	if !ok {
		// Some browsers send an empty type; fall back to the file extension.
		extension = strings.ToLower(filepath.Ext(fileHeader.Filename))
		valid := false
		for _, allowed := range allowedImageTypes {
			if extension == allowed {
				valid = true
				break
			}
		}
		if !valid {
			return utils.Unprocessable(c,
				"Yalnızca JPG, PNG, WEBP, GIF veya SVG formatında görsel yükleyebilirsiniz.")
		}
	}

	filename := fmt.Sprintf("%d-%s%s", time.Now().Unix(), uuid.NewString()[:8], extension)
	destination := filepath.Join(h.Cfg.UploadDir, filename)

	if err := c.SaveFile(fileHeader, destination); err != nil {
		return utils.Internal(c, fmt.Errorf("could not save the uploaded file: %w", err))
	}

	// An SVG is markup, not a picture: it is read back and thrown away again
	// when it carries anything executable.
	if extension == ".svg" {
		clean, err := scanSVG(destination)
		if err != nil {
			_ = os.Remove(destination)
			return utils.Internal(c, fmt.Errorf("could not read the uploaded SVG: %w", err))
		}
		if !clean {
			_ = os.Remove(destination)
			return utils.Unprocessable(c,
				"SVG dosyası betik içeremez. Lütfen sadeleştirilmiş bir SVG yükleyin.")
		}
	}

	return utils.Created(c, fiber.Map{
		"url":  "/uploads/" + filename,
		"size": fileHeader.Size,
	})
}

// scanSVG reports whether a saved SVG file is free of active content:
// <script tags, javascript: URLs and on<event>= attributes.
func scanSVG(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	for _, pattern := range svgScriptPatterns {
		if pattern.Match(content) {
			return false, nil
		}
	}
	return true, nil
}
