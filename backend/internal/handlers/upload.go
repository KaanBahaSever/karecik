package handlers

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"karecik/backend/internal/utils"
)

// Accepted image content types and their file extensions.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
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
				"Yalnızca JPG, PNG, WEBP veya GIF formatında görsel yükleyebilirsiniz.")
		}
	}

	filename := fmt.Sprintf("%d-%s%s", time.Now().Unix(), uuid.NewString()[:8], extension)
	destination := filepath.Join(h.Cfg.UploadDir, filename)

	if err := c.SaveFile(fileHeader, destination); err != nil {
		return utils.Internal(c, fmt.Errorf("could not save the uploaded file: %w", err))
	}

	return utils.Created(c, fiber.Map{
		"url":  "/uploads/" + filename,
		"size": fileHeader.Size,
	})
}
