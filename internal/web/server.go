package web

import (
	_ "embed"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed static/index.html
var indexHTML []byte

// Server serves the web terminal UI.
type Server struct {
	addr    string
	apiAddr string
	dbPath  string
	router  chi.Router
}

// NewServer creates a web terminal server.
// apiAddr is the embedded API server address (e.g. "http://127.0.0.1:8888").
// dbPath is passed through to TUI subprocesses.
func NewServer(addr, apiAddr, dbPath string) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	s := &Server{
		addr:    addr,
		apiAddr: apiAddr,
		dbPath:  dbPath,
		router:  r,
	}

	r.Get("/", s.handleIndex)
	r.Get("/ws", s.handleWebSocket)

	return s
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

// ListenAndServe starts the web terminal server.
func (s *Server) ListenAndServe() error {
	log.Printf("web terminal listening on %s", s.addr)
	return http.ListenAndServe(s.addr, s.router)
}
