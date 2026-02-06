package server

import (
	"net/http"

	"github.com/simonvc/miniledger/internal/ledger"
)

func (s *Server) balanceSheet(w http.ResponseWriter, r *http.Request) {
	bs, err := s.store.BalanceSheet(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bs)
}

func (s *Server) trialBalance(w http.ResponseWriter, r *http.Request) {
	tb, err := s.store.TrialBalance(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tb)
}

func (s *Server) getChart(w http.ResponseWriter, r *http.Request) {
	all := make([]ledger.ChartEntry, 0, len(ledger.PredefinedAccounts)+len(ledger.SystemAccounts))
	all = append(all, ledger.PredefinedAccounts...)
	all = append(all, ledger.SystemAccounts...)
	writeJSON(w, http.StatusOK, all)
}
