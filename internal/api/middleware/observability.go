package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ehienabs/eagle-bank/pkg/logger"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
	"github.com/ehienabs/eagle-bank/pkg/tracing"
	"go.opentelemetry.io/otel/propagation"
)

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// After request
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		bodySize := c.Writer.Size()

		if raw != "" {
			path = path + "?" + raw
		}

		requestID, _ := c.Get("request_id")
		userID := GetUserID(c)

		// Create request logger
		reqLogger := log.WithFields(map[string]any{
			"request_id":  requestID,
			"client_ip":   clientIP,
			"method":      method,
			"path":        path,
			"status":      statusCode,
			"latency_ms":  latency.Milliseconds(),
			"body_size":   bodySize,
			"user_agent":  c.Request.UserAgent(),
		})

		if userID != "" {
			reqLogger = reqLogger.WithUserID(userID)
		}

		// Log at appropriate level based on status
		switch {
		case statusCode >= 500:
			reqLogger.Error("server error")
		case statusCode >= 400:
			reqLogger.Warn("client error")
		default:
			reqLogger.Info("request completed")
		}
	}
}

// MetricsMiddleware records HTTP metrics
func MetricsMiddleware(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		method := c.Request.Method

		// Increment in-flight requests
		m.IncHTTPInFlight()

		// Process request
		c.Next()

		// After request
		m.DecHTTPInFlight()

		duration := time.Since(start)
		status := c.Writer.Status()
		responseSize := c.Writer.Size()

		m.RecordHTTPRequest(method, path, status, duration, responseSize)
	}
}

// TracingMiddleware adds distributed tracing to requests
func TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract trace context from incoming headers
		ctx := tracing.ExtractTraceContext(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// Start span
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		ctx, span := tracing.StartHTTPSpan(ctx, c.Request.Method, path)
		defer span.End()

		// Add request ID if present
		if requestID, ok := c.Get("request_id"); ok {
			span.SetAttributes(tracing.RequestIDAttr(requestID.(string)))
		}

		// Set context
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Record status
		if c.Writer.Status() >= 400 {
			span.SetAttributes(
				tracing.RequestIDAttr(c.GetString("request_id")),
			)
		}

		if c.Writer.Status() >= 500 {
			span.SetError(nil) // Mark as error
		} else {
			span.SetOK()
		}
	}
}

// RecoveryMiddleware recovers from panics and logs them
func RecoveryMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("panic recovered",
					"error", err,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)
				c.AbortWithStatusJSON(500, gin.H{
					"message": "internal server error",
				})
			}
		}()
		c.Next()
	}
}

// CORSMiddleware handles CORS
func CORSMiddleware(allowedOrigins, allowedMethods, allowedHeaders []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", joinStrings(allowedMethods))
			c.Header("Access-Control-Allow-Headers", joinStrings(allowedHeaders))
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func joinStrings(s []string) string {
	result := ""
	for i, str := range s {
		if i > 0 {
			result += ", "
		}
		result += str
	}
	return result
}
