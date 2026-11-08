import * as anchor from '@anchor-lang/core';
import {
  createAssociatedTokenAccount,
  createInitializeMintInstruction,
  getAccount,
  getAssociatedTokenAddressSync,
  MINT_SIZE,
  mintTo,
  ASSOCIATED_TOKEN_PROGRAM_ID,
  TOKEN_PROGRAM_ID,
} from '@solana/spl-token';
import { assert, expect } from 'chai';

describe('solstice-escrow scaffold', () => {
  const provider = anchor.AnchorProvider.env();
  anchor.setProvider(provider);
  const program = anchor.workspace.SolsticeEscrow;
  let acceptedMint: anchor.web3.PublicKey;
  let config: anchor.web3.PublicKey;

  it('initializes the marketplace config PDA', async () => {
    const mint = anchor.web3.Keypair.generate();
    const signature = await provider.connection.requestAirdrop(provider.wallet.publicKey, 1_000_000_000);
    await provider.connection.confirmTransaction(signature, 'confirmed');
    const mintRent = await provider.connection.getMinimumBalanceForRentExemption(MINT_SIZE);
    const createMint = new anchor.web3.Transaction().add(
      anchor.web3.SystemProgram.createAccount({
        fromPubkey: provider.wallet.publicKey,
        newAccountPubkey: mint.publicKey,
        lamports: mintRent,
        space: MINT_SIZE,
        programId: TOKEN_PROGRAM_ID,
      }),
      createInitializeMintInstruction(mint.publicKey, 6, provider.wallet.publicKey, null, TOKEN_PROGRAM_ID),
    );
    await provider.sendAndConfirm(createMint, [mint]);

    [config] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from('marketplace_config')],
      program.programId,
    );
    acceptedMint = mint.publicKey;
    await program.methods.initialize().accounts({
      administrator: provider.wallet.publicKey,
      acceptedMint: mint.publicKey,
      config,
      systemProgram: anchor.web3.SystemProgram.programId,
    }).rpc();

    const account = await program.account.marketplaceConfig.fetch(config);
    assert(account.administrator.equals(provider.wallet.publicKey));
    assert(account.acceptedMint.equals(mint.publicKey));
    expect(account.version).to.equal(1);

    try {
      await program.methods.initialize().accounts({
        administrator: provider.wallet.publicKey,
        acceptedMint: mint.publicKey,
        config,
        systemProgram: anchor.web3.SystemProgram.programId,
      }).rpc();
      assert.fail('initialization should not overwrite the existing config');
    } catch (error) {
      expect(String(error)).to.include('already in use');
    }
  });

  it('funds the exact amount and releases it only after buyer confirmation', async () => {
    const buyer = provider.wallet.publicKey;
    const seller = anchor.web3.Keypair.generate();
    const airdrop = await provider.connection.requestAirdrop(seller.publicKey, 1_000_000_000);
    await provider.connection.confirmTransaction(airdrop, 'confirmed');
    const amount = 1_234_500;
    const buyerTokenAccount = await createAssociatedTokenAccount(
      provider.connection,
      provider.wallet.payer,
      acceptedMint,
      buyer,
      undefined,
      TOKEN_PROGRAM_ID,
    );
    const sellerTokenAccount = getAssociatedTokenAddressSync(acceptedMint, seller.publicKey, false, TOKEN_PROGRAM_ID);
    await mintTo(provider.connection, provider.wallet.payer, acceptedMint, buyerTokenAccount, provider.wallet.payer, amount);

    const orderId = Array.from({ length: 32 }, (_, index) => index + 1);
    const [escrow] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from('escrow'), Buffer.from(orderId)],
      program.programId,
    );
    const vault = getAssociatedTokenAddressSync(acceptedMint, escrow, true, TOKEN_PROGRAM_ID);
    const fundingExpiresAt = Math.floor(Date.now() / 1000) + 30 * 60;
    await program.methods.createEscrow(orderId, new anchor.BN(amount), new anchor.BN(fundingExpiresAt)).accounts({
      buyer,
      seller: seller.publicKey,
      config,
      acceptedMint,
      escrow,
      vault,
      tokenProgram: TOKEN_PROGRAM_ID,
      associatedTokenProgram: ASSOCIATED_TOKEN_PROGRAM_ID,
      systemProgram: anchor.web3.SystemProgram.programId,
    }).rpc();

    await program.methods.fundEscrow().accounts({
      escrow,
      buyer,
      tokenMint: acceptedMint,
      vault,
      buyerTokenAccount,
      tokenProgram: TOKEN_PROGRAM_ID,
    }).rpc();
    const fundedEscrow = await program.account.escrow.fetch(escrow);
    expect(JSON.stringify(fundedEscrow.status).toLowerCase()).to.include('funded');
    expect((await getAccount(provider.connection, vault, 'confirmed', TOKEN_PROGRAM_ID)).amount).to.equal(BigInt(amount));
    expect(fundedEscrow.fundedAt.toNumber()).to.be.greaterThan(0);
    expect(fundedEscrow.expiresAt.toNumber() - fundedEscrow.fundedAt.toNumber()).to.equal(7 * 24 * 60 * 60);

    try {
      await program.methods.refundAfterTimeout().accounts({
        caller: buyer,
        escrow,
        buyer,
        tokenMint: acceptedMint,
        vault,
        buyerTokenAccount,
        tokenProgram: TOKEN_PROGRAM_ID,
      }).rpc();
      assert.fail('an unexpired funded escrow must not be refunded');
    } catch (error) {
      expect(String(error)).to.include('deadline');
    }

    await program.methods.releaseEscrow().accounts({
      escrow,
      buyer,
      seller: seller.publicKey,
      tokenMint: acceptedMint,
      vault,
      sellerTokenAccount,
      associatedTokenProgram: ASSOCIATED_TOKEN_PROGRAM_ID,
      tokenProgram: TOKEN_PROGRAM_ID,
      systemProgram: anchor.web3.SystemProgram.programId,
    }).rpc();
    const releasedEscrow = await program.account.escrow.fetch(escrow);
    expect(JSON.stringify(releasedEscrow.status).toLowerCase()).to.include('released');
    expect((await getAccount(provider.connection, sellerTokenAccount, 'confirmed', TOKEN_PROGRAM_ID)).amount).to.equal(BigInt(amount));
    expect((await getAccount(provider.connection, vault, 'confirmed', TOKEN_PROGRAM_ID)).amount).to.equal(0n);
  });
});
