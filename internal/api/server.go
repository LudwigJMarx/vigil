// Package api serves vigil's HTTP interface: the JSON API the browser
// extension talks to, and the small operator UI.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/LudwigJMarx/vigil/internal/store"
)

// Options configures a Server.
type Options struct {
	Store *store.Store
	Log   *slog.Logger
	// Version is reported by /healthz so an operator can tell which binary is
	// answering without reading the process list.
	Version string
	// Now is injected so tests can score against a fixed instant. Nil means
	// time.Now.
	Now func() time.Time
}

// Server is the HTTP handler for every vigil route.
type Server struct {
	store   *store.Store
	log     *slog.Logger
	version string
	now     func() time.Time
	mux     *http.ServeMux
}

// New wires the routes. The returned Server is an http.Handler.
func New(opts Options) *Server {
	if opts.Log == nil {
		opts.Log = slog.Default()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	s := &Server{
		store:   opts.Store,
		log:     opts.Log,
		version: opts.Version,
		now:     opts.Now,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// Open: it answers whether the process and its database are alive, which
	// is the one thing a monitor needs before it holds a credential.
	s.mux.HandleFunc("GET /healthz", s.handleHealth)

	// Everything below requires a token.
	guarded := map[string]http.HandlerFunc{
		"POST /api/v1/ingest":             s.handleIngest,
		"GET /api/v1/accounts":            s.handleListAccounts,
		"POST /api/v1/accounts":           s.handleCreateAccount,
		"GET /api/v1/accounts/{id}":       s.handleAccount,
		"DELETE /api/v1/accounts/{id}":    s.handleDeleteAccount,
		"POST /api/v1/accounts/{id}/note": s.handleAddNote,
		"GET /api/v1/rules":               s.handleListRules,
		"PUT /api/v1/rules/{kind}":        s.handlePutRule,
		"DELETE /api/v1/rules/{kind}":     s.handleDeleteRule,
		"GET /api/v1/tokens":              s.handleListTokens,
		"POST /api/v1/tokens":             s.handleCreateToken,
		"DELETE /api/v1/tokens/{id}":      s.handleRevokeToken,
	}
	for pattern, handler := range guarded {
		s.mux.Handle(pattern, s.requireToken(handler))
	}

	s.mux.Handle("GET /", uiHandler())
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.cors(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// cors lets the browser extension call the API from its own origin.
//
// A wildcard origin is safe here and only here: vigil authenticates with an
// Authorization header, never with a cookie, so a page that has not been given
// a token gets a 401 no matter which origin it calls from. The wildcard would
// be a hole the moment a cookie session were added, which is why there is a
// test that fails if one appears.
func (s *Server) cors(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") == "" {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Max-Age", "600")
}

type contextKey string

const tokenKey contextKey = "vigil.token"

// requireToken rejects a request that carries no usable credential. Every
// failure is the same 401 with the same body: which of "unknown", "revoked" or
// "malformed" applies is exactly the hint a guesser wants.
func (s *Server) requireToken(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := bearer(r)
		if secret == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="vigil"`)
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		token, err := s.store.Authenticate(r.Context(), secret)
		if err != nil {
			if !errors.Is(err, store.ErrBadToken) {
				s.log.Error("authenticate", "error", err)
				writeError(w, http.StatusInternalServerError, "authentication failed")
				return
			}
			writeError(w, http.StatusUnauthorized, "token is not valid")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), tokenKey, token)))
	})
}

func bearer(r *http.Request) string {
	header := r.Header.Get("Authorization")
	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "bearer") {
		return ""
	}
	return strings.TrimSpace(value)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// The status line is already gone; there is nothing left to tell the
		// client. Logging is the only honest option.
		slog.Default().Error("write response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body: "+err.Error())
		return false
	}
	return true
}
