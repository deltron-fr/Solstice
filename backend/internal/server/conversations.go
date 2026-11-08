package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/pact-marketplace/pact/backend/internal/data"
)

type createConversationRequest struct {
	SellerID int64  `json:"seller_id"`
	OrderID  *int64 `json:"order_id"`
}

type conversationResponse struct {
	ID        int64  `json:"id"`
	BuyerID   int64  `json:"buyer_id"`
	SellerID  int64  `json:"seller_id"`
	OrderID   *int64 `json:"order_id"`
	CreatedAt string `json:"created_at"`
}

func presentConversation(conversation data.Conversation) conversationResponse {
	response := conversationResponse{
		ID: conversation.ID, BuyerID: conversation.BuyerID, SellerID: conversation.SellerID,
		CreatedAt: conversation.CreatedAt,
	}
	if conversation.OrderID.Valid {
		orderID := conversation.OrderID.Int64
		response.OrderID = &orderID
	}
	return response
}

func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	var input createConversationRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if input.SellerID < 1 || input.SellerID == user.ID {
		writeError(w, http.StatusBadRequest, "invalid_participants", "seller_id must identify another user")
		return
	}
	var orderID sql.NullInt64
	if input.OrderID != nil {
		if *input.OrderID < 1 {
			writeError(w, http.StatusBadRequest, "invalid_order_id", "order_id must be a positive integer")
			return
		}
		order, err := s.queries.GetOrderForUser(r.Context(), data.GetOrderForUserParams{ID: *input.OrderID, BuyerID: user.ID, SellerID: user.ID})
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (order.BuyerID != user.ID || order.SellerID != input.SellerID)) {
			writeError(w, http.StatusNotFound, "order_not_found", "conversation order not found for these participants")
			return
		}
		if err != nil {
			s.internalError(w, "load conversation order", err)
			return
		}
		orderID = sql.NullInt64{Int64: *input.OrderID, Valid: true}
	}
	conversation, err := s.queries.CreateConversation(r.Context(), data.CreateConversationParams{BuyerID: user.ID, SellerID: input.SellerID, OrderID: orderID})
	if err != nil {
		// The no-order partial unique index can win a concurrent create; return its existing conversation.
		conversation, err = s.queries.GetConversationBetween(r.Context(), data.GetConversationBetweenParams{BuyerID: user.ID, SellerID: input.SellerID, OrderID: orderID})
		if err != nil {
			s.internalError(w, "create conversation", err)
			return
		}
	}
	if conversation.ID == 0 {
		conversation, err = s.queries.GetConversationBetween(r.Context(), data.GetConversationBetweenParams{BuyerID: user.ID, SellerID: input.SellerID, OrderID: orderID})
		if err != nil {
			s.internalError(w, "load existing conversation", err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, presentConversation(conversation))
}

func (s *Server) listConversations(w http.ResponseWriter, r *http.Request) {
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
	conversations, err := s.queries.ListConversationsForUser(r.Context(), data.ListConversationsForUserParams{
		BuyerID: user.ID, SellerID: user.ID, Limit: limit, Offset: offset,
	})
	if err != nil {
		s.internalError(w, "list conversations", err)
		return
	}
	response := make([]conversationResponse, 0, len(conversations))
	for _, conversation := range conversations {
		response = append(response, presentConversation(conversation))
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": response, "limit": limit, "offset": offset})
}

func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	conversation, err := s.queries.GetConversation(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && conversation.BuyerID != user.ID && conversation.SellerID != user.ID) {
		writeError(w, http.StatusNotFound, "not_found", "conversation not found")
		return
	}
	if err != nil {
		s.internalError(w, "load conversation", err)
		return
	}
	limit, err := positiveIntQuery(r, "limit", 50, 100)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_limit", err.Error())
		return
	}
	beforeID := int64(1<<63 - 1)
	if value := r.URL.Query().Get("before_id"); value != "" {
		beforeID, err = positiveIntQuery(r, "before_id", beforeID, 1<<63-1)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
			return
		}
	}
	messages, err := s.queries.ListMessages(r.Context(), data.ListMessagesParams{ConversationID: id, ID: beforeID, Limit: limit})
	if err != nil {
		s.internalError(w, "list conversation messages", err)
		return
	}
	if messages == nil {
		messages = []data.Message{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": messages, "before_id": beforeID, "limit": limit})
}

type createMessageRequest struct {
	Content string `json:"content"`
}

func (s *Server) createMessage(w http.ResponseWriter, r *http.Request) {
	user, _ := currentUser(r)
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	conversation, err := s.queries.GetConversation(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && conversation.BuyerID != user.ID && conversation.SellerID != user.ID) {
		writeError(w, http.StatusNotFound, "not_found", "conversation not found")
		return
	}
	if err != nil {
		s.internalError(w, "load conversation for message", err)
		return
	}
	var input createMessageRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Content = strings.TrimSpace(input.Content)
	if input.Content == "" || len([]rune(input.Content)) > 4000 {
		writeError(w, http.StatusBadRequest, "validation_error", "message content must contain between 1 and 4000 characters")
		return
	}
	message, err := s.queries.CreateMessage(r.Context(), data.CreateMessageParams{ConversationID: id, SenderID: user.ID, Content: input.Content})
	if err != nil {
		s.internalError(w, "create message", err)
		return
	}
	writeJSON(w, http.StatusCreated, message)
}
