package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/deltron-fr/solstice/backend/internal/blockchain"
	"github.com/deltron-fr/solstice/backend/internal/config"
	"github.com/deltron-fr/solstice/backend/internal/data"
)

type Server struct {
	logger  *slog.Logger
	db      *sql.DB
	queries *data.Queries
	chain   blockchain.Client
	config  config.Config
}

func New(logger *slog.Logger, db *sql.DB, chain blockchain.Client, cfg config.Config) *Server {
	return &Server{logger: logger, db: db, queries: data.New(db), chain: chain, config: cfg}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.Handle("POST /auth/challenge", s.checkOrigin(http.HandlerFunc(s.createChallenge)))
	mux.Handle("POST /auth/verify", s.checkOrigin(http.HandlerFunc(s.verifyChallenge)))
	mux.Handle("POST /auth/logout", s.checkOrigin(http.HandlerFunc(s.logout)))
	mux.Handle("GET /auth/me", s.protected(http.HandlerFunc(s.me)))
	mux.Handle("GET /users/{id}", s.protected(http.HandlerFunc(s.getUser)))
	mux.HandleFunc("GET /public/users/{id}", s.getPublicUser)
	mux.HandleFunc("GET /listings", s.listListings)
	mux.HandleFunc("GET /uploads/{filename}", s.getUploadedImage)
	mux.Handle("POST /uploads", s.writeProtected(http.HandlerFunc(s.uploadImage)))
	mux.Handle("GET /listings/mine", s.protected(http.HandlerFunc(s.listSellerListings)))
	mux.HandleFunc("GET /listings/{id}", s.getListing)
	mux.Handle("POST /listings", s.writeProtected(http.HandlerFunc(s.createListing)))
	mux.Handle("PATCH /listings/{id}", s.writeProtected(http.HandlerFunc(s.updateListing)))
	mux.Handle("POST /orders", s.writeProtected(http.HandlerFunc(s.createOrder)))
	mux.Handle("GET /orders", s.protected(http.HandlerFunc(s.listOrders)))
	mux.Handle("GET /orders/{id}", s.protected(http.HandlerFunc(s.getOrder)))
	mux.Handle("POST /orders/{id}/fulfill", s.writeProtected(http.HandlerFunc(s.fulfillOrder)))
	mux.Handle("POST /orders/{id}/cancel", s.writeProtected(http.HandlerFunc(s.cancelOrder)))
	mux.Handle("GET /orders/{id}/payment", s.protected(http.HandlerFunc(s.getPayment)))
	mux.Handle("POST /orders/{id}/payment/transactions", s.writeProtected(http.HandlerFunc(s.submitPaymentTransaction)))
	mux.Handle("POST /conversations", s.writeProtected(http.HandlerFunc(s.createConversation)))
	mux.Handle("GET /conversations", s.protected(http.HandlerFunc(s.listConversations)))
	mux.Handle("GET /conversations/{id}/messages", s.protected(http.HandlerFunc(s.listMessages)))
	mux.Handle("POST /conversations/{id}/messages", s.writeProtected(http.HandlerFunc(s.createMessage)))
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type contextKey string

const userContextKey contextKey = "authenticated-user"

func (s *Server) protected(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.config.CookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication_required", "sign in with your wallet to continue")
			return
		}
		token, err := decodeSessionToken(cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_session", "session is invalid or expired")
			return
		}
		hash := sha256.Sum256(token)
		user, err := s.queries.GetSessionUser(r.Context(), hash[:])
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid_session", "session is invalid or expired")
			return
		}
		if err != nil {
			s.internalError(w, "load session", err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func (s *Server) writeProtected(next http.Handler) http.Handler {
	return s.checkOrigin(s.protected(next))
}

func (s *Server) checkOrigin(next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(s.config.AllowedOrigins))
	for _, origin := range s.config.AllowedOrigins {
		allowed[strings.TrimRight(origin, "/")] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		if origin == "" {
			writeError(w, http.StatusForbidden, "origin_required", "a trusted browser origin is required")
			return
		}
		if !originAllowed(origin, allowed) {
			writeError(w, http.StatusForbidden, "origin_not_allowed", "request origin is not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originAllowed(origin string, allowed map[string]struct{}) bool {
	if _, ok := allowed[origin]; ok {
		return true
	}
	if _, wildcardEnabled := allowed["https://*.trycloudflare.com"]; !wildcardEnabled {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Opaque != "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.HasSuffix(host, ".trycloudflare.com") && len(host) > len(".trycloudflare.com")
}

func currentUser(r *http.Request) (data.User, bool) {
	user, ok := r.Context().Value(userContextKey).(data.User)
	return user, ok
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("request body must be valid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON value")
	}
	return nil
}

func (s *Server) internalError(w http.ResponseWriter, action string, err error) {
	s.logger.Error(action+" failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "the request could not be completed")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func positiveIntQuery(r *http.Request, key string, fallback, maximum int64) (int64, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 || parsed > maximum {
		return 0, fmt.Errorf("%s must be between 1 and %d", key, maximum)
	}
	return parsed, nil
}

func nonNegativeIntQuery(r *http.Request, key string, fallback int64) (int64, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return parsed, nil
}

func notFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }
