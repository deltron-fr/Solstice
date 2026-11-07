package blockchain

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	solana "github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/rpc"
	"github.com/solana-foundation/solana-go/v2/rpc/jsonrpc"
)

type nullRPC struct{}

func (nullRPC) CallForInto(_ context.Context, out any, method string, _ []any) error {
	if method != "getTransaction" {
		return nil
	}
	return json.Unmarshal([]byte("null"), out)
}

func (nullRPC) CallWithCallback(context.Context, string, []any, func(*http.Request, *http.Response) error) error {
	return nil
}

func (nullRPC) CallBatch(context.Context, jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	return nil, nil
}

func TestDeriveEscrowAddressAndInvalidProgram(t *testing.T) {
	client := NewRPCClient(Config{RPCURL: "http://127.0.0.1:8899", ProgramID: "11111111111111111111111111111111"})
	var orderID [32]byte
	orderID[0] = 42
	address, err := client.DeriveEscrowAddress(orderID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := solana.PublicKeyFromBase58(address); err != nil {
		t.Fatalf("derived PDA is not a valid public key: %v", err)
	}
	invalid := NewRPCClient(Config{RPCURL: "http://127.0.0.1:8899", ProgramID: "not-a-key"})
	if _, err := invalid.DeriveEscrowAddress(orderID); err == nil {
		t.Fatal("expected invalid program ID error")
	}
}

func TestVerifyTransactionReportsNotFinalizedWhenRPCHasNoFinalizedResult(t *testing.T) {
	client := NewRPCClient(Config{RPCURL: "http://invalid.local", ProgramID: "11111111111111111111111111111111"})
	client.rpc = rpc.NewWithCustomRPCClient(nullRPC{})
	signature := solana.SignatureFromBytes(make([]byte, 64)).String()
	result, err := client.VerifyTransaction(context.Background(), signature, Expectation{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Finalized || result.Succeeded {
		t.Fatalf("not-found transaction result = %+v, want pending", result)
	}
}

func TestVerifyTransactionRejectsMalformedSignatureBeforeRPC(t *testing.T) {
	client := NewRPCClient(Config{RPCURL: "not-a-url", ProgramID: "11111111111111111111111111111111"})
	if _, err := client.VerifyTransaction(context.Background(), "bad-signature", Expectation{}); err == nil {
		t.Fatal("expected malformed signature to fail before RPC")
	}
}

func TestVerifyInstructionChecksProgramAccountsAndBuyerSignature(t *testing.T) {
	program := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	client := &RPCClient{program: program, programOK: true}
	var buyerBytes, sellerBytes, escrowBytes [32]byte
	buyerBytes[0], sellerBytes[0], escrowBytes[0] = 1, 2, 3
	buyer := solana.PublicKeyFromBytes(buyerBytes[:])
	seller := solana.PublicKeyFromBytes(sellerBytes[:])
	escrow := solana.PublicKeyFromBytes(escrowBytes[:])
	mint := solana.MustPublicKeyFromBase58("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU")
	vault, err := associatedTokenAddress(escrow, mint)
	if err != nil {
		t.Fatal(err)
	}
	buyerToken, err := associatedTokenAddress(buyer, mint)
	if err != nil {
		t.Fatal(err)
	}
	tokenProgram := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	keys := solana.PublicKeySlice{buyer, program, escrow, mint, vault, buyerToken, tokenProgram}
	discriminator := instructionDiscriminator(string(TransactionFund))
	instruction := solana.CompiledInstruction{
		ProgramIDIndex: 1, Accounts: []uint16{2, 0, 3, 4, 5, 6},
		Data: solana.Base58(discriminator[:]),
	}
	tx := &solana.Transaction{Message: solana.Message{
		AccountKeys:  keys,
		Header:       solana.MessageHeader{NumRequiredSignatures: 1},
		Instructions: []solana.CompiledInstruction{instruction},
	}}
	expected := Expectation{
		EscrowAddress: escrow.String(), Buyer: buyer.String(), Seller: seller.String(),
		TokenMint: mint.String(), Type: TransactionFund,
	}
	if err := client.verifyInstruction(tx, rpc.LoadedAddresses{}, expected); err != nil {
		t.Fatalf("expected matching funding instruction to pass: %v", err)
	}
	tx.Message.Header.NumRequiredSignatures = 0
	if err := client.verifyInstruction(tx, rpc.LoadedAddresses{}, expected); err == nil {
		t.Fatal("expected funding instruction without buyer signature to fail")
	}
	tx.Message.Header.NumRequiredSignatures = 1
	expected.TokenMint = buyer.String()
	if err := client.verifyInstruction(tx, rpc.LoadedAddresses{}, expected); err == nil {
		t.Fatal("expected instruction with unexpected mint to fail")
	}
}
