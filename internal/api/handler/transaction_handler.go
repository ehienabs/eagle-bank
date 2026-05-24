package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ehienabs/eagle-bank/internal/api/middleware"
	"github.com/ehienabs/eagle-bank/internal/domain"
	"github.com/ehienabs/eagle-bank/internal/service"
	"github.com/ehienabs/eagle-bank/pkg/errors"
)

// TransactionHandler handles transaction-related HTTP requests
type TransactionHandler struct {
	transactionService *service.TransactionService
}

// NewTransactionHandler creates a new TransactionHandler
func NewTransactionHandler(transactionService *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
	}
}

// CreateTransaction handles POST /v1/accounts/:accountNumber/transactions
func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	userID := middleware.GetUserID(c)
	accountNumber := c.Param("accountNumber")

	// Validate account number format
	if !domain.ValidateAccountNumber(accountNumber) {
		respondError(c, errors.InvalidInput("invalid account number format"))
		return
	}

	var req domain.CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, errors.InvalidInput("invalid request body"))
		return
	}

	transaction, err := h.transactionService.CreateTransaction(c.Request.Context(), userID, accountNumber, &req)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, transaction.ToResponse())
}

// ListTransactions handles GET /v1/accounts/:accountNumber/transactions
func (h *TransactionHandler) ListTransactions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	accountNumber := c.Param("accountNumber")

	// Validate account number format
	if !domain.ValidateAccountNumber(accountNumber) {
		respondError(c, errors.InvalidInput("invalid account number format"))
		return
	}

	transactions, err := h.transactionService.ListTransactions(c.Request.Context(), userID, accountNumber)
	if err != nil {
		respondError(c, err)
		return
	}

	response := &domain.ListTransactionsResponse{
		Transactions: make([]*domain.TransactionResponse, len(transactions)),
	}
	for i, txn := range transactions {
		response.Transactions[i] = txn.ToResponse()
	}

	c.JSON(http.StatusOK, response)
}

// GetTransaction handles GET /v1/accounts/:accountNumber/transactions/:transactionId
func (h *TransactionHandler) GetTransaction(c *gin.Context) {
	userID := middleware.GetUserID(c)
	accountNumber := c.Param("accountNumber")
	transactionID := c.Param("transactionId")

	// Validate account number format
	if !domain.ValidateAccountNumber(accountNumber) {
		respondError(c, errors.InvalidInput("invalid account number format"))
		return
	}

	// Validate transaction ID format
	if !domain.ValidateTransactionID(transactionID) {
		respondError(c, errors.InvalidInput("invalid transaction ID format"))
		return
	}

	transaction, err := h.transactionService.GetTransaction(c.Request.Context(), userID, accountNumber, transactionID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, transaction.ToResponse())
}
