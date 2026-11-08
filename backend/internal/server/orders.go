package server

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/deltron-fr/solstice/backend/internal/blockchain"
	"github.com/deltron-fr/solstice/backend/internal/data"
	solana "github.com/solana-foundation/solana-go/v2"
)

type createOrderRequest struct {
	ListingID int64 `json:"listing_id"`
	Quantity  int64 `json:"quantity"`
}

type orderResponse struct {
	ID           int64  `json:"id"`
	BuyerID      int64  `json:"buyer_id"`
	SellerID     int64  `json:"seller_id"`
	ListingID    int64  `json:"listing_id"`
	Quantity     int64  `json:"quantity"`
	UnitPrice    int64  `json:"unit_price"`
	TotalAmount  int64  `json:"total_amount"`
	ChainOrderID string `json:"chain_order_id"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func presentOrder(order data.Order) orderResponse {
	return orderResponse{
		ID: order.ID, BuyerID: order.BuyerID, SellerID: order.SellerID, ListingID: order.ListingID,
		Quantity: order.Quantity, UnitPrice: order.UnitPrice, TotalAmount: order.TotalAmount,
		ChainOrderID: strings.ToLower(fmtHex(order.ChainOrderID)), Status: order.Status,
		CreatedAt: order.CreatedAt, UpdatedAt: order.UpdatedAt,
	}
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	var input createOrderRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if input.ListingID < 1 || input.Quantity < 1 {
		writeError(w, http.StatusBadRequest, "validation_error", "listing_id and positive quantity are required")
		return
	}
	if strings.TrimSpace(s.config.USDCMint) == "" {
		writeError(w, http.StatusServiceUnavailable, "payments_not_configured", "SOLSTICE_USDC_MINT must be configured before escrow-backed orders are available")
		return
	}
	if _, err := solana.PublicKeyFromBase58(s.config.USDCMint); err != nil {
		writeError(w, http.StatusServiceUnavailable, "payments_not_configured", "SOLSTICE_USDC_MINT is not a valid Solana address")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.internalError(w, "begin order creation", err)
		return
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)
	listing, err := queries.GetListingForUpdate(r.Context(), input.ListingID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && listing.Status != "active") {
		writeError(w, http.StatusNotFound, "listing_not_found", "active listing not found")
		return
	}
	if err != nil {
		s.internalError(w, "load order listing", err)
		return
	}
	if listing.SellerID == user.ID {
		writeError(w, http.StatusBadRequest, "self_purchase", "a seller cannot purchase their own listing")
		return
	}
	if listing.Currency != "USDC" {
		writeError(w, http.StatusUnprocessableEntity, "listing_not_escrow_eligible", "escrow-backed orders currently require a USDC-priced listing")
		return
	}
	if input.Quantity > listing.InventoryQuantity {
		writeError(w, http.StatusConflict, "insufficient_inventory", "listing does not have enough inventory")
		return
	}
	if listing.Price > math.MaxInt64/input.Quantity {
		writeError(w, http.StatusBadRequest, "amount_overflow", "order total exceeds the supported amount")
		return
	}
	var chainOrderID [32]byte
	if _, err := rand.Read(chainOrderID[:]); err != nil {
		s.internalError(w, "generate chain order reference", err)
		return
	}
	escrowAddress, err := s.chain.DeriveEscrowAddress(chainOrderID)
	if err != nil {
		s.internalError(w, "derive escrow PDA", err)
		return
	}
	expiresAt := time.Now().UTC().Add(30 * time.Minute)
	reserved, err := queries.ReserveListingInventory(r.Context(), data.ReserveListingInventoryParams{
		InventoryQuantity: input.Quantity, InventoryQuantity_2: input.Quantity,
		ID: listing.ID, InventoryQuantity_3: input.Quantity,
	})
	if err != nil {
		s.internalError(w, "reserve listing inventory", err)
		return
	}
	if reserved != 1 {
		writeError(w, http.StatusConflict, "insufficient_inventory", "listing inventory changed; refresh and try again")
		return
	}
	order, err := queries.CreateOrder(r.Context(), data.CreateOrderParams{
		BuyerID: user.ID, SellerID: listing.SellerID, ListingID: listing.ID, Quantity: input.Quantity,
		UnitPrice: listing.Price, TotalAmount: listing.Price * input.Quantity, ChainOrderID: chainOrderID[:],
	})
	if err != nil {
		s.internalError(w, "create order", err)
		return
	}
	_, err = queries.CreatePayment(r.Context(), data.CreatePaymentParams{
		OrderID: order.ID, EscrowPdaAddress: sql.NullString{String: escrowAddress, Valid: true},
		Amount: order.TotalAmount, TokenMint: s.config.USDCMint, ExpiresAt: expiresAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		s.internalError(w, "create payment record", err)
		return
	}
	if err := tx.Commit(); err != nil {
		s.internalError(w, "commit order", err)
		return
	}
	writeJSON(w, http.StatusCreated, presentOrder(order))
}

func (s *Server) listOrders(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	role := r.URL.Query().Get("as")
	if role == "" {
		role = "buyer"
	}
	if role != "buyer" && role != "seller" {
		writeError(w, http.StatusBadRequest, "invalid_role", "as must be buyer or seller")
		return
	}
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
	orders, err := s.queries.ListOrdersForUser(r.Context(), data.ListOrdersForUserParams{
		BuyerID: user.ID, Column2: role, SellerID: user.ID, Column4: role, Limit: limit, Offset: offset,
	})
	if err != nil {
		s.internalError(w, "list orders", err)
		return
	}
	if orders == nil {
		orders = []data.Order{}
	}
	response := make([]orderResponse, len(orders))
	for i := range orders {
		response[i] = presentOrder(orders[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": response, "role": role, "limit": limit, "offset": offset})
}

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	order, err := s.queries.GetOrderForUser(r.Context(), data.GetOrderForUserParams{ID: id, BuyerID: user.ID, SellerID: user.ID})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	if err != nil {
		s.internalError(w, "get order", err)
		return
	}
	writeJSON(w, http.StatusOK, presentOrder(order))
}

func (s *Server) fulfillOrder(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	updated, err := s.queries.MarkOrderFulfilling(r.Context(), data.MarkOrderFulfillingParams{ID: id, SellerID: user.ID})
	if err != nil {
		s.internalError(w, "mark order fulfilling", err)
		return
	}
	if updated != 1 {
		writeError(w, http.StatusConflict, "invalid_order_transition", "only the seller can mark a paid order as fulfilling")
		return
	}
	order, err := s.queries.GetOrder(r.Context(), id)
	if err != nil {
		s.internalError(w, "load fulfilled order", err)
		return
	}
	writeJSON(w, http.StatusOK, presentOrder(order))
}

func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	order, err := s.queries.GetOrderForUser(r.Context(), data.GetOrderForUserParams{ID: id, BuyerID: user.ID, SellerID: user.ID})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	if err != nil {
		s.internalError(w, "load order to cancel", err)
		return
	}
	if order.BuyerID != user.ID || order.Status != "pending_payment" {
		writeError(w, http.StatusConflict, "invalid_order_transition", "only the buyer may cancel an unpaid order")
		return
	}
	payment, err := s.queries.GetPaymentForOrder(r.Context(), id)
	if err != nil {
		s.internalError(w, "load payment to cancel", err)
		return
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, payment.ExpiresAt)
	if err != nil {
		s.internalError(w, "parse payment expiry", err)
		return
	}
	// Wait past the on-chain payment window and a finality grace period before
	// restoring inventory. The deployed program must reject deposits after this expiry.
	if time.Now().Before(expiresAt.Add(5 * time.Minute)) {
		writeError(w, http.StatusConflict, "payment_window_open", "order can be cancelled after its payment window expires")
		return
	}
	if s.chain == nil || !payment.EscrowPdaAddress.Valid {
		writeError(w, http.StatusServiceUnavailable, "chain_verification_unavailable", "cannot safely cancel without checking finalized escrow state")
		return
	}
	escrow, err := s.chain.EscrowState(r.Context(), payment.EscrowPdaAddress.String)
	if err != nil && !errors.Is(err, blockchain.ErrEscrowNotFound) {
		s.logger.Warn("cancellation escrow check failed", "order_id", order.ID, "error", err)
		writeError(w, http.StatusServiceUnavailable, "chain_verification_unavailable", "cannot safely cancel without checking finalized escrow state")
		return
	}
	if err == nil && (escrow.Status != "created" || escrow.VaultAmount != 0 || escrow.FundingExpires > time.Now().Unix()) {
		writeError(w, http.StatusConflict, "escrow_not_refundable", "an escrow exists in a state that cannot be cancelled off-chain")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.internalError(w, "begin order cancellation", err)
		return
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)
	updated, err := queries.CancelUnfundedOrder(r.Context(), data.CancelUnfundedOrderParams{ID: id, BuyerID: user.ID})
	if err != nil {
		s.internalError(w, "cancel order", err)
		return
	}
	if updated != 1 {
		writeError(w, http.StatusConflict, "invalid_order_transition", "order state changed; refresh and retry")
		return
	}
	if err := queries.RestoreListingInventory(r.Context(), data.RestoreListingInventoryParams{InventoryQuantity: order.Quantity, ID: order.ListingID}); err != nil {
		s.internalError(w, "restore listing inventory", err)
		return
	}
	if err := tx.Commit(); err != nil {
		s.internalError(w, "commit order cancellation", err)
		return
	}
	order.Status = "cancelled"
	writeJSON(w, http.StatusOK, presentOrder(order))
}

func (s *Server) getPayment(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	order, err := s.queries.GetOrderForUser(r.Context(), data.GetOrderForUserParams{ID: id, BuyerID: user.ID, SellerID: user.ID})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	if err != nil {
		s.internalError(w, "load payment order", err)
		return
	}
	payment, err := s.queries.GetPaymentForOrder(r.Context(), id)
	if err != nil {
		s.internalError(w, "get order payment", err)
		return
	}
	var escrowPDA, transactionSignature *string
	if payment.EscrowPdaAddress.Valid {
		value := payment.EscrowPdaAddress.String
		escrowPDA = &value
	}
	if payment.TransactionSignature.Valid {
		value := payment.TransactionSignature.String
		transactionSignature = &value
	}
	var escrowState any
	if s.chain != nil && payment.EscrowPdaAddress.Valid {
		state, stateErr := s.chain.EscrowState(r.Context(), payment.EscrowPdaAddress.String)
		if stateErr == nil {
			escrowState = map[string]any{
				"status": state.Status, "funding_expires_at": state.FundingExpires,
				"funded_at": state.FundedAt, "expires_at": state.Expires,
				"vault_amount": state.VaultAmount,
			}
		} else if !errors.Is(stateErr, blockchain.ErrEscrowNotFound) {
			s.logger.Warn("read escrow for payment response", "order_id", id, "error", stateErr)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"payment": map[string]any{
			"id": payment.ID, "order_id": payment.OrderID, "escrow_pda_address": escrowPDA,
			"transaction_signature": transactionSignature, "amount": payment.Amount,
			"token_mint": payment.TokenMint, "status": payment.Status,
			"funding_expires_at": payment.ExpiresAt, "created_at": payment.CreatedAt,
			"updated_at": payment.UpdatedAt,
		},
		"chain_order_id":     strings.ToLower(fmtHex(order.ChainOrderID)),
		"escrow_pda_address": escrowPDA, "funding_expires_at": payment.ExpiresAt,
		"escrow_state": escrowState,
	})
}

func fmtHex(value []byte) string {
	const digits = "0123456789abcdef"
	result := make([]byte, len(value)*2)
	for i, b := range value {
		result[i*2] = digits[b>>4]
		result[i*2+1] = digits[b&0x0f]
	}
	return string(result)
}
