// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ehienabs/eagle-bank/internal/service"
	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// AuthMiddleware creates an authentication middleware
func AuthMiddleware(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			respondError(c, errors.Unauthorized("missing authorization header"))
			c.Abort()
			return
		}

		// Extract Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			respondError(c, errors.Unauthorized("invalid authorization header format"))
			c.Abort()
			return
		}

		token := parts[1]

		// Validate token
		claims, err := authService.ValidateToken(token)
		if err != nil {
			respondError(c, err)
			c.Abort()
			return
		}

		// Set user ID in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)

		// Also set in request context for service layer
		ctx := service.ContextWithUserID(c.Request.Context(), claims.UserID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetUserID extracts user ID from gin context
func GetUserID(c *gin.Context) string {
	if userID, ok := c.Get("user_id"); ok {
		return userID.(string)
	}
	return ""
}

// respondError sends an error response
func respondError(c *gin.Context, err error) {
	status := errors.GetHTTPStatus(err)
	response := errors.ToResponse(err)
	c.JSON(status, response)
}
