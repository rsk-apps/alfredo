package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/logger"
)

var validate = validator.New()

type errorResponse struct {
	Error   string       `json:"error"`
	Message string       `json:"message"`
	Fields  []fieldError `json:"fields,omitempty"`
}

type fieldError struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

func newErrorResponse(code, message string, fields []fieldError) errorResponse {
	return errorResponse{Error: code, Message: message, Fields: fields}
}

func parseUUID(c echo.Context, param string) (string, bool) {
	val := c.Param(param)
	if _, err := uuid.Parse(val); err != nil {
		_ = c.JSON(http.StatusBadRequest, newErrorResponse(
			"invalid_param",
			param+" must be a valid UUID",
			nil,
		))
		return "", false
	}
	return val, true
}

func validateRequest(c echo.Context, req any) bool {
	if err := validate.Struct(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			fields := make([]fieldError, len(ve))
			for i, fe := range ve {
				fields[i] = fieldError{
					Field: strings.ToLower(fe.Field()),
					Issue: fe.Tag(),
				}
			}
			_ = c.JSON(http.StatusBadRequest, newErrorResponse(
				"validation_failed",
				"Request validation failed",
				fields,
			))
			return false
		}
		_ = c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", err.Error(), nil))
		return false
	}
	return true
}

func mapError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		logger.SetError(c, "not_found")
		return c.JSON(http.StatusNotFound, newErrorResponse("not_found", "Resource not found", nil))
	case errors.Is(err, domain.ErrValidation):
		logger.SetError(c, "validation_failed")
		msg := strings.TrimPrefix(err.Error(), "validation failed: ")
		return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", msg, nil))
	default:
		logger.SetError(c, "internal_error")
		return c.JSON(http.StatusInternalServerError, newErrorResponse("internal_error", "An unexpected error occurred", nil))
	}
}
