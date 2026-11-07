package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/pact-marketplace/pact/backend/internal/data"
	solana "github.com/solana-foundation/solana-go/v2"
)

type challengeRequest struct {
	WalletAddress string `json:"wallet_address"`
}

type challengeResponse struct {
	Nonce     string `json:"nonce"`
	Message   string `json:"message"`
	ExpiresAt string `json:"expires_at"`
}

func (s *Server) createChallenge(w http.ResponseWriter, r *http.Request) {
	var input challengeRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	wallet, err := solana.PublicKeyFromBase58(strings.TrimSpace(input.WalletAddress))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_wallet", "wallet_address must be a valid Solana public key")
		return
	}
	walletAddress := wallet.String()
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		s.internalError(w, "create auth challenge", err)
		return
	}
	nonce := hex.EncodeToString(nonceBytes)
	expires := time.Now().UTC().Add(s.config.ChallengeTTL)
	origin := strings.TrimRight(r.Header.Get("Origin"), "/")
	message := fmt.Sprintf("Pact wallet sign-in\nOrigin: %s\nWallet: %s\nNonce: %s\nIssued At: %s\nExpiration Time: %s\n", origin, walletAddress, nonce, time.Now().UTC().Format(time.RFC3339), expires.Format(time.RFC3339))
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.internalError(w, "begin auth challenge", err)
		return
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)
	count, err := queries.CountRecentAuthChallenges(r.Context(), walletAddress)
	if err != nil {
		s.internalError(w, "count auth challenges", err)
		return
	}
	if count >= 10 {
		writeError(w, http.StatusTooManyRequests, "challenge_rate_limited", "too many sign-in challenges; try again later")
		return
	}
	if _, err := queries.CreateAuthChallenge(r.Context(), data.CreateAuthChallengeParams{
		WalletAddress: walletAddress, Nonce: nonce, Message: message, ExpiresAt: expires.Format(time.RFC3339Nano),
	}); err != nil {
		s.internalError(w, "store auth challenge", err)
		return
	}
	if err := tx.Commit(); err != nil {
		s.internalError(w, "commit auth challenge", err)
		return
	}
	if err := s.queries.DeleteExpiredChallenges(r.Context()); err != nil {
		s.logger.Warn("cleanup expired wallet challenges failed", "error", err)
	}
	writeJSON(w, http.StatusCreated, challengeResponse{Nonce: nonce, Message: message, ExpiresAt: expires.Format(time.RFC3339)})
}

type verifyChallengeRequest struct {
	WalletAddress string `json:"wallet_address"`
	Nonce         string `json:"nonce"`
	Signature     string `json:"signature"`
	Name          string `json:"name"`
	Email         string `json:"email"`
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

func (s *Server) verifyChallenge(w http.ResponseWriter, r *http.Request) {
	var input verifyChallengeRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	wallet, err := solana.PublicKeyFromBase58(strings.TrimSpace(input.WalletAddress))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_wallet", "wallet_address must be a valid Solana public key")
		return
	}
	input.WalletAddress = wallet.String()
	challenge, err := s.queries.GetAuthChallenge(r.Context(), strings.TrimSpace(input.Nonce))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (challenge.WalletAddress != input.WalletAddress || challenge.ConsumedAt.Valid)) {
		writeError(w, http.StatusUnauthorized, "invalid_challenge", "challenge is missing, expired, consumed, or belongs to another wallet")
		return
	}
	if err != nil {
		s.internalError(w, "load auth challenge", err)
		return
	}
	expires, err := time.Parse(time.RFC3339Nano, challenge.ExpiresAt)
	if err != nil || !time.Now().Before(expires) {
		writeError(w, http.StatusUnauthorized, "challenge_expired", "wallet sign-in challenge has expired")
		return
	}
	signature, err := solana.SignatureFromBase58(strings.TrimSpace(input.Signature))
	if err != nil || !ed25519.Verify(ed25519.PublicKey(wallet[:]), []byte(challenge.Message), signature[:]) {
		writeError(w, http.StatusUnauthorized, "invalid_signature", "wallet signature does not match the challenge")
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.internalError(w, "begin wallet sign-in", err)
		return
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)
	consumed, err := queries.ConsumeAuthChallenge(r.Context(), data.ConsumeAuthChallengeParams{Nonce: challenge.Nonce, WalletAddress: input.WalletAddress})
	if err != nil {
		s.internalError(w, "consume auth challenge", err)
		return
	}
	if consumed != 1 {
		writeError(w, http.StatusUnauthorized, "invalid_challenge", "wallet sign-in challenge has expired or was already used")
		return
	}
	user, err := queries.GetUserByWallet(r.Context(), sql.NullString{String: input.WalletAddress, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		name := strings.TrimSpace(input.Name)
		email := strings.ToLower(strings.TrimSpace(input.Email))
		parsedEmail, emailErr := mail.ParseAddress(email)
		if name == "" || emailErr != nil || parsedEmail.Address != email {
			writeError(w, http.StatusBadRequest, "profile_required", "name and a valid contact email are required for first sign-in")
			return
		}
		user, err = queries.CreateUser(r.Context(), data.CreateUserParams{
			Name: name, Email: email, WalletAddress: sql.NullString{String: input.WalletAddress, Valid: true},
		})
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, "profile_conflict", "that email or wallet is already associated with another account")
			return
		}
		s.internalError(w, "load or create wallet user", err)
		return
	}
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		s.internalError(w, "create session token", err)
		return
	}
	tokenHash := sha256.Sum256(rawToken)
	expiresAt := time.Now().UTC().Add(s.config.SessionTTL)
	if err := queries.CreateSession(r.Context(), data.CreateSessionParams{
		TokenHash: tokenHash[:], UserID: user.ID, ExpiresAt: expiresAt.Format(time.RFC3339Nano),
	}); err != nil {
		s.internalError(w, "create session", err)
		return
	}
	if err := tx.Commit(); err != nil {
		s.internalError(w, "commit wallet sign-in", err)
		return
	}
	if err := s.queries.DeleteExpiredSessions(r.Context()); err != nil {
		s.logger.Warn("cleanup expired sessions failed", "error", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name: s.config.CookieName, Value: hex.EncodeToString(rawToken), Path: "/", HttpOnly: true,
		Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: expiresAt, MaxAge: int(s.config.SessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, presentUser(user))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.config.CookieName); err == nil {
		if token, decodeErr := decodeSessionToken(cookie.Value); decodeErr == nil {
			hash := sha256.Sum256(token)
			if err := s.queries.DeleteSession(r.Context(), hash[:]); err != nil {
				s.internalError(w, "revoke session", err)
				return
			}
		}
	}
	http.SetCookie(w, &http.Cookie{Name: s.config.CookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func decodeSessionToken(value string) ([]byte, error) {
	token, err := hex.DecodeString(value)
	if err != nil || len(token) != 32 {
		return nil, errors.New("invalid session token")
	}
	return token, nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "sign in with your wallet to continue")
		return
	}
	writeJSON(w, http.StatusOK, presentUser(user))
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
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
	current, ok := currentUser(r)
	if !ok || current.ID != user.ID {
		writeError(w, http.StatusForbidden, "profile_private", "users may only read their own private profile")
		return
	}
	writeJSON(w, http.StatusOK, presentUser(user))
}
