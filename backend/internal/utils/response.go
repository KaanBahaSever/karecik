package utils

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

// ErrorBody, tum hata yanitlarinin ortak govdesidir.
type ErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Fail, verilen durum kodu ve hata koduyla JSON hata yaniti dondurur.
func Fail(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(ErrorBody{Error: message, Code: code})
}

// BadRequest — 400: istek govdesi cozulemedi.
func BadRequest(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", message)
}

// Unauthorized — 401: token yok ya da gecersiz.
func Unauthorized(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusUnauthorized, "UNAUTHORIZED", message)
}

// Forbidden — 403: kayit baska bir isletmeye ait.
func Forbidden(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusForbidden, "FORBIDDEN", message)
}

// NotFound — 404: kayit bulunamadi.
func NotFound(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusNotFound, "NOT_FOUND", message)
}

// Conflict — 409: e-posta veya slug cakismasi.
func Conflict(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusConflict, "CONFLICT", message)
}

// Unprocessable — 422: dogrulama hatasi.
func Unprocessable(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", message)
}

// TooLarge — 413: dosya boyutu asildi.
func TooLarge(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", message)
}

// Internal — 500: beklenmeyen hata. Ayrintiyi log'a yazar, istemciye sizdirmaz.
func Internal(c *fiber.Ctx, err error) error {
	log.Printf("[karecik] sunucu hatasi: %v", err)
	return Fail(c, fiber.StatusInternalServerError, "INTERNAL_ERROR",
		"Sunucuda beklenmeyen bir hata olustu.")
}

// OK — 200.
func OK(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(data)
}

// Created — 201.
func Created(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(data)
}
