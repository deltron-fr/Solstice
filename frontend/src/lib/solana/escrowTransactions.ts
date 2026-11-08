import {
  AccountRole,
  address,
  appendTransactionMessageInstructions,
  compileTransaction,
  createSolanaRpc,
  createTransactionMessage,
  estimateResourceLimitsFactory,
  fillTransactionMessageProvisoryResourceLimits,
  getAddressEncoder,
  getBase64EncodedWireTransaction,
  getSignatureFromTransaction,
  getTransactionDecoder,
  getTransactionEncoder,
  getUtf8Encoder,
  getProgramDerivedAddress,
  setTransactionMessageConfig,
  setTransactionMessageFeePayer,
  setTransactionMessageLifetimeUsingBlockhash,
  type Address,
  type Instruction,
  type ReadonlyUint8Array,
  type Transaction,
} from '@solana/kit';
import type { OrderResponse, PaymentResponse } from '../api/client';
import { rpcUrl, walletChain } from './client';

const PROGRAM_ID = address(import.meta.env.VITE_SOLANA_PROGRAM_ID ?? 'GDSz1GPfxqK5yioYqQRbubENibvvaZDkkcRYDzkhzG4A');
const TOKEN_PROGRAM = address('TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA');
const ASSOCIATED_TOKEN_PROGRAM = address('ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL');
const SYSTEM_PROGRAM = address('11111111111111111111111111111111');
const utf8 = getUtf8Encoder();
const addressEncoder = getAddressEncoder();
const rpc = createSolanaRpc(rpcUrl);

export type EscrowAction = 'fund' | 'release' | 'refund';

function hexBytes(value: string): Uint8Array {
  if (!/^[0-9a-f]{64}$/i.test(value)) throw new Error('The order is missing a valid 32-byte on-chain reference.');
  return Uint8Array.from({ length: 32 }, (_, index) => Number.parseInt(value.slice(index * 2, index * 2 + 2), 16));
}

async function anchorDiscriminator(name: string): Promise<Uint8Array> {
  const digest = await crypto.subtle.digest('SHA-256', utf8.encode(`global:${name}`));
  return new Uint8Array(digest).slice(0, 8);
}

function encodeInteger(value: bigint, byteLength: number, signed: boolean): Uint8Array {
  const buffer = new ArrayBuffer(byteLength);
  const view = new DataView(buffer);
  if (byteLength === 8) {
    if (signed) view.setBigInt64(0, value, true);
    else view.setBigUint64(0, value, true);
  } else if (byteLength === 1) {
    view.setUint8(0, Number(value));
  } else {
    throw new Error('Unsupported instruction integer width');
  }
  return new Uint8Array(buffer);
}

function joinBytes(...parts: Uint8Array[]): Uint8Array {
  const result = new Uint8Array(parts.reduce((size, part) => size + part.length, 0));
  let offset = 0;
  for (const part of parts) { result.set(part, offset); offset += part.length; }
  return result;
}

async function associatedTokenAddress(owner: Address, mint: Address): Promise<Address> {
  const [tokenAccount] = await getProgramDerivedAddress({
    programAddress: ASSOCIATED_TOKEN_PROGRAM,
    seeds: [addressEncoder.encode(owner), addressEncoder.encode(TOKEN_PROGRAM), addressEncoder.encode(mint)],
  });
  return tokenAccount;
}

async function deriveEscrow(orderId: Uint8Array): Promise<{ address: Address; config: Address }> {
  const [escrow] = await getProgramDerivedAddress({ programAddress: PROGRAM_ID, seeds: [utf8.encode('escrow'), orderId] });
  const [config] = await getProgramDerivedAddress({ programAddress: PROGRAM_ID, seeds: [utf8.encode('marketplace_config')] });
  return { address: escrow, config };
}

function meta(account: Address, role: AccountRole) {
  return { address: account, role };
}

async function createEscrowInstruction(
  buyer: Address,
  seller: Address,
  acceptedMint: Address,
  config: Address,
  escrow: Address,
  vault: Address,
  orderId: Uint8Array,
  amount: bigint,
  fundingExpiresAt: bigint,
): Promise<Instruction> {
  const data = joinBytes(await anchorDiscriminator('create_escrow'), orderId, encodeInteger(amount, 8, false), encodeInteger(fundingExpiresAt, 8, true));
  return {
    programAddress: PROGRAM_ID,
    accounts: [
      meta(buyer, AccountRole.WRITABLE_SIGNER),
      meta(seller, AccountRole.READONLY),
      meta(config, AccountRole.READONLY),
      meta(acceptedMint, AccountRole.READONLY),
      meta(escrow, AccountRole.WRITABLE),
      meta(vault, AccountRole.WRITABLE),
      meta(TOKEN_PROGRAM, AccountRole.READONLY),
      meta(ASSOCIATED_TOKEN_PROGRAM, AccountRole.READONLY),
      meta(SYSTEM_PROGRAM, AccountRole.READONLY),
    ],
    data,
  };
}

async function fundEscrowInstruction(buyer: Address, mint: Address, escrow: Address, vault: Address, buyerTokenAccount: Address): Promise<Instruction> {
  return {
    programAddress: PROGRAM_ID,
    accounts: [
      meta(escrow, AccountRole.WRITABLE),
      meta(buyer, AccountRole.WRITABLE_SIGNER),
      meta(mint, AccountRole.READONLY),
      meta(vault, AccountRole.WRITABLE),
      meta(buyerTokenAccount, AccountRole.WRITABLE),
      meta(TOKEN_PROGRAM, AccountRole.READONLY),
    ],
    data: await anchorDiscriminator('fund_escrow'),
  };
}

async function releaseEscrowInstruction(buyer: Address, seller: Address, mint: Address, escrow: Address, vault: Address, sellerTokenAccount: Address): Promise<Instruction> {
  return {
    programAddress: PROGRAM_ID,
    accounts: [
      meta(escrow, AccountRole.WRITABLE),
      meta(buyer, AccountRole.WRITABLE_SIGNER),
      meta(seller, AccountRole.READONLY),
      meta(mint, AccountRole.READONLY),
      meta(vault, AccountRole.WRITABLE),
      meta(sellerTokenAccount, AccountRole.WRITABLE),
      meta(ASSOCIATED_TOKEN_PROGRAM, AccountRole.READONLY),
      meta(TOKEN_PROGRAM, AccountRole.READONLY),
      meta(SYSTEM_PROGRAM, AccountRole.READONLY),
    ],
    data: await anchorDiscriminator('release_escrow'),
  };
}

async function refundAfterTimeoutInstruction(buyer: Address, mint: Address, escrow: Address, vault: Address, buyerTokenAccount: Address): Promise<Instruction> {
  return {
    programAddress: PROGRAM_ID,
    accounts: [
      meta(buyer, AccountRole.WRITABLE_SIGNER),
      meta(escrow, AccountRole.WRITABLE),
      meta(buyer, AccountRole.READONLY),
      meta(mint, AccountRole.READONLY),
      meta(vault, AccountRole.WRITABLE),
      meta(buyerTokenAccount, AccountRole.WRITABLE),
      meta(TOKEN_PROGRAM, AccountRole.READONLY),
    ],
    data: await anchorDiscriminator('refund_after_timeout'),
  };
}

function createAssociatedTokenAccountIdempotent(payer: Address, owner: Address, mint: Address, tokenAccount: Address): Instruction {
  return {
    programAddress: ASSOCIATED_TOKEN_PROGRAM,
    accounts: [
      meta(payer, AccountRole.WRITABLE_SIGNER),
      meta(tokenAccount, AccountRole.WRITABLE),
      meta(owner, AccountRole.READONLY),
      meta(mint, AccountRole.READONLY),
      meta(SYSTEM_PROGRAM, AccountRole.READONLY),
      meta(TOKEN_PROGRAM, AccountRole.READONLY),
    ],
    data: new Uint8Array([1]),
  };
}

export async function buildEscrowTransaction(
  action: EscrowAction,
  order: OrderResponse,
  payment: PaymentResponse,
  buyerAddress: string,
  sellerAddress: string,
): Promise<Transaction> {
  const buyer = address(buyerAddress);
  const seller = address(sellerAddress);
  const mint = address(payment.payment.token_mint);
  const configuredMint = import.meta.env.VITE_SOLANA_USDC_MINT;
  if (configuredMint && configuredMint !== mint) throw new Error('The order token mint does not match VITE_SOLANA_USDC_MINT.');
  const orderId = hexBytes(payment.chain_order_id);
  const { address: escrow, config } = await deriveEscrow(orderId);
  if (payment.escrow_pda_address !== escrow) throw new Error('The backend escrow address does not match the program PDA derived by this app.');
  if (order.id !== payment.payment.order_id || order.chain_order_id !== payment.chain_order_id) throw new Error('Payment details do not match this order. Refresh and try again.');
  const amount = BigInt(payment.payment.amount);
  if (amount <= 0n) throw new Error('The payment amount must be greater than zero.');
  const vault = await associatedTokenAddress(escrow, mint);
  const buyerToken = await associatedTokenAddress(buyer, mint);
  const latestBlockhash = await rpc.getLatestBlockhash({ commitment: 'confirmed' }).send();
  const instructions: Instruction[] = [];

  if (action === 'fund') {
    instructions.push(createAssociatedTokenAccountIdempotent(buyer, buyer, mint, buyerToken));
    if (payment.escrow_state === null) {
      const fundingDeadline = BigInt(Math.floor(Date.parse(payment.funding_expires_at) / 1000));
      if (fundingDeadline <= BigInt(Math.floor(Date.now() / 1000))) throw new Error('The payment window has expired.');
      instructions.push(await createEscrowInstruction(buyer, seller, mint, config, escrow, vault, orderId, amount, fundingDeadline));
    } else if (payment.escrow_state.status !== 'created') {
      throw new Error(`This escrow cannot be funded from its current on-chain state (${payment.escrow_state.status}).`);
    }
    instructions.push(await fundEscrowInstruction(buyer, mint, escrow, vault, buyerToken));
  } else if (action === 'release') {
    if (payment.escrow_state?.status !== 'funded') throw new Error('Only a funded escrow can release payment.');
    const sellerToken = await associatedTokenAddress(seller, mint);
    instructions.push(await releaseEscrowInstruction(buyer, seller, mint, escrow, vault, sellerToken));
  } else {
    if (payment.escrow_state?.status !== 'funded' || payment.escrow_state.expires_at > Math.floor(Date.now() / 1000)) {
      throw new Error('The funded escrow refund timeout has not passed.');
    }
    instructions.push(createAssociatedTokenAccountIdempotent(buyer, buyer, mint, buyerToken));
    instructions.push(await refundAfterTimeoutInstruction(buyer, mint, escrow, vault, buyerToken));
  }

  const draft = fillTransactionMessageProvisoryResourceLimits(appendTransactionMessageInstructions(
    instructions,
    setTransactionMessageLifetimeUsingBlockhash(latestBlockhash.value, setTransactionMessageFeePayer(buyer, createTransactionMessage({ version: 1 }))),
  ));
  const estimate = await estimateResourceLimitsFactory({ rpc })(draft);
  const dataPage = 32 * 1024;
  const computeUnitLimit = Math.min(1_400_000, Math.ceil(estimate.computeUnitLimit * 1.2) + 10_000);
  const loadedAccountsDataSizeLimit = Math.min(
    64 * 1024 * 1024,
    Math.ceil((estimate.loadedAccountsDataSizeLimit + dataPage) / dataPage) * dataPage,
  );
  const message = setTransactionMessageConfig({ computeUnitLimit, loadedAccountsDataSizeLimit }, draft);
  return compileTransaction(message);
}

export async function simulateAndSubmit(
  transaction: Transaction,
  signWithWallet: (bytes: ReadonlyUint8Array) => Promise<ReadonlyUint8Array>,
  onStage?: (stage: 'wallet' | 'submit') => void,
): Promise<string> {
  const encoded = getBase64EncodedWireTransaction(transaction);
  const simulation = await rpc.simulateTransaction(encoded, {
    encoding: 'base64', commitment: 'confirmed', sigVerify: false, replaceRecentBlockhash: true,
  }).send();
  if (simulation.value.err !== null) {
    const matchingLogs = simulation.value.logs?.filter((line) => line.includes('Error') || line.includes('failed')) ?? [];
    const simulationError = JSON.stringify(simulation.value.err, (_, value: unknown) => typeof value === 'bigint' ? value.toString() : value);
    const detail = matchingLogs.join(' ') || `Transaction simulation failed: ${simulationError}`;
    throw new Error(detail);
  }

  const serialized = getTransactionEncoder().encode(transaction);
  onStage?.('wallet');
  const walletSigned = await signWithWallet(serialized);
  const signedTransaction = getTransactionDecoder().decode(walletSigned);
  const signature = getSignatureFromTransaction(signedTransaction);
  onStage?.('submit');
  await rpc.sendTransaction(getBase64EncodedWireTransaction(signedTransaction), {
    encoding: 'base64', preflightCommitment: 'confirmed', maxRetries: 3n,
  }).send();
  return signature;
}

export { walletChain };
