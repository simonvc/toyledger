package web

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

//go:embed static/index.html
var indexHTML []byte

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Server serves the web terminal UI.
type Server struct {
	addr      string
	ledgerDir string
	router    chi.Router
}

// NewServer creates a web terminal server.
// ledgerDir is the directory where per-session SQLite databases are stored.
func NewServer(addr, ledgerDir string) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	s := &Server{
		addr:      addr,
		ledgerDir: ledgerDir,
		router:    r,
	}

	r.Get("/", s.handleIndex)
	r.Get("/ws", s.handleWebSocket)
	r.Post("/reset", s.handleReset)
	r.Get("/join/{id}", s.handleJoin)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	return s
}

const cookieName = "miniledger_session"

// sessionID reads or creates a session UUID cookie and returns the session ID
// and the corresponding database path.
func (s *Server) sessionID(w http.ResponseWriter, r *http.Request) (string, string) {
	if c, err := r.Cookie(cookieName); err == nil && uuidRe.MatchString(c.Value) {
		dbPath := filepath.Join(s.ledgerDir, c.Value+".db")
		return c.Value, dbPath
	}

	id := uuid.New().String()
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    id,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		SameSite: http.SameSiteLaxMode,
	})
	dbPath := filepath.Join(s.ledgerDir, id+".db")
	return id, dbPath
}

// readSessionDBPath reads the session cookie without writing headers.
// Returns the DB path or an error if no valid session exists.
func (s *Server) readSessionDBPath(r *http.Request) (string, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", fmt.Errorf("no session cookie")
	}
	if !uuidRe.MatchString(c.Value) {
		return "", fmt.Errorf("invalid session cookie")
	}
	return filepath.Join(s.ledgerDir, c.Value+".db"), nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.sessionID(w, r) // ensure cookie is set
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(cookieName)
	if err != nil || !uuidRe.MatchString(c.Value) {
		http.Error(w, "no valid session", http.StatusBadRequest)
		return
	}

	base := filepath.Join(s.ledgerDir, c.Value+".db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		os.Remove(base + suffix)
	}

	// Clear the cookie so the next page load gets a fresh session.
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("reset"))
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !uuidRe.MatchString(id) {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	dbPath := filepath.Join(s.ledgerDir, id+".db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		http.Error(w, "ledger not found", http.StatusNotFound)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    id,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// ListenAndServe starts the web terminal server.
func (s *Server) ListenAndServe() error {
	log.Printf("web terminal listening on %s", s.addr)
	return http.ListenAndServe(s.addr, s.router)
}
