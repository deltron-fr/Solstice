package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/pact-marketplace/pact/backend/internal/blockchain"
	"github.com/pact-marketplace/pact/backend/internal/data"
)

type Server struct {
	logger  *slog.Logger
	queries *data.Queries
	chain   blockchain.Client
}

func New(logger *slog.Logger, queries *data.Queries, chain blockchain.Client) *Server {
	return &Server{logger: logger, queries: queries, chain: chain}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /users", s.createUser)
	mux.HandleFunc("GET /users/{id}", s.getUser)
	mux.HandleFunc("POST /listings", s.createListing)
	mux.HandleFunc("GET /listings", s.listListings)
	mux.HandleFunc("GET /listings/{id}", s.getListing)
	mux.HandleFunc("PATCH /listings/{id}", s.notImplemented)
	mux.HandleFunc("POST /orders", s.notImplemented)
	mux.HandleFunc("GET /orders/{id}", s.notImplemented)
	mux.HandleFunc("GET /orders", s.notImplemented)
	mux.HandleFunc("GET /orders/{id}/payment", s.notImplemented)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) notImplemented(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "endpoint_not_implemented", "This API operation is not implemented yet")
}

type createUserRequest struct {
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	WalletAddress *string `json:"wallet_address"`
}

type userResponse struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	WalletAddress *string `json:"wallet_address"`
	CreatedAt     string  `json:"created_at"`
}

func presentUser(user data.User) userResponse {
	var walletAddress *string
	if user.WalletAddress.Valid {
		walletAddress = &user.WalletAddress.String
	}
	return userResponse{ID: user.ID, Name: user.Name, Email: user.Email, WalletAddress: walletAddress, CreatedAt: user.CreatedAt}
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var input createUserRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(input.Email)
	if input.Name == "" || input.Email == "" || !strings.Contains(input.Email, "@") {
		writeError(w, http.StatusBadRequest, "validation_error", "name and a valid email are required")
		return
	}
	var wallet sql.NullString
	if input.WalletAddress != nil && strings.TrimSpace(*input.WalletAddress) != "" {
		wallet = sql.NullString{String: strings.TrimSpace(*input.WalletAddress), Valid: true}
	}
	user, err := s.queries.CreateUser(r.Context(), data.CreateUserParams{Name: input.Name, Email: input.Email, WalletAddress: wallet})
	if err != nil {
		s.logger.Error("create user failed", "error", err)
		writeError(w, http.StatusConflict, "user_conflict", "A user with that email or wallet already exists")
		return
	}
	writeJSON(w, http.StatusCreated, presentUser(user))
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "user id must be a positive integer")
		return
	}
	user, err := s.queries.GetUser(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err != nil {
		s.internalError(w, "get user", err)
		return
	}
	writeJSON(w, http.StatusOK, presentUser(user))
}

type createListingRequest struct {
	SellerID          int64  `json:"seller_id"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	Price             int64  `json:"price"`
	Currency          string `json:"currency"`
	InventoryQuantity int64  `json:"inventory_quantity"`
}

func (s *Server) createListing(w http.ResponseWriter, r *http.Request) {
	var input createListingRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.SellerID < 1 || input.Title == "" || input.Price < 1 || input.InventoryQuantity < 0 || !validCurrency(input.Currency) {
		writeError(w, http.StatusBadRequest, "validation_error", "seller_id, title, positive price, non-negative inventory, and currency (NGN, USD, or USDC) are required")
		return
	}
	listing, err := s.queries.CreateListing(r.Context(), data.CreateListingParams{
		SellerID: input.SellerID, Title: input.Title, Description: input.Description,
		Price: input.Price, Currency: input.Currency, InventoryQuantity: input.InventoryQuantity,
	})
	if err != nil {
		s.logger.Error("create listing failed", "error", err)
		writeError(w, http.StatusBadRequest, "listing_not_created", "listing could not be created; check the seller and listing values")
		return
	}
	writeJSON(w, http.StatusCreated, listing)
}

func (s *Server) listListings(w http.ResponseWriter, r *http.Request) {
	limit, offset := int64(20), int64(0)
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	if value := r.URL.Query().Get("offset"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid_offset", "offset must be a non-negative integer")
			return
		}
		offset = parsed
	}
	listings, err := s.queries.ListActiveListings(r.Context(), data.ListActiveListingsParams{Limit: limit, Offset: offset})
	if err != nil {
		s.internalError(w, "list listings", err)
		return
	}
	if listings == nil {
		listings = []data.Listing{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"listings": listings, "limit": limit, "offset": offset})
}

func (s *Server) getListing(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "listing id must be a positive integer")
		return
	}
	listing, err := s.queries.GetListing(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		s.internalError(w, "get listing", err)
		return
	}
	writeJSON(w, http.StatusOK, listing)
}

func (s *Server) internalError(w http.ResponseWriter, action string, err error) {
	s.logger.Error(action+" failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "the request could not be completed")
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

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func validCurrency(value string) bool { return value == "NGN" || value == "USD" || value == "USDC" }

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
