package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pact-marketplace/pact/backend/internal/app"
	"github.com/pact-marketplace/pact/backend/internal/blockchain"
	"github.com/pact-marketplace/pact/backend/internal/config"
	solana "github.com/solana-foundation/solana-go/v2"
)

const testUSDCMint = "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"

type fakeChain struct {
	verification blockchain.Verification
	lastExpected blockchain.Expectation
	lastSig      string
	escrowState  blockchain.EscrowState
	stateErr     error
	deriveErr    error
	verifyErr    error
}

func (f *fakeChain) DeriveEscrowAddress([32]byte) (string, error) {
	if f.deriveErr != nil {
		return "", f.deriveErr
	}
	return "test-escrow-pda", nil
}

func (f *fakeChain) VerifyTransaction(_ context.Context, signature string, expected blockchain.Expectation) (blockchain.Verification, error) {
	f.lastSig, f.lastExpected = signature, expected
	return f.verification, f.verifyErr
}

func (f *fakeChain) EscrowState(context.Context, string) (blockchain.EscrowState, error) {
	return f.escrowState, f.stateErr
}

func newTestServer(t *testing.T, chain *fakeChain) (*Server, *sql.DB) {
	t.Helper()
	db, err := app.OpenDatabase("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cfg := config.Config{
		CookieName: "pact_session", SessionTTL: time.Hour, ChallengeTTL: time.Minute,
		AllowedOrigins: []string{"http://localhost:5173"}, USDCMint: testUSDCMint,
	}
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)), db, chain, cfg), db
}

func TestHealth(t *testing.T) {
	s, _ := newTestServer(t, nil)
	r := httptest.NewRecorder()
	s.Routes().ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/health", nil))
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health response: %d %s", r.Code, r.Body.String())
	}
}

func TestWalletAuthenticationAndReplayProtection(t *testing.T) {
	s, _ := newTestServer(t, nil)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wallet := solana.PublicKeyFromBytes(public).String()
	challengeRequest := httptest.NewRequest(http.MethodPost, "/auth/challenge", strings.NewReader(`{"wallet_address":"`+wallet+`"}`))
	challengeRequest.Header.Set("Origin", "http://localhost:5173")
	challengeRecorder := httptest.NewRecorder()
	s.Routes().ServeHTTP(challengeRecorder, challengeRequest)
	if challengeRecorder.Code != http.StatusCreated {
		t.Fatalf("challenge status %d: %s", challengeRecorder.Code, challengeRecorder.Body.String())
	}
	var challenge challengeResponse
	if err := json.Unmarshal(challengeRecorder.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	signature := solana.SignatureFromBytes(ed25519.Sign(private, []byte(challenge.Message))).String()
	verifyBody := `{"wallet_address":"` + wallet + `","nonce":"` + challenge.Nonce + `","signature":"` + signature + `","name":"Ada Business","email":"ada@example.test"}`
	verifyRequest := httptest.NewRequest(http.MethodPost, "/auth/verify", strings.NewReader(verifyBody))
	verifyRequest.Header.Set("Origin", "http://localhost:5173")
	verifyResponse := httptest.NewRecorder()
	s.Routes().ServeHTTP(verifyResponse, verifyRequest)
	if verifyResponse.Code != http.StatusOK || len(verifyResponse.Result().Cookies()) != 1 {
		t.Fatalf("verify status %d: %s", verifyResponse.Code, verifyResponse.Body.String())
	}
	cookie := verifyResponse.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.Name != "pact_session" {
		t.Fatalf("unexpected session cookie: %#v", cookie)
	}
	protected := httptest.NewRequest(http.MethodPost, "/listings", strings.NewReader(`{"title":"Beans","price":1000000,"currency":"USDC","inventory_quantity":2}`))
	protected.Header.Set("Origin", "http://localhost:5173")
	protected.AddCookie(cookie)
	protectedResponse := httptest.NewRecorder()
	s.Routes().ServeHTTP(protectedResponse, protected)
	if protectedResponse.Code != http.StatusCreated {
		t.Fatalf("authenticated listing status %d: %s", protectedResponse.Code, protectedResponse.Body.String())
	}
	replay := httptest.NewRequest(http.MethodPost, "/auth/verify", strings.NewReader(verifyBody))
	replay.Header.Set("Origin", "http://localhost:5173")
	replayResponse := httptest.NewRecorder()
	s.Routes().ServeHTTP(replayResponse, replay)
	if replayResponse.Code != http.StatusUnauthorized {
		t.Fatalf("replayed challenge status = %d, want %d", replayResponse.Code, http.StatusUnauthorized)
	}
}

func TestOrderReservesInventoryAndVerifiesEscrowTransitions(t *testing.T) {
	chain := &fakeChain{}
	s, db := newTestServer(t, chain)
	sellerCookie := createUserSession(t, s, "Seller", "seller@example.test")
	buyerCookie := createUserSession(t, s, "Buyer", "buyer@example.test")
	listingResponse := serve(s, http.MethodPost, "/listings", `{"title":"Roasted beans","description":"Fresh","price":2500000,"currency":"USDC","inventory_quantity":3}`, sellerCookie)
	if listingResponse.Code != http.StatusCreated {
		t.Fatalf("create listing status %d: %s", listingResponse.Code, listingResponse.Body.String())
	}
	var listing struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(listingResponse.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	orderRecorder := serve(s, http.MethodPost, "/orders", `{"listing_id":`+itoa(listing.ID)+`,"quantity":2}`, buyerCookie)
	if orderRecorder.Code != http.StatusCreated {
		t.Fatalf("create order status %d: %s", orderRecorder.Code, orderRecorder.Body.String())
	}
	var order orderResponse
	if err := json.Unmarshal(orderRecorder.Body.Bytes(), &order); err != nil {
		t.Fatal(err)
	}
	if order.TotalAmount != 5_000_000 || len(order.ChainOrderID) != 64 || order.Status != "pending_payment" {
		t.Fatalf("unexpected order response: %+v", order)
	}
	var remaining int64
	if err := db.QueryRow("SELECT inventory_quantity FROM listings WHERE id = ?", listing.ID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("inventory = %d, want 1", remaining)
	}
	// The same listing cannot be over-reserved.
	overOrder := serve(s, http.MethodPost, "/orders", `{"listing_id":`+itoa(listing.ID)+`,"quantity":2}`, buyerCookie)
	if overOrder.Code != http.StatusConflict {
		t.Fatalf("over-reservation status = %d, want %d", overOrder.Code, http.StatusConflict)
	}
	// A syntactically valid signature is enough for this fake verifier; production verification uses finalized RPC.
	sig := solana.SignatureFromBytes(make([]byte, 64)).String()
	chain.verification = blockchain.Verification{}
	fund := serve(s, http.MethodPost, "/orders/1/payment/transactions", `{"type":"fund","signature":"`+sig+`"}`, buyerCookie)
	if fund.Code != http.StatusAccepted {
		t.Fatalf("fund status %d: %s", fund.Code, fund.Body.String())
	}
	if chain.lastExpected.Amount != 5_000_000 || chain.lastExpected.TokenMint != testUSDCMint {
		t.Fatalf("unexpected verification expectation: %+v", chain.lastExpected)
	}
	orderPending := serve(s, http.MethodGet, "/orders/1", "", buyerCookie)
	if orderPending.Code != http.StatusOK || !strings.Contains(orderPending.Body.String(), `"status":"pending_payment"`) {
		t.Fatalf("unfinalized funding changed order status: %d %s", orderPending.Code, orderPending.Body.String())
	}
	if _, err := db.Exec("UPDATE transaction_history SET next_verification_at = '2000-01-01T00:00:00Z' WHERE transaction_signature = ?", sig); err != nil {
		t.Fatal(err)
	}
	chain.verification = blockchain.Verification{Finalized: true, Succeeded: true}
	s.reconcilePending(context.Background())
	fulfill := serve(s, http.MethodPost, "/orders/1/fulfill", `{}`, sellerCookie)
	if fulfill.Code != http.StatusOK {
		t.Fatalf("fulfill status %d: %s", fulfill.Code, fulfill.Body.String())
	}
	releaseSig := solana.SignatureFromBytes(append(make([]byte, 63), 1)).String()
	release := serve(s, http.MethodPost, "/orders/1/payment/transactions", `{"type":"release","signature":"`+releaseSig+`"}`, buyerCookie)
	if release.Code != http.StatusOK {
		t.Fatalf("release status %d: %s", release.Code, release.Body.String())
	}
	orderGet := serve(s, http.MethodGet, "/orders/1", "", buyerCookie)
	if orderGet.Code != http.StatusOK || !strings.Contains(orderGet.Body.String(), `"status":"completed"`) {
		t.Fatalf("order after release: %d %s", orderGet.Code, orderGet.Body.String())
	}
}

func TestWriteRoutesRequireAllowedOriginAndSession(t *testing.T) {
	s, _ := newTestServer(t, nil)
	noOrigin := httptest.NewRequest(http.MethodPost, "/auth/challenge", strings.NewReader(`{"wallet_address":"11111111111111111111111111111111"}`))
	noOriginResponse := httptest.NewRecorder()
	s.Routes().ServeHTTP(noOriginResponse, noOrigin)
	if noOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("missing origin status = %d, want %d", noOriginResponse.Code, http.StatusForbidden)
	}
	unauthenticated := serve(s, http.MethodPost, "/listings", `{}`, nil)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", unauthenticated.Code, http.StatusUnauthorized)
	}
}

func TestOriginAllowedCloudflareWildcardIsHTTPSAndHostBound(t *testing.T) {
	allowed := map[string]struct{}{"https://*.trycloudflare.com": {}}
	tests := []struct {
		origin string
		want   bool
	}{
		{origin: "https://dozens-years-alternatively-laboratory.trycloudflare.com", want: true},
		{origin: "https://nested.tunnel.trycloudflare.com", want: true},
		{origin: "http://dozens-years-alternatively-laboratory.trycloudflare.com", want: false},
		{origin: "https://trycloudflare.com", want: false},
		{origin: "https://tunnel.trycloudflare.com.attacker.test", want: false},
		{origin: "https://tunnel.trycloudflare.com:8443", want: false},
		{origin: "https://tunnel.trycloudflare.com/path", want: false},
	}
	for _, test := range tests {
		t.Run(test.origin, func(t *testing.T) {
			if got := originAllowed(test.origin, allowed); got != test.want {
				t.Errorf("originAllowed(%q) = %t, want %t", test.origin, got, test.want)
			}
		})
	}
}

func TestListingStorefrontFieldsAndPublicSellerProfile(t *testing.T) {
	s, _ := newTestServer(t, nil)
	cookie := createUserSession(t, s, "Market Seller", "market-seller@example.test")
	profileResponse := serve(s, http.MethodGet, "/auth/me", "", cookie)
	var profile struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(profileResponse.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	publicProfile := serve(s, http.MethodGet, "/public/users/"+itoa(profile.ID), "", nil)
	if publicProfile.Code != http.StatusOK || strings.Contains(publicProfile.Body.String(), "market-seller@example.test") {
		t.Fatalf("public profile exposed private data: %d %s", publicProfile.Code, publicProfile.Body.String())
	}

	created := serve(s, http.MethodPost, "/listings", `{"title":"Fresh yams","description":"Harvested this week","price":2500000,"currency":"USDC","category":"Food & produce","unit":"crate","location":"Lagos","image_url":"https://images.example.test/yams.jpg","inventory_quantity":12}`, cookie)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"category":"Food \u0026 produce"`) || !strings.Contains(created.Body.String(), `"image_url":"https://images.example.test/yams.jpg"`) {
		t.Fatalf("create listing with storefront metadata: %d %s", created.Code, created.Body.String())
	}
	var listing struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	invalidImage := serve(s, http.MethodPatch, "/listings/"+itoa(listing.ID), `{"image_url":"javascript:alert(1)"}`, cookie)
	if invalidImage.Code != http.StatusBadRequest {
		t.Fatalf("non-HTTPS image status = %d, want %d", invalidImage.Code, http.StatusBadRequest)
	}
	updated := serve(s, http.MethodPatch, "/listings/"+itoa(listing.ID), `{"location":"Ikeja"}`, cookie)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"location":"Ikeja"`) {
		t.Fatalf("patch listing location: %d %s", updated.Code, updated.Body.String())
	}
}

func TestOrderCancellationWaitsForExpiryAndRestoresInventory(t *testing.T) {
	chain := &fakeChain{stateErr: blockchain.ErrEscrowNotFound}
	s, db := newTestServer(t, chain)
	sellerCookie := createUserSession(t, s, "Seller", "seller-cancel@example.test")
	buyerCookie := createUserSession(t, s, "Buyer", "buyer-cancel@example.test")
	listing := serve(s, http.MethodPost, "/listings", `{"title":"Tea","price":1000000,"currency":"USDC","inventory_quantity":2}`, sellerCookie)
	var listingResult struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(listing.Body.Bytes(), &listingResult); err != nil {
		t.Fatal(err)
	}
	created := serve(s, http.MethodPost, "/orders", `{"listing_id":`+itoa(listingResult.ID)+`,"quantity":1}`, buyerCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create order status %d: %s", created.Code, created.Body.String())
	}
	cancelTooEarly := serve(s, http.MethodPost, "/orders/1/cancel", `{}`, buyerCookie)
	if cancelTooEarly.Code != http.StatusConflict {
		t.Fatalf("early cancellation status = %d, want %d", cancelTooEarly.Code, http.StatusConflict)
	}
	if _, err := db.Exec("UPDATE payments SET expires_at = ? WHERE order_id = 1", time.Now().UTC().Add(-6*time.Minute).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	cancel := serve(s, http.MethodPost, "/orders/1/cancel", `{}`, buyerCookie)
	if cancel.Code != http.StatusOK || !strings.Contains(cancel.Body.String(), `"status":"cancelled"`) {
		t.Fatalf("expired order cancellation status %d: %s", cancel.Code, cancel.Body.String())
	}
	var inventory int64
	if err := db.QueryRow("SELECT inventory_quantity FROM listings WHERE id = ?", listingResult.ID).Scan(&inventory); err != nil {
		t.Fatal(err)
	}
	if inventory != 2 {
		t.Fatalf("restored inventory = %d, want 2", inventory)
	}
}

func TestConversationMessagesAreParticipantOnly(t *testing.T) {
	s, db := newTestServer(t, nil)
	buyerCookie := createUserSession(t, s, "Buyer", "message-buyer@example.test")
	sellerCookie := createUserSession(t, s, "Seller", "message-seller@example.test")
	strangerCookie := createUserSession(t, s, "Stranger", "message-stranger@example.test")
	var sellerID int64
	if err := db.QueryRow("SELECT id FROM users WHERE email = ?", "message-seller@example.test").Scan(&sellerID); err != nil {
		t.Fatal(err)
	}
	conversationResponse := serve(s, http.MethodPost, "/conversations", `{"seller_id":`+itoa(sellerID)+`}`, buyerCookie)
	if conversationResponse.Code != http.StatusCreated {
		t.Fatalf("create conversation status %d: %s", conversationResponse.Code, conversationResponse.Body.String())
	}
	var conversation struct {
		ID      int64  `json:"id"`
		OrderID *int64 `json:"order_id"`
	}
	if err := json.Unmarshal(conversationResponse.Body.Bytes(), &conversation); err != nil {
		t.Fatal(err)
	}
	if conversation.OrderID != nil {
		t.Fatalf("new marketplace conversation order_id = %d, want null", *conversation.OrderID)
	}
	listResponse := serve(s, http.MethodGet, "/conversations", "", buyerCookie)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), `"order_id":null`) {
		t.Fatalf("list conversations response should expose nullable order_id as JSON null: %d %s", listResponse.Code, listResponse.Body.String())
	}
	message := serve(s, http.MethodPost, "/conversations/"+itoa(conversation.ID)+"/messages", `{"content":"Hello, is this available?"}`, buyerCookie)
	if message.Code != http.StatusCreated {
		t.Fatalf("create message status %d: %s", message.Code, message.Body.String())
	}
	read := serve(s, http.MethodGet, "/conversations/"+itoa(conversation.ID)+"/messages", "", sellerCookie)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), "Hello, is this available?") {
		t.Fatalf("participant read status %d: %s", read.Code, read.Body.String())
	}
	forbidden := serve(s, http.MethodGet, "/conversations/"+itoa(conversation.ID)+"/messages", "", strangerCookie)
	if forbidden.Code != http.StatusNotFound {
		t.Fatalf("non-participant read status = %d, want %d", forbidden.Code, http.StatusNotFound)
	}
}

func createUserSession(t *testing.T, s *Server, name, email string) *http.Cookie {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wallet := solana.PublicKeyFromBytes(public).String()
	challengeRequest := httptest.NewRequest(http.MethodPost, "/auth/challenge", strings.NewReader(`{"wallet_address":"`+wallet+`"}`))
	challengeRequest.Header.Set("Origin", "http://localhost:5173")
	challengeRecorder := httptest.NewRecorder()
	s.Routes().ServeHTTP(challengeRecorder, challengeRequest)
	if challengeRecorder.Code != http.StatusCreated {
		t.Fatalf("challenge status %d: %s", challengeRecorder.Code, challengeRecorder.Body.String())
	}
	var challenge challengeResponse
	if err := json.Unmarshal(challengeRecorder.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	signature := solana.SignatureFromBytes(ed25519.Sign(private, []byte(challenge.Message))).String()
	body := `{"wallet_address":"` + wallet + `","nonce":"` + challenge.Nonce + `","signature":"` + signature + `","name":"` + name + `","email":"` + email + `"}`
	verifyRequest := httptest.NewRequest(http.MethodPost, "/auth/verify", strings.NewReader(body))
	verifyRequest.Header.Set("Origin", "http://localhost:5173")
	verifyResponse := httptest.NewRecorder()
	s.Routes().ServeHTTP(verifyResponse, verifyRequest)
	if verifyResponse.Code != http.StatusOK {
		t.Fatalf("verify status %d: %s", verifyResponse.Code, verifyResponse.Body.String())
	}
	return verifyResponse.Result().Cookies()[0]
}

func serve(s *Server, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPost || method == http.MethodPatch {
		request.Header.Set("Origin", "http://localhost:5173")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	s.Routes().ServeHTTP(response, request)
	return response
}

func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}
