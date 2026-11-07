package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pact-marketplace/pact/backend/internal/blockchain"
	"github.com/pact-marketplace/pact/backend/internal/data"
	solana "github.com/solana-foundation/solana-go/v2"
)

var errStalePaymentAction = errors.New("order state changed while verifying transaction")

type submitTransactionRequest struct {
	Type      blockchain.TransactionType `json:"type"`
	Signature string                     `json:"signature"`
}

func (s *Server) submitPaymentTransaction(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input submitTransactionRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Signature = strings.TrimSpace(input.Signature)
	if _, err := solana.SignatureFromBase58(input.Signature); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_signature", "signature must be a base58 Solana transaction signature")
		return
	}
	order, err := s.queries.GetOrderForUser(r.Context(), data.GetOrderForUserParams{ID: id, BuyerID: user.ID, SellerID: user.ID})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	if err != nil {
		s.internalError(w, "load order for transaction", err)
		return
	}
	if input.Type != blockchain.TransactionRefund && order.BuyerID != user.ID {
		writeError(w, http.StatusForbidden, "buyer_required", "only the buyer may submit this escrow transaction")
		return
	}
	if input.Type == blockchain.TransactionRefund && order.BuyerID != user.ID && order.SellerID != user.ID {
		writeError(w, http.StatusForbidden, "order_participant_required", "only an order participant may submit this escrow transaction")
		return
	}
	if !validPaymentAction(input.Type, order.Status) {
		writeError(w, http.StatusConflict, "invalid_payment_transition", "transaction type is not valid for the current order state")
		return
	}
	payment, err := s.queries.GetPaymentForOrder(r.Context(), order.ID)
	if err != nil {
		s.internalError(w, "load order payment", err)
		return
	}
	verification, err := s.verifyPaymentTransaction(r.Context(), order, payment, input.Signature, input.Type)
	if errors.Is(err, blockchain.ErrInvalidTransaction) {
		writeError(w, http.StatusUnprocessableEntity, "transaction_mismatch", "transaction does not match this order and requested escrow action")
		return
	}
	if err != nil {
		s.logger.Warn("payment verification deferred", "order_id", order.ID, "signature", input.Signature, "error", err)
		writeError(w, http.StatusServiceUnavailable, "chain_verification_unavailable", "Solana RPC could not verify the transaction; retry shortly")
		return
	}
	if err := s.recordTransactionResult(r.Context(), order, input.Signature, input.Type, verification); err != nil {
		if errors.Is(err, errStalePaymentAction) {
			writeError(w, http.StatusConflict, "invalid_payment_transition", "order state changed while the transaction was being verified")
			return
		}
		s.internalError(w, "record payment transaction", err)
		return
	}
	status := http.StatusAccepted
	state := "submitted"
	if verification.Finalized {
		status = http.StatusOK
		state = "failed"
		if verification.Succeeded {
			state = "finalized"
		}
	}
	writeJSON(w, status, map[string]any{"signature": input.Signature, "status": state, "order_id": order.ID})
}

func validPaymentAction(action blockchain.TransactionType, orderStatus string) bool {
	switch action {
	case blockchain.TransactionFund:
		return orderStatus == "pending_payment"
	case blockchain.TransactionRelease:
		return orderStatus == "fulfilling"
	case blockchain.TransactionRefund:
		return orderStatus == "paid" || orderStatus == "fulfilling"
	default:
		return false
	}
}

func (s *Server) verifyPaymentTransaction(ctx context.Context, order data.Order, payment data.Payment, signature string, action blockchain.TransactionType) (blockchain.Verification, error) {
	if len(order.ChainOrderID) != 32 || !payment.EscrowPdaAddress.Valid {
		return blockchain.Verification{}, blockchain.ErrInvalidTransaction
	}
	fundingExpiresAt, err := time.Parse(time.RFC3339Nano, payment.ExpiresAt)
	if err != nil {
		return blockchain.Verification{}, errors.New("payment expiry is invalid")
	}
	var orderID [32]byte
	copy(orderID[:], order.ChainOrderID)
	if s.chain == nil {
		return blockchain.Verification{}, errors.New("blockchain verifier is not configured")
	}
	buyer, err := s.queries.GetUser(ctx, order.BuyerID)
	if err != nil || !buyer.WalletAddress.Valid {
		return blockchain.Verification{}, errors.New("order buyer has no linked wallet")
	}
	seller, err := s.queries.GetUser(ctx, order.SellerID)
	if err != nil || !seller.WalletAddress.Valid {
		return blockchain.Verification{}, errors.New("order seller has no linked wallet")
	}
	return s.chain.VerifyTransaction(ctx, signature, blockchain.Expectation{
		OrderID: orderID, EscrowAddress: payment.EscrowPdaAddress.String,
		Buyer: buyer.WalletAddress.String, Seller: seller.WalletAddress.String,
		TokenMint: payment.TokenMint, Amount: uint64(payment.Amount), FundingExpiresAt: fundingExpiresAt.Unix(), Type: action,
	})
}

func (s *Server) recordTransactionResult(ctx context.Context, order data.Order, signature string, action blockchain.TransactionType, verification blockchain.Verification) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)
	currentOrder, err := queries.GetOrder(ctx, order.ID)
	if err != nil {
		return err
	}
	if !verification.Finalized {
		if !validPaymentAction(action, currentOrder.Status) {
			return errStalePaymentAction
		}
	} else if verification.Succeeded && !validPaymentAction(action, currentOrder.Status) && !orderAlreadyInResultState(currentOrder.Status, action) {
		return errStalePaymentAction
	}
	entry, err := queries.GetTransactionHistoryBySignature(ctx, signature)
	if err == nil {
		if entry.OrderID != order.ID || entry.TransactionType != string(action) {
			return errors.New("transaction signature already belongs to another action or order")
		}
		if entry.Status == "finalized" || entry.Status == "failed" {
			return tx.Commit()
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	} else {
		if _, err := queries.CreateTransactionHistory(ctx, data.CreateTransactionHistoryParams{
			OrderID: order.ID, TransactionSignature: signature, TransactionType: string(action),
		}); err != nil {
			return err
		}
	}
	if !verification.Finalized {
		if action == blockchain.TransactionFund {
			if err := queries.SetPaymentSubmitted(ctx, data.SetPaymentSubmittedParams{
				TransactionSignature: sql.NullString{String: signature, Valid: true}, OrderID: order.ID,
			}); err != nil {
				return err
			}
		}
		if err := queries.ScheduleTransactionRetry(ctx, data.ScheduleTransactionRetryParams{
			NextVerificationAt:    time.Now().UTC().Add(retryDelay(0)).Format(time.RFC3339Nano),
			LastVerificationError: "", TransactionSignature: signature,
		}); err != nil {
			return err
		}
		return tx.Commit()
	}
	if !verification.Succeeded {
		if err := queries.UpdateTransactionStatus(ctx, data.UpdateTransactionStatusParams{Status: "failed", TransactionSignature: signature}); err != nil {
			return err
		}
		if action == blockchain.TransactionFund {
			if err := queries.SetPaymentSubmitted(ctx, data.SetPaymentSubmittedParams{
				TransactionSignature: sql.NullString{String: signature, Valid: true}, OrderID: order.ID,
			}); err != nil {
				return err
			}
			if err := queries.SetPaymentFailed(ctx, data.SetPaymentFailedParams{OrderID: order.ID, TransactionSignature: sql.NullString{String: signature, Valid: true}}); err != nil {
				return err
			}
		}
		return tx.Commit()
	}
	if err := queries.UpdateTransactionStatus(ctx, data.UpdateTransactionStatusParams{Status: "finalized", TransactionSignature: signature}); err != nil {
		return err
	}
	var affected int64
	switch action {
	case blockchain.TransactionFund:
		affected, err = queries.SetOrderPaid(ctx, order.ID)
		if err == nil {
			err = queries.SetPaymentStatus(ctx, data.SetPaymentStatusParams{Status: "finalized", OrderID: order.ID})
		}
	case blockchain.TransactionRelease:
		affected, err = queries.SetOrderCompleted(ctx, order.ID)
		if err == nil {
			err = queries.SetPaymentStatus(ctx, data.SetPaymentStatusParams{Status: "released", OrderID: order.ID})
		}
	case blockchain.TransactionRefund:
		affected, err = queries.SetOrderRefunded(ctx, order.ID)
		if err == nil {
			err = queries.SetPaymentStatus(ctx, data.SetPaymentStatusParams{Status: "refunded", OrderID: order.ID})
		}
	default:
		return fmt.Errorf("unsupported escrow action %q", action)
	}
	if err != nil {
		return err
	}
	if affected != 1 {
		// A previous request may have committed the same chain result first.
		fresh, getErr := queries.GetOrder(ctx, order.ID)
		if getErr != nil || !orderAlreadyInResultState(fresh.Status, action) {
			return errors.New("on-chain state finalized but the order transition did not match the application state")
		}
	}
	return tx.Commit()
}

func orderAlreadyInResultState(status string, action blockchain.TransactionType) bool {
	switch action {
	case blockchain.TransactionFund:
		return status == "paid" || status == "fulfilling" || status == "completed"
	case blockchain.TransactionRelease:
		return status == "completed"
	case blockchain.TransactionRefund:
		return status == "cancelled"
	default:
		return false
	}
}

// RunReconciler retries submitted signatures and updates the DB only after RPC verification.
func (s *Server) RunReconciler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcilePending(ctx)
		}
	}
}

func (s *Server) reconcilePending(ctx context.Context) {
	entries, err := s.queries.ListPendingTransactions(ctx, 50)
	if err != nil {
		s.logger.Error("list pending chain transactions failed", "error", err)
		return
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}
		order, err := s.queries.GetOrder(ctx, entry.OrderID)
		if err != nil {
			s.logger.Error("load pending transaction order failed", "order_id", entry.OrderID, "error", err)
			continue
		}
		payment, err := s.queries.GetPaymentForOrder(ctx, entry.OrderID)
		if err != nil {
			s.logger.Error("load pending transaction payment failed", "order_id", entry.OrderID, "error", err)
			continue
		}
		verification, err := s.verifyPaymentTransaction(ctx, order, payment, entry.TransactionSignature, blockchain.TransactionType(entry.TransactionType))
		if err != nil {
			s.logger.Warn("pending transaction verification failed", "signature", entry.TransactionSignature, "error", err)
			if errors.Is(err, blockchain.ErrInvalidTransaction) || errors.Is(err, errStalePaymentAction) {
				reason := "transaction did not match expected order action"
				if errors.Is(err, errStalePaymentAction) {
					reason = "order state changed before chain verification completed"
				}
				if rejectErr := s.queries.RejectTransactionVerification(ctx, data.RejectTransactionVerificationParams{
					LastVerificationError: reason, TransactionSignature: entry.TransactionSignature,
				}); rejectErr != nil {
					s.logger.Error("reject mismatched transaction failed", "signature", entry.TransactionSignature, "error", rejectErr)
				}
				if entry.TransactionType == string(blockchain.TransactionFund) {
					if paymentErr := s.queries.SetPaymentFailed(ctx, data.SetPaymentFailedParams{OrderID: entry.OrderID, TransactionSignature: sql.NullString{String: entry.TransactionSignature, Valid: true}}); paymentErr != nil {
						s.logger.Error("mark rejected funding payment failed", "order_id", entry.OrderID, "error", paymentErr)
					}
				}
				continue
			}
			if retryErr := s.scheduleRetry(ctx, entry, err.Error()); retryErr != nil {
				s.logger.Error("schedule chain verification retry failed", "signature", entry.TransactionSignature, "error", retryErr)
			}
			continue
		}
		if verification.Finalized {
			if err := s.recordTransactionResult(ctx, order, entry.TransactionSignature, blockchain.TransactionType(entry.TransactionType), verification); err != nil {
				s.logger.Error("reconcile transaction result failed", "signature", entry.TransactionSignature, "error", err)
			}
		} else if err := s.scheduleRetry(ctx, entry, "transaction not finalized yet"); err != nil {
			s.logger.Error("schedule pending transaction retry failed", "signature", entry.TransactionSignature, "error", err)
		}
	}
}

func (s *Server) scheduleRetry(ctx context.Context, entry data.TransactionHistory, reason string) error {
	return s.queries.ScheduleTransactionRetry(ctx, data.ScheduleTransactionRetryParams{
		NextVerificationAt:    time.Now().UTC().Add(retryDelay(entry.VerificationAttempts)).Format(time.RFC3339Nano),
		LastVerificationError: reason, TransactionSignature: entry.TransactionSignature,
	})
}

func retryDelay(attempt int64) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 5 {
		attempt = 5
	}
	delay := 15 * time.Second * time.Duration(1<<attempt)
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}
