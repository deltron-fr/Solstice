import * as anchor from '@anchor-lang/core';
import { getMint, TOKEN_PROGRAM_ID } from '@solana/spl-token';

async function initializeDevnetConfig(): Promise<void> {
  const mintAddress = process.env.PACT_USDC_MINT?.trim();
  if (!mintAddress) {
    throw new Error('Set PACT_USDC_MINT to the address of your devnet test mint.');
  }

  const provider = anchor.AnchorProvider.env();
  const rpc = new URL(provider.connection.rpcEndpoint);
  if (!rpc.hostname.toLowerCase().includes('devnet')) {
    throw new Error(`Refusing to initialize outside devnet. RPC endpoint is ${provider.connection.rpcEndpoint}`);
  }

  const mint = new anchor.web3.PublicKey(mintAddress);
  const mintInfo = await getMint(provider.connection, mint, 'confirmed', TOKEN_PROGRAM_ID);
  if (mintInfo.decimals !== 6) {
    throw new Error(`Expected a classic SPL test mint with 6 decimals; this mint has ${mintInfo.decimals}.`);
  }

  anchor.setProvider(provider);
  const program = anchor.workspace.PactEscrow;
  const [config] = anchor.web3.PublicKey.findProgramAddressSync(
    [Buffer.from('marketplace_config')],
    program.programId,
  );

  const existingAccount = await provider.connection.getAccountInfo(config, 'confirmed');
  if (existingAccount) {
    const existingConfig = await program.account.marketplaceConfig.fetch(config);
    if (!existingConfig.acceptedMint.equals(mint)) {
      throw new Error(`Config is already initialized for mint ${existingConfig.acceptedMint.toBase58()}; it cannot be changed by this command.`);
    }
    if (!existingConfig.administrator.equals(provider.wallet.publicKey)) {
      throw new Error(`Config is already initialized with administrator ${existingConfig.administrator.toBase58()}, not the configured Anchor wallet.`);
    }
    console.log(`Marketplace config is already initialized at ${config.toBase58()}.`);
    console.log(`Accepted mint: ${mint.toBase58()}`);
    return;
  }

  const signature = await program.methods.initialize().accounts({
    administrator: provider.wallet.publicKey,
    acceptedMint: mint,
    config,
    systemProgram: anchor.web3.SystemProgram.programId,
  }).rpc();

  console.log(`Initialized marketplace config: ${config.toBase58()}`);
  console.log(`Program: ${program.programId.toBase58()}`);
  console.log(`Administrator: ${provider.wallet.publicKey.toBase58()}`);
  console.log(`Accepted mint: ${mint.toBase58()}`);
  console.log(`Transaction: ${signature}`);
}

initializeDevnetConfig().catch((error: unknown) => {
  console.error(error);
  process.exitCode = 1;
});
