// Package blockchain verifies marketplace payment facts against Solana RPC.
// It never signs or submits transactions and never trusts browser notifications.
package blockchain

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	solana "github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/rpc"
)

const (
	EscrowSeed               = "escrow"
	ConfigSeed               = "marketplace_config"
	FulfillmentWindowSeconds = int64(7 * 24 * 60 * 60)
)

type TransactionType string

const (
	TransactionFund    TransactionType = "fund"
	TransactionRelease TransactionType = "release"
	TransactionRefund  TransactionType = "refund"
)

type Config struct {
	RPCURL    string
	ProgramID string
}

type Expectation struct {
	OrderID          [32]byte
	EscrowAddress    string
	Buyer            string
	Seller           string
	TokenMint        string
	Amount           uint64
	FundingExpiresAt int64
	Type             TransactionType
}

// Client is the chain boundary used by the order/payment workflows.
type Client interface {
	DeriveEscrowAddress([32]byte) (string, error)
	VerifyTransaction(context.Context, string, Expectation) (Verification, error)
	EscrowState(context.Context, string) (EscrowState, error)
}

type Verification struct {
	Finalized bool
	Succeeded bool
	State     EscrowState
}

type EscrowState struct {
	Address        string
	VaultAddress   string
	OrderID        [32]byte
	Buyer          string
	Seller         string
	Status         string
	Amount         uint64
	VaultAmount    uint64
	Mint           string
	Created        int64
	FundingExpires int64
	FundedAt       int64
	Expires        int64
}

type RPCClient struct {
	config    Config
	rpc       *rpc.Client
	program   solana.PublicKey
	programOK bool
}

func NewRPCClient(config Config) *RPCClient {
	client := &RPCClient{config: config, rpc: rpc.NewWithTimeout(config.RPCURL, 10*time.Second)}
	program, err := solana.PublicKeyFromBase58(config.ProgramID)
	if err == nil {
		client.program = program
		client.programOK = true
	}
	return client
}

func (c *RPCClient) DeriveEscrowAddress(orderID [32]byte) (string, error) {
	if !c.programOK {
		return "", errors.New("PACT_SOLANA_PROGRAM_ID is not a valid Solana address")
	}
	address, _, err := solana.FindProgramAddress([][]byte{[]byte(EscrowSeed), orderID[:]}, c.program)
	if err != nil {
		return "", fmt.Errorf("derive escrow PDA: %w", err)
	}
	return address.String(), nil
}

func (c *RPCClient) VerifyTransaction(ctx context.Context, signature string, expected Expectation) (Verification, error) {
	if !c.programOK {
		return Verification{}, errors.New("PACT_SOLANA_PROGRAM_ID is not a valid Solana address")
	}
	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return Verification{}, fmt.Errorf("invalid transaction signature: %w", err)
	}
	version := uint64(1)
	result, err := c.rpc.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
		Encoding: solana.EncodingJSON, Commitment: rpc.CommitmentFinalized,
		MaxSupportedTransactionVersion: &version,
	})
	if errors.Is(err, rpc.ErrNotFound) {
		return Verification{}, nil
	}
	if err != nil {
		return Verification{}, fmt.Errorf("get finalized transaction: %w", err)
	}
	if result.Meta == nil {
		return Verification{}, errors.New("finalized transaction has no metadata")
	}
	if result.Transaction == nil {
		return Verification{}, errors.New("finalized transaction has no transaction data")
	}
	tx, err := result.Transaction.GetTransaction()
	if err != nil {
		return Verification{}, fmt.Errorf("decode transaction: %w", err)
	}
	if len(tx.Signatures) == 0 || tx.Signatures[0] != sig {
		return Verification{}, ErrInvalidTransaction
	}
	if err := c.verifyInstruction(tx, result.Meta.LoadedAddresses, expected); err != nil {
		return Verification{}, err
	}
	if result.Meta.Err != nil {
		return Verification{Finalized: true, Succeeded: false}, nil
	}
	state, err := c.EscrowState(ctx, expected.EscrowAddress)
	if err != nil {
		return Verification{}, fmt.Errorf("read escrow account after transaction: %w", err)
	}
	if err := compareState(state, expected); err != nil {
		return Verification{}, err
	}
	wantState := map[TransactionType]string{
		TransactionFund: "funded", TransactionRelease: "released", TransactionRefund: "refunded",
	}[expected.Type]
	if state.Status != wantState {
		return Verification{}, fmt.Errorf("%w: escrow status is %q, expected %q", ErrInvalidTransaction, state.Status, wantState)
	}
	if (expected.Type == TransactionFund && state.VaultAmount != expected.Amount) ||
		(expected.Type != TransactionFund && state.VaultAmount != 0) {
		return Verification{}, fmt.Errorf("%w: escrow vault balance does not match the expected state", ErrInvalidTransaction)
	}
	return Verification{Finalized: true, Succeeded: true, State: state}, nil
}

func (c *RPCClient) EscrowState(ctx context.Context, address string) (EscrowState, error) {
	if !c.programOK {
		return EscrowState{}, errors.New("PACT_SOLANA_PROGRAM_ID is not a valid Solana address")
	}
	key, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return EscrowState{}, fmt.Errorf("invalid escrow address: %w", err)
	}
	accountInfo, err := c.rpc.GetAccountInfoWithOpts(ctx, key, &rpc.GetAccountInfoOpts{
		Encoding: solana.EncodingBase64, Commitment: rpc.CommitmentFinalized,
	})
	if errors.Is(err, rpc.ErrNotFound) {
		return EscrowState{}, ErrEscrowNotFound
	}
	if err != nil {
		return EscrowState{}, fmt.Errorf("get escrow account: %w", err)
	}
	if accountInfo.Value == nil || accountInfo.Value.Owner != c.program {
		return EscrowState{}, ErrInvalidEscrowAccount
	}
	data := accountInfo.GetBinary()
	if len(data) != 8+32+32+32+32+8+1+8+8+8+8+1 {
		return EscrowState{}, ErrInvalidEscrowAccount
	}
	discriminator := accountDiscriminator("Escrow")
	if !equalBytes(data[:8], discriminator[:]) {
		return EscrowState{}, ErrInvalidEscrowAccount
	}
	data = data[8:]
	var state EscrowState
	state.Address = address
	copy(state.OrderID[:], data[:32])
	data = data[32:]
	buyer := solana.PublicKeyFromBytes(data[:32])
	state.Buyer = buyer.String()
	data = data[32:]
	seller := solana.PublicKeyFromBytes(data[:32])
	state.Seller = seller.String()
	data = data[32:]
	mint := solana.PublicKeyFromBytes(data[:32])
	state.Mint = mint.String()
	data = data[32:]
	state.Amount = binary.LittleEndian.Uint64(data[:8])
	status := data[8]
	if int(status) >= len(escrowStatuses) {
		return EscrowState{}, ErrInvalidEscrowAccount
	}
	state.Status = escrowStatuses[status]
	data = data[9:]
	state.Created = int64(binary.LittleEndian.Uint64(data[:8]))
	state.FundingExpires = int64(binary.LittleEndian.Uint64(data[8:16]))
	state.FundedAt = int64(binary.LittleEndian.Uint64(data[16:24]))
	state.Expires = int64(binary.LittleEndian.Uint64(data[24:32]))
	derived, err := c.DeriveEscrowAddress(state.OrderID)
	if err != nil || derived != address {
		return EscrowState{}, ErrInvalidEscrowAccount
	}
	vault, err := associatedTokenAddress(key, mint)
	if err != nil {
		return EscrowState{}, err
	}
	vaultInfo, err := c.rpc.GetAccountInfoWithOpts(ctx, vault, &rpc.GetAccountInfoOpts{
		Encoding: solana.EncodingBase64, Commitment: rpc.CommitmentFinalized,
	})
	if err != nil {
		return EscrowState{}, fmt.Errorf("get escrow token vault: %w", err)
	}
	tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	vaultData := vaultInfo.GetBinary()
	if vaultInfo.Value == nil || vaultInfo.Value.Owner != tokenProgram || len(vaultData) < 165 ||
		!equalBytes(vaultData[:32], mint[:]) || !equalBytes(vaultData[32:64], key[:]) {
		return EscrowState{}, ErrInvalidEscrowAccount
	}
	state.VaultAddress = vault.String()
	state.VaultAmount = binary.LittleEndian.Uint64(vaultData[64:72])
	if state.Status == "created" {
		if state.FundedAt != 0 || state.Expires != 0 {
			return EscrowState{}, ErrInvalidEscrowAccount
		}
	} else if state.Status == "funded" || state.Status == "released" || state.Status == "refunded" {
		if state.FundedAt <= 0 || state.Expires != state.FundedAt+FulfillmentWindowSeconds {
			return EscrowState{}, ErrInvalidEscrowAccount
		}
	}
	return state, nil
}

var escrowStatuses = [...]string{"created", "funded", "released", "refunded", "disputed"}

func (c *RPCClient) verifyInstruction(tx *solana.Transaction, loaded rpc.LoadedAddresses, expected Expectation) error {
	keys := append(solana.PublicKeySlice(nil), tx.Message.AccountKeys...)
	keys = append(keys, loaded.Writable...)
	keys = append(keys, loaded.ReadOnly...)
	if len(keys) == 0 {
		return ErrInvalidTransaction
	}
	escrow, err := solana.PublicKeyFromBase58(expected.EscrowAddress)
	if err != nil {
		return ErrInvalidTransaction
	}
	buyer, err := solana.PublicKeyFromBase58(expected.Buyer)
	if err != nil {
		return ErrInvalidTransaction
	}
	seller, err := solana.PublicKeyFromBase58(expected.Seller)
	if err != nil {
		return ErrInvalidTransaction
	}
	mint, err := solana.PublicKeyFromBase58(expected.TokenMint)
	if err != nil {
		return ErrInvalidTransaction
	}
	discriminator := instructionDiscriminator(string(expected.Type))
	for _, instruction := range tx.Message.Instructions {
		if int(instruction.ProgramIDIndex) >= len(keys) || keys[instruction.ProgramIDIndex] != c.program {
			continue
		}
		data := []byte(instruction.Data)
		if len(data) < len(discriminator) || !equalBytes(data[:8], discriminator[:]) {
			continue
		}
		accountKeys := make([]solana.PublicKey, len(instruction.Accounts))
		for i, index := range instruction.Accounts {
			if int(index) >= len(keys) {
				return ErrInvalidTransaction
			}
			accountKeys[i] = keys[index]
		}
		if len(accountKeys) < 3 {
			return ErrInvalidTransaction
		}
		switch expected.Type {
		case TransactionFund:
			vault, vaultErr := associatedTokenAddress(escrow, mint)
			buyerToken, buyerTokenErr := associatedTokenAddress(buyer, mint)
			tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
			if vaultErr != nil || buyerTokenErr != nil || len(accountKeys) < 6 || accountKeys[0] != escrow || accountKeys[1] != buyer || accountKeys[2] != mint || accountKeys[3] != vault || accountKeys[4] != buyerToken || accountKeys[5] != tokenProgram || !tx.IsSigner(buyer) {
				return ErrInvalidTransaction
			}
		case TransactionRelease:
			vault, vaultErr := associatedTokenAddress(escrow, mint)
			sellerToken, sellerTokenErr := associatedTokenAddress(seller, mint)
			tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
			associatedProgram := solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
			if vaultErr != nil || sellerTokenErr != nil || len(accountKeys) < 9 || accountKeys[0] != escrow ||
				accountKeys[1] != buyer || accountKeys[2] != seller || accountKeys[3] != mint ||
				accountKeys[4] != vault || accountKeys[5] != sellerToken || accountKeys[6] != associatedProgram ||
				accountKeys[7] != tokenProgram || !tx.IsSigner(buyer) {
				return ErrInvalidTransaction
			}
		case TransactionRefund:
			vault, vaultErr := associatedTokenAddress(escrow, mint)
			buyerToken, buyerTokenErr := associatedTokenAddress(buyer, mint)
			tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
			if vaultErr != nil || buyerTokenErr != nil || len(accountKeys) < 7 || accountKeys[1] != escrow ||
				accountKeys[2] != buyer || accountKeys[3] != mint || accountKeys[4] != vault ||
				accountKeys[5] != buyerToken || accountKeys[6] != tokenProgram || !tx.IsSigner(accountKeys[0]) {
				return ErrInvalidTransaction
			}
		default:
			return ErrInvalidTransaction
		}
		return nil
	}
	return ErrInvalidTransaction
}

func associatedTokenAddress(owner, mint solana.PublicKey) (solana.PublicKey, error) {
	tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	associatedProgram := solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	address, _, err := solana.FindProgramAddress([][]byte{owner[:], tokenProgram[:], mint[:]}, associatedProgram)
	return address, err
}

func compareState(state EscrowState, expected Expectation) error {
	if state.OrderID != expected.OrderID || state.Buyer != expected.Buyer || state.Seller != expected.Seller ||
		state.Mint != expected.TokenMint || state.Amount != expected.Amount || state.FundingExpires != expected.FundingExpiresAt {
		return ErrInvalidTransaction
	}
	return nil
}

func instructionDiscriminator(name string) [8]byte {
	anchorName := map[string]string{
		string(TransactionFund):    "fund_escrow",
		string(TransactionRelease): "release_escrow",
		string(TransactionRefund):  "refund_after_timeout",
	}[name]
	hash := sha256.Sum256([]byte("global:" + anchorName))
	return [8]byte(hash[:8])
}

func accountDiscriminator(name string) [8]byte {
	hash := sha256.Sum256([]byte("account:" + name))
	return [8]byte(hash[:8])
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// DecodeOrderID is useful at the HTTP boundary, where chain_order_id is returned as hex.
func DecodeOrderID(value string) ([32]byte, error) {
	var id [32]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != len(id) {
		return id, errors.New("chain order id must be 64 hexadecimal characters")
	}
	copy(id[:], decoded)
	return id, nil
}
