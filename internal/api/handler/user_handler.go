// Package handler provides HTTP handlers for the API.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ehienabs/eagle-bank/internal/api/middleware"
	"github.com/ehienabs/eagle-bank/internal/domain"
	"github.com/ehienabs/eagle-bank/internal/service"
	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	userService *service.UserService
	authService *service.AuthService
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(userService *service.UserService, authService *service.AuthService) *UserHandler {
	return &UserHandler{
		userService: userService,
		authService: authService,
	}
}

// CreateUser handles POST /v1/users
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req domain.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, errors.InvalidInput("invalid request body"))
		return
	}

	user, err := h.userService.CreateUser(c.Request.Context(), &req)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, user.ToResponse())
}

// GetUser handles GET /v1/users/:userId
func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.Param("userId")
	requestingUserID := middleware.GetUserID(c)

	// Users can only access their own data
	if userID != requestingUserID {
		respondError(c, errors.Forbidden("you can only access your own data"))
		return
	}

	user, err := h.userService.GetUser(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, user.ToResponse())
}

// UpdateUser handles PATCH /v1/users/:userId
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("userId")
	requestingUserID := middleware.GetUserID(c)

	// Users can only update their own data
	if userID != requestingUserID {
		respondError(c, errors.Forbidden("you can only update your own data"))
		return
	}

	var req domain.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, errors.InvalidInput("invalid request body"))
		return
	}

	user, err := h.userService.UpdateUser(c.Request.Context(), userID, &req)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, user.ToResponse())
}

// DeleteUser handles DELETE /v1/users/:userId
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("userId")
	requestingUserID := middleware.GetUserID(c)

	// Users can only delete their own data
	if userID != requestingUserID {
		respondError(c, errors.Forbidden("you can only delete your own data"))
		return
	}

	if err := h.userService.DeleteUser(c.Request.Context(), userID); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Login handles POST /v1/auth/login
func (h *UserHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, errors.InvalidInput("invalid request body"))
		return
	}

	tokens, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// respondError sends an error response
func respondError(c *gin.Context, err error) {
	status := errors.GetHTTPStatus(err)
	response := errors.ToResponse(err)
	c.JSON(status, response)
}
