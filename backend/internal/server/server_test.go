package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pact-marketplace/pact/backend/internal/app"
	"github.com/pact-marketplace/pact/backend/internal/data"
)

func TestHealth(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	r := httptest.NewRecorder()
	s.Routes().ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/health", nil))
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health response: %d %s", r.Code, r.Body.String())
	}
}

func TestScaffoldRouteIsExplicitlyNotImplemented(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	r := httptest.NewRecorder()
	s.Routes().ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{}`)))
	if r.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", r.Code, http.StatusNotImplemented)
	}
}

func TestCreateUserAndListing(t *testing.T) {
	db, err := app.OpenDatabase("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), data.New(db), nil)

	userResponse := httptest.NewRecorder()
	s.Routes().ServeHTTP(userResponse, httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(`{"name":"Ada Business","email":"ada@example.test"}`)))
	if userResponse.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, body %s", userResponse.Code, userResponse.Body.String())
	}
	var user struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(userResponse.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}

	listingRequest := map[string]any{"seller_id": user.ID, "title": "Coffee beans", "description": "Freshly roasted", "price": 12500, "currency": "NGN", "inventory_quantity": 8}
	body, err := json.Marshal(listingRequest)
	if err != nil {
		t.Fatal(err)
	}
	listingResponse := httptest.NewRecorder()
	s.Routes().ServeHTTP(listingResponse, httptest.NewRequest(http.MethodPost, "/listings", bytes.NewReader(body)))
	if listingResponse.Code != http.StatusCreated {
		t.Fatalf("create listing status = %d, body %s", listingResponse.Code, listingResponse.Body.String())
	}
}
