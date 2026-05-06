// Package response provides standardized API response helpers.
// Every handler uses these to guarantee a consistent JSON envelope.
package response

import "github.com/gofiber/fiber/v2"

// Envelope is the standard JSON wrapper returned by all endpoints.
//
//	{ "success": true, "data": {...}, "message": "ok" }
type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// Paginated wraps a list response with pagination metadata.
type Paginated struct {
	Items      interface{} `json:"items"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PerPage    int         `json:"per_page"`
	TotalPages int         `json:"total_pages"`
}

// OK sends a 200 success response.
func OK(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Envelope{
		Success: true,
		Data:    data,
	})
}

// Created sends a 201 response.
func Created(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(Envelope{
		Success: true,
		Data:    data,
	})
}

// NoContent sends a 204 with no body.
func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// Error sends an error response with the given status code.
func Error(c *fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(Envelope{
		Success: false,
		Message: msg,
	})
}

// BadRequest is a convenience for 400.
func BadRequest(c *fiber.Ctx, msg string) error {
	return Error(c, fiber.StatusBadRequest, msg)
}

// Unauthorized is a convenience for 401.
func Unauthorized(c *fiber.Ctx, msg string) error {
	return Error(c, fiber.StatusUnauthorized, msg)
}

// Forbidden is a convenience for 403.
func Forbidden(c *fiber.Ctx, msg string) error {
	return Error(c, fiber.StatusForbidden, msg)
}

// NotFound is a convenience for 404.
func NotFound(c *fiber.Ctx, msg string) error {
	return Error(c, fiber.StatusNotFound, msg)
}

// InternalError is a convenience for 500.
func InternalError(c *fiber.Ctx, msg string) error {
	return Error(c, fiber.StatusInternalServerError, msg)
}
