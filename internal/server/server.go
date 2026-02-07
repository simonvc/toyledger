package server

import (
	"log"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/simonvc/miniledger/internal/store"
)

type Server struct {
	store  *store.Store
	router chi.Router
	addr   string
}

func New(st *store.Store, addr string) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	s := &Server{store: st, router: r, addr: addr}

	r.Route("/api/v1", func(r chi.Router) {
		// Accounts
		r.Post("/accounts", s.createAccount)
		r.Get("/accounts", s.listAccounts)
		r.Get("/accounts/{id}", s.getAccount)
		r.Get("/accounts/{id}/balance", s.getAccountBalance)
		r.Get("/accounts/{id}/entries", s.listAccountEntries)
		r.Patch("/accounts/{id}", s.renameAccount)
		r.Delete("/accounts/{id}", s.deleteAccount)

		// Transactions
		r.Post("/transactions", s.createTransaction)
		r.Get("/transactions", s.listTransactions)
		r.Get("/transactions/{id}", s.getTransaction)

		// Reports
		r.Get("/reports/balance-sheet", s.balanceSheet)
		r.Get("/reports/trial-balance", s.trialBalance)
		r.Get("/reports/ratios", s.regulatoryRatios)

		// Chart of accounts reference
		r.Get("/chart", s.getChart)

		// CoA code settings
		r.Get("/settings", s.listSettings)
		r.Get("/settings/{code}", s.getCodeSettings)
		r.Put("/settings/{code}/{setting}", s.upsertSetting)
		r.Delete("/settings/{code}/{setting}", s.deleteSetting)
	})

	return s
}

func (s *Server) ListenAndServe() error {
	log.Printf("miniledger server listening on %s", s.addr)
	return http.ListenAndServe(s.addr, s.router)
}

func (s *Server) Serve(ln net.Listener) error {
	log.Printf("miniledger server listening on %s", ln.Addr())
	return http.Serve(ln, s.router)
}

func (s *Server) Handler() http.Handler {
	return s.router
}
