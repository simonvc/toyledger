package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/simonvc/miniledger/internal/ledger"
	"github.com/simonvc/miniledger/internal/store"
)

type createTransactionRequest struct {
	Description string `json:"description"`
	Entries     []struct {
		AccountID string `json:"account_id"`
		Amount    int64  `json:"amount"`
		Currency  string `json:"currency"`
	} `json:"entries"`
}

func (s *Server) createTransaction(w http.ResponseWriter, r *http.Request) {
	var req createTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	txn := &ledger.Transaction{
		Description: req.Description,
	}
	for _, e := range req.Entries {
		txn.Entries = append(txn.Entries, ledger.Entry{
			AccountID: e.AccountID,
			Amount:    e.Amount,
			Currency:  e.Currency,
		})
	}

	if err := s.store.CreateTransaction(r.Context(), txn); err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}

	// Fetch back the full transaction
	created, err := s.store.GetTransaction(r.Context(), txn.ID)
	if err != nil {
		writeJSON(w, http.StatusCreated, txn)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) {
	filter := store.TxnFilter{}
	if aid := r.URL.Query().Get("account_id"); aid != "" {
		filter.AccountID = aid
	}

	txns, err := s.store.ListTransactions(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if txns == nil {
		txns = []ledger.Transaction{}
	}
	writeJSON(w, http.StatusOK, txns)
}

func (s *Server) getTransaction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	txn, err := s.store.GetTransaction(r.Context(), id)
	if err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, txn)
}
