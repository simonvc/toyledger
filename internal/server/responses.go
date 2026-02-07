package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/simonvc/miniledger/internal/ledger"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func mapError(err error) int {
	switch {
	case errors.Is(err, ledger.ErrAccountNotFound), errors.Is(err, ledger.ErrTransactionNotFound):
		return http.StatusNotFound
	case errors.Is(err, ledger.ErrDuplicateAccount):
		return http.StatusConflict
	case errors.Is(err, ledger.ErrUnbalancedTransaction),
		errors.Is(err, ledger.ErrTooFewEntries),
		errors.Is(err, ledger.ErrEmptyDescription),
		errors.Is(err, ledger.ErrInvalidAccountCode),
		errors.Is(err, ledger.ErrInvalidAccountID),
		errors.Is(err, ledger.ErrInvalidCategory),
		errors.Is(err, ledger.ErrCodeCategoryMismatch),
		errors.Is(err, ledger.ErrInvalidCurrency),
		errors.Is(err, ledger.ErrCurrencyMismatch),
		errors.Is(err, ledger.ErrSystemAccountPrefix),
		errors.Is(err, ledger.ErrNonSystemAccountTilde):
		return http.StatusBadRequest
	case errors.Is(err, ledger.ErrInvertedBalance),
		errors.Is(err, ledger.ErrEntryDirectionViolation):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}
