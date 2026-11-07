// Package blockchain is the boundary for future Solana RPC integration.
package blockchain

import "context"

type Config struct {
	RPCURL    string
	ProgramID string
}

// Client describes the chain facts the application must verify. Implementations
// should query finalized RPC state; they must not trust browser notifications.
type Client interface {
	VerifyTransaction(context.Context, string) (Verification, error)
	EscrowState(context.Context, string) (EscrowState, error)
}

type Verification struct {
	Signature string
	Finalized bool
	Succeeded bool
}

type EscrowState struct {
	Address string
	Status  string
	Amount  uint64
	Mint    string
}

// RPCClient is configuration scaffolding. RPC methods intentionally return an
// explicit error until an RPC implementation and generated program client exist.
type RPCClient struct{ config Config }

func NewRPCClient(config Config) *RPCClient { return &RPCClient{config: config} }

func (c *RPCClient) VerifyTransaction(context.Context, string) (Verification, error) {
	return Verification{}, ErrNotImplemented
}

func (c *RPCClient) EscrowState(context.Context, string) (EscrowState, error) {
	return EscrowState{}, ErrNotImplemented
}

func (c *RPCClient) Config() Config { return c.config }
