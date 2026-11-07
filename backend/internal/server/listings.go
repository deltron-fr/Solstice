package server

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/pact-marketplace/pact/backend/internal/data"
)

type createListingRequest struct {
	Title             string `json:"title"`
	Description       string `json:"description"`
	Price             int64  `json:"price"`
	Currency          string `json:"currency"`
	Category          string `json:"category"`
	Unit              string `json:"unit"`
	Location          string `json:"location"`
	ImageURL          string `json:"image_url"`
	InventoryQuantity int64  `json:"inventory_quantity"`
}

func (s *Server) createListing(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "sign in with your wallet to continue")
		return
	}
	var input createListingRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Description = strings.TrimSpace(input.Description)
	input.Category = strings.TrimSpace(input.Category)
	input.Unit = strings.TrimSpace(input.Unit)
	input.Location = strings.TrimSpace(input.Location)
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	if input.Category == "" {
		input.Category = "General"
	}
	if input.Unit == "" {
		input.Unit = "item"
	}
	if !validListingFields(input.Title, input.Description, input.Price, input.Currency, input.Category, input.Unit, input.Location, input.ImageURL, input.InventoryQuantity) {
		writeError(w, http.StatusBadRequest, "validation_error", "listing fields are invalid; image_url must be an HTTPS URL")
		return
	}
	status := "active"
	if input.InventoryQuantity == 0 {
		status = "sold_out"
	}
	listing, err := s.queries.CreateListing(r.Context(), data.CreateListingParams{
		SellerID: user.ID, Title: input.Title, Description: input.Description,
		Price: input.Price, Currency: input.Currency, Category: input.Category, Unit: input.Unit,
		Location: input.Location, ImageUrl: input.ImageURL, InventoryQuantity: input.InventoryQuantity, Status: status,
	})
	if err != nil {
		s.internalError(w, "create listing", err)
		return
	}
	writeJSON(w, http.StatusCreated, listing)
}

func (s *Server) listListings(w http.ResponseWriter, r *http.Request) {
	limit, err := positiveIntQuery(r, "limit", 20, 100)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_limit", err.Error())
		return
	}
	offset, err := nonNegativeIntQuery(r, "offset", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_offset", err.Error())
		return
	}
	listings, err := s.queries.ListActiveListings(r.Context(), data.ListActiveListingsParams{Limit: limit, Offset: offset})
	if err != nil {
		s.internalError(w, "list active listings", err)
		return
	}
	if listings == nil {
		listings = []data.Listing{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"listings": listings, "limit": limit, "offset": offset})
}

func (s *Server) listSellerListings(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	limit, err := positiveIntQuery(r, "limit", 20, 100)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_limit", err.Error())
		return
	}
	offset, err := nonNegativeIntQuery(r, "offset", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_offset", err.Error())
		return
	}
	listings, err := s.queries.ListListingsForSeller(r.Context(), data.ListListingsForSellerParams{SellerID: user.ID, Limit: limit, Offset: offset})
	if err != nil {
		s.internalError(w, "list seller listings", err)
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
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
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

type patchListingRequest struct {
	Title             *string `json:"title"`
	Description       *string `json:"description"`
	Price             *int64  `json:"price"`
	Currency          *string `json:"currency"`
	Category          *string `json:"category"`
	Unit              *string `json:"unit"`
	Location          *string `json:"location"`
	ImageURL          *string `json:"image_url"`
	InventoryQuantity *int64  `json:"inventory_quantity"`
	Status            *string `json:"status"`
}

func (s *Server) updateListing(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	listing, err := s.queries.GetListingForUpdate(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		s.internalError(w, "load listing for update", err)
		return
	}
	if listing.SellerID != user.ID {
		writeError(w, http.StatusForbidden, "not_listing_owner", "only the seller may update this listing")
		return
	}
	var input patchListingRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if input.Title == nil && input.Description == nil && input.Price == nil && input.Currency == nil && input.Category == nil && input.Unit == nil && input.Location == nil && input.ImageURL == nil && input.InventoryQuantity == nil && input.Status == nil {
		writeError(w, http.StatusBadRequest, "empty_patch", "provide at least one listing field to update")
		return
	}
	if input.Title != nil {
		listing.Title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		listing.Description = strings.TrimSpace(*input.Description)
	}
	if input.Price != nil {
		listing.Price = *input.Price
	}
	if input.Currency != nil {
		listing.Currency = strings.ToUpper(strings.TrimSpace(*input.Currency))
	}
	if input.Category != nil {
		listing.Category = strings.TrimSpace(*input.Category)
	}
	if input.Unit != nil {
		listing.Unit = strings.TrimSpace(*input.Unit)
	}
	if input.Location != nil {
		listing.Location = strings.TrimSpace(*input.Location)
	}
	if input.ImageURL != nil {
		listing.ImageUrl = strings.TrimSpace(*input.ImageURL)
	}
	if input.InventoryQuantity != nil {
		listing.InventoryQuantity = *input.InventoryQuantity
	}
	if input.Status != nil {
		listing.Status = strings.TrimSpace(*input.Status)
	}
	if !validListingFields(listing.Title, listing.Description, listing.Price, listing.Currency, listing.Category, listing.Unit, listing.Location, listing.ImageUrl, listing.InventoryQuantity) || !validListingStatus(listing.Status) {
		writeError(w, http.StatusBadRequest, "validation_error", "listing title, price, currency, inventory, or status is invalid")
		return
	}
	if listing.InventoryQuantity == 0 && listing.Status == "active" {
		listing.Status = "sold_out"
	}
	if listing.InventoryQuantity > 0 && listing.Status == "sold_out" {
		listing.Status = "active"
	}
	updated, err := s.queries.UpdateListing(r.Context(), data.UpdateListingParams{
		Title: listing.Title, Description: listing.Description, Price: listing.Price, Currency: listing.Currency,
		Category: listing.Category, Unit: listing.Unit, Location: listing.Location, ImageUrl: listing.ImageUrl,
		InventoryQuantity: listing.InventoryQuantity, Status: listing.Status, ID: listing.ID, SellerID: user.ID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		s.internalError(w, "update listing", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func validCurrency(value string) bool { return value == "NGN" || value == "USD" || value == "USDC" }

func validListingFields(title, description string, price int64, currency, category, unit, location, imageURL string, inventory int64) bool {
	if title == "" || len([]rune(title)) > 200 || len([]rune(description)) > 20_000 || price <= 0 ||
		!validCurrency(currency) || category == "" || len([]rune(category)) > 80 || unit == "" ||
		len([]rune(unit)) > 60 || len([]rune(location)) > 160 || inventory < 0 {
		return false
	}
	if imageURL == "" {
		return true
	}
	parsed, err := url.ParseRequestURI(imageURL)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

type publicUserResponse struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	WalletAddress *string `json:"wallet_address"`
}

func (s *Server) getPublicUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	user, err := s.queries.GetPublicUser(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "seller not found")
		return
	}
	if err != nil {
		s.internalError(w, "get public seller profile", err)
		return
	}
	var walletAddress *string
	if user.WalletAddress.Valid {
		walletAddress = &user.WalletAddress.String
	}
	writeJSON(w, http.StatusOK, publicUserResponse{ID: user.ID, Name: user.Name, WalletAddress: walletAddress})
}

func validListingStatus(value string) bool {
	switch value {
	case "draft", "active", "paused", "sold_out", "archived":
		return true
	default:
		return false
	}
}
