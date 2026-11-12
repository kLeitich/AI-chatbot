package main

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Standard machine-readable error codes
const (
	ErrCodeTokenExpired     = "TOKEN_EXPIRED"
	ErrCodeTokenRevoked     = "TOKEN_REVOKED"
	ErrCodeAccountDisabled  = "ACCOUNT_DISABLED"
	ErrCodePermissionDenied = "PERMISSION_DENIED"
	ErrCodeSlotTaken        = "SLOT_TAKEN"
	ErrCodeValidationError  = "VALIDATION_ERROR"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeServerError      = "SERVER_ERROR"
)

// AppError is a structured error carrying an HTTP status code, message, and machine-readable error code
type AppError struct {
	StatusCode   int      `json:"-"`
	Message      string   `json:"message"`
	Code         string   `json:"code"`
	Alternatives []string `json:"alternatives,omitempty"`
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(status int, code, message string) *AppError {
	return &AppError{StatusCode: status, Code: code, Message: message}
}

// sendError helper sends a JSON response with status, message, and machine-readable code
func sendError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"message": message,
		"code":    code,
	})
}

// customErrorHandler intercepts all errors returned from handlers and middleware
// to guarantee JSON responses shaped as {message, code}
func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	errCode := ErrCodeServerError
	msg := "internal server error"

	var appErr *AppError
	if errors.As(err, &appErr) {
		res := fiber.Map{
			"message": appErr.Message,
			"code":    appErr.Code,
		}
		if len(appErr.Alternatives) > 0 {
			res["alternatives"] = appErr.Alternatives
		}
		return c.Status(appErr.StatusCode).JSON(res)
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		msg = fiberErr.Message
		switch code {
		case fiber.StatusUnauthorized:
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "revoked") {
				errCode = ErrCodeTokenRevoked
			} else if strings.Contains(lower, "disabled") {
				errCode = ErrCodeAccountDisabled
			} else {
				errCode = ErrCodeTokenExpired
			}
		case fiber.StatusForbidden:
			errCode = ErrCodePermissionDenied
		case fiber.StatusConflict:
			errCode = ErrCodeSlotTaken
		case fiber.StatusBadRequest, fiber.StatusUnprocessableEntity:
			errCode = ErrCodeValidationError
		case fiber.StatusNotFound:
			errCode = ErrCodeNotFound
		case fiber.StatusTooManyRequests:
			errCode = "TOO_MANY_REQUESTS"
		}
	} else if err != nil {
		msg = err.Error()
	}

	return c.Status(code).JSON(fiber.Map{
		"message": msg,
		"code":    errCode,
	})
}

