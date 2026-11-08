package blockchain

import "errors"

var (
	ErrInvalidTransaction   = errors.New("transaction does not match the expected Solstice escrow action")
	ErrInvalidEscrowAccount = errors.New("account is not a valid Solstice escrow PDA")
	ErrEscrowNotFound       = errors.New("escrow PDA does not exist")
)
