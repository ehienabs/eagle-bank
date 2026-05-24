package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ehienabs/eagle-bank/internal/api/middleware"
	"github.com/ehienabs/eagle-bank/internal/domain"
	"github.com/ehienabs/eagle-bank/internal/service"
	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// AccountHandler handles account-related HTTP requests
type AccountHandler struct {
	accountService *service.AccountService
}

// NewAccountHandler creates a new AccountHandler
func NewAccountHandler(accountService *service.AccountService) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
	}
}

// CreateAccount handles POST /v1/accounts
func (h *AccountHandler) CreateAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req domain.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, errors.InvalidInput("invalid request body"))
		return
	}

	account, err := h.accountService.CreateAccount(c.Request.Context(), userID, &req)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, account.ToResponse())
}

// ListAccounts handles GET /v1/accounts
func (h *AccountHandler) ListAccounts(c *gin.Context) {
	userID := middleware.GetUserID(c)

	accounts, err := h.accountService.ListAccounts(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}

	response := &domain.ListAccountsResponse{
		Accounts: make([]*domain.AccountResponse, len(accounts)),
	}
	for i, account := range accounts {
		response.Accounts[i] = account.ToResponse()
	}

	c.JSON(http.StatusOK, response)
}

// GetAccount handles GET /v1/accounts/:accountNumber
func (h *AccountHandler) GetAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)
	accountNumber := c.Param("accountNumber")

	// Validate account number format
	if !domain.ValidateAccountNumber(accountNumber) {
		respondError(c, errors.InvalidInput("invalid account number format"))
		return
	}

	account, err := h.accountService.GetAccount(c.Request.Context(), userID, accountNumber)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, account.ToResponse())
}

// UpdateAccount handles PATCH /v1/accounts/:accountNumber
func (h *AccountHandler) UpdateAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)
	accountNumber := c.Param("accountNumber")

	// Validate account number format
	if !domain.ValidateAccountNumber(accountNumber) {
		respondError(c, errors.InvalidInput("invalid account number format"))
		return
	}

	var req domain.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, errors.InvalidInput("invalid request body"))
		return
	}

	account, err := h.accountService.UpdateAccount(c.Request.Context(), userID, accountNumber, &req)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, account.ToResponse())
}

// DeleteAccount handles DELETE /v1/accounts/:accountNumber
func (h *AccountHandler) DeleteAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)
	accountNumber := c.Param("accountNumber")

	// Validate account number format
	if !domain.ValidateAccountNumber(accountNumber) {
		respondError(c, errors.InvalidInput("invalid account number format"))
		return
	}

	if err := h.accountService.DeleteAccount(c.Request.Context(), userID, accountNumber); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
