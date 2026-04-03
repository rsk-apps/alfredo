package middleware

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/rafaelsoares/alfredo/internal/logger"
)

const maxBodyBytes = 64 * 1024

// RequestLogger returns an Echo middleware that:
//   - generates a request_id (UUID v4)
//   - creates a child zap logger with request_id attached and stores it in context
//   - buffers the request body (up to 64KB) for non-GET requests, restoring it for the handler
//   - logs a completion entry at INFO (2xx), WARN (4xx), or ERROR (5xx) after the handler returns
func RequestLogger(root *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqID := uuid.New().String()
			child := root.With(zap.String("request_id", reqID))
			logger.Set(c, child)
			isDebug := root.Core().Enabled(zapcore.DebugLevel)

			req := c.Request()
			start := time.Now()

			var body, query string
			if req.Method != http.MethodGet && req.Body != nil {
				buf, _ := io.ReadAll(io.LimitReader(req.Body, maxBodyBytes))
				req.Body = io.NopCloser(bytes.NewReader(buf))
				body = string(buf)
			} else {
				query = req.URL.RawQuery
			}

			err := next(c)

			status := c.Response().Status
			durationMs := time.Since(start).Milliseconds()
			errStr := logger.GetError(c)

			base := []zap.Field{
				zap.String("client_ip", c.RealIP()),
				zap.String("method", req.Method),
				zap.String("path", req.URL.Path),
				zap.Int("status", status),
				zap.Int64("duration_ms", durationMs),
			}

			switch {
			case status >= 500:
				fields := append(base, zap.String("error", errStr))
				if isDebug {
					fields = append(fields, zap.String("request_body", body), zap.String("query", query))
				}
				child.Error("request", fields...)
			case status >= 400:
				fields := append(base, zap.String("error", errStr))
				if isDebug {
					fields = append(fields, zap.String("request_body", body), zap.String("query", query))
				}
				child.Warn("request", fields...)
			default:
				child.Info("request", base...)
			}

			return err
		}
	}
}
