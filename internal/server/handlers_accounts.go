package server

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/simonvc/miniledger/internal/ledger"
	"github.com/simonvc/miniledger/internal/store"
)

type createAccountRequest struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Code     int             `json:"code"`
	Currency string          `json:"currency"`
	Category ledger.Category `json:"category,omitempty"`
	IsSystem bool            `json:"is_system,omitempty"`
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Currency == "" {
		req.Currency = "USD"
	}

	// Auto-derive category from code if not provided
	if req.Category == "" && !req.IsSystem {
		cat, err := ledger.CategoryForCode(req.Code)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.Category = cat
	}

	acct := &ledger.Account{
		ID:       req.ID,
		Name:     req.Name,
		Code:     req.Code,
		Category: req.Category,
		Currency: req.Currency,
		IsSystem: req.IsSystem,
	}

	if err := s.store.CreateAccount(r.Context(), acct); err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}

	// Fetch back to get created_at
	created, err := s.store.GetAccount(r.Context(), acct.ID)
	if err != nil {
		writeJSON(w, http.StatusCreated, acct)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	filter := store.AccountFilter{}

	if cat := r.URL.Query().Get("category"); cat != "" {
		filter.Category = ledger.Category(cat)
	}
	if sys := r.URL.Query().Get("system"); sys != "" {
		v := sys == "true" || sys == "1"
		filter.IsSystem = &v
	}

	accounts, err := s.store.ListAccounts(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if accounts == nil {
		accounts = []ledger.Account{}
	}
	writeJSON(w, http.StatusOK, accounts)
}

func (s *Server) getAccount(w http.ResponseWriter, r *http.Request) {
	id, _ := url.PathUnescape(chi.URLParam(r, "id"))
	acct, err := s.store.GetAccount(r.Context(), id)
	if err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, acct)
}

func (s *Server) getAccountBalance(w http.ResponseWriter, r *http.Request) {
	id, _ := url.PathUnescape(chi.URLParam(r, "id"))
	balance, currency, err := s.store.AccountBalance(r.Context(), id)
	if err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"account_id": id,
		"balance":    balance,
		"currency":   currency,
		"formatted":  ledger.FormatAmount(balance, currency),
	})
}

func (s *Server) renameAccount(w http.ResponseWriter, r *http.Request) {
	id, _ := url.PathUnescape(chi.URLParam(r, "id"))
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := s.store.RenameAccount(r.Context(), id, req.Name); err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}
	acct, err := s.store.GetAccount(r.Context(), id)
	if err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, acct)
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	id, _ := url.PathUnescape(chi.URLParam(r, "id"))
	if err := s.store.DeleteAccount(r.Context(), id); err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listAccountEntries(w http.ResponseWriter, r *http.Request) {
	id, _ := url.PathUnescape(chi.URLParam(r, "id"))

	entries, err := s.store.ListEntriesByAccount(r.Context(), id, store.EntryFilter{Limit: 100})
	if err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}
	if entries == nil {
		entries = []ledger.Entry{}
	}
	writeJSON(w, http.StatusOK, entries)
}
