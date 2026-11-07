import * as anchor from '@anchor-lang/core';
import { createInitializeMintInstruction, MINT_SIZE, TOKEN_PROGRAM_ID } from '@solana/spl-token';
import { assert, expect } from 'chai';

describe('pact-escrow scaffold', () => {
  const provider = anchor.AnchorProvider.env();
  anchor.setProvider(provider);
  const program = anchor.workspace.PactEscrow;

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

    const [config] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from('marketplace_config')],
      program.programId,
    );
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
});
