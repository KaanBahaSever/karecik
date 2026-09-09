package utils

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

// ErrorBody is the shared shape of every error response.
//
// NOTE: the messages passed to these helpers are shown to the end user and are
// therefore written in Turkish on purpose.
type ErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Fail returns a JSON error response with the given HTTP status and error code.
func Fail(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(ErrorBody{Error: message, Code: code})
}

// BadRequest — 400: the request body could not be parsed.
func BadRequest(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", message)
}

// Unauthorized — 401: the CREDENTIALS in this request were wrong.
//
// A wrong password on the login form, or the wrong current password when
// changing it. The caller is asking a question and the answer is no; their
// session, if they have one, is untouched.
func Unauthorized(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusUnauthorized, "UNAUTHORIZED", message)
}

// SessionExpired — 401: there is no usable SESSION behind this request.
//
// Same status as Unauthorized, deliberately different code, and the difference
// matters to the client. Both are 401, so a browser that only looks at the
// status cannot tell "you typed the wrong password" from "you are no longer
// signed in" — and it has to, because the correct response to the first is to
// show the error on the form and to the second is to clear the local session
// and go to the login page. Redirecting on the first would throw a user out of
// the panel for mistyping their own password.
func SessionExpired(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusUnauthorized, "SESSION_EXPIRED", message)
}

// Forbidden — 403: the record belongs to another business.
func Forbidden(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusForbidden, "FORBIDDEN", message)
}

// NotFound — 404: no such record.
func NotFound(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusNotFound, "NOT_FOUND", message)
}

// Conflict — 409: duplicate email or slug.
func Conflict(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusConflict, "CONFLICT", message)
}

// Unprocessable — 422: validation error.
func Unprocessable(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", message)
}

// TooLarge — 413: upload size exceeded.
func TooLarge(c *fiber.Ctx, message string) error {
	return Fail(c, fiber.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", message)
}

// Internal — 500: unexpected failure. The details are logged, never leaked.
func Internal(c *fiber.Ctx, err error) error {
	log.Printf("[karecik] internal error: %v", err)
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
