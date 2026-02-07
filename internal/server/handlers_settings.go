package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/simonvc/miniledger/internal/ledger"
)

func (s *Server) listSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.ListAllSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if settings == nil {
		settings = []ledger.CoASetting{}
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) getCodeSettings(w http.ResponseWriter, r *http.Request) {
	codeStr := chi.URLParam(r, "code")
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid code: "+codeStr)
		return
	}

	cs, err := s.store.GetCodeSettings(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

type upsertSettingRequest struct {
	Value string `json:"value"`
}

func (s *Server) upsertSetting(w http.ResponseWriter, r *http.Request) {
	codeStr := chi.URLParam(r, "code")
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid code: "+codeStr)
		return
	}

	setting := ledger.SettingName(chi.URLParam(r, "setting"))
	if setting != ledger.SettingBlockInverted && setting != ledger.SettingEntryDirection {
		writeError(w, http.StatusBadRequest, "unknown setting: "+string(setting))
		return
	}

	var req upsertSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Validate value
	switch setting {
	case ledger.SettingBlockInverted:
		if req.Value != "0" && req.Value != "1" {
			writeError(w, http.StatusBadRequest, "BLOCK_NORMAL_INVERTED value must be '0' or '1'")
			return
		}
	case ledger.SettingEntryDirection:
		switch ledger.EntryDirection(req.Value) {
		case ledger.DirectionBoth, ledger.DirectionDebitOnly, ledger.DirectionCreditOnly:
		default:
			writeError(w, http.StatusBadRequest, "ENTRY_DIRECTION must be BOTH, DEBIT_ONLY, or CREDIT_ONLY")
			return
		}
	}

	cs := ledger.CoASetting{Code: code, Setting: setting, Value: req.Value}
	if err := s.store.UpsertSetting(r.Context(), cs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

func (s *Server) deleteSetting(w http.ResponseWriter, r *http.Request) {
	codeStr := chi.URLParam(r, "code")
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid code: "+codeStr)
		return
	}

	setting := ledger.SettingName(chi.URLParam(r, "setting"))
	if setting != ledger.SettingBlockInverted && setting != ledger.SettingEntryDirection {
		writeError(w, http.StatusBadRequest, "unknown setting: "+string(setting))
		return
	}

	if err := s.store.DeleteSetting(r.Context(), code, setting); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
