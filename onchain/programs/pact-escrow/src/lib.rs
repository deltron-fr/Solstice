use anchor_lang::prelude::*;
use anchor_spl::associated_token::AssociatedToken;
use anchor_spl::token::{Mint, Token, TokenAccount};

declare_id!("GDSz1GPfxqK5yioYqQRbubENibvvaZDkkcRYDzkhzG4A");

pub const CONFIG_SEED: &[u8] = b"marketplace_config";
pub const ESCROW_SEED: &[u8] = b"escrow";

#[program]
pub mod pact_escrow {
    use super::*;

    /// Creates the singleton config PDA. The accepted mint is supplied at setup;
    /// no production token mint is embedded in this program.
    pub fn initialize(ctx: Context<Initialize>) -> Result<()> {
        let config = &mut ctx.accounts.config;
        config.administrator = ctx.accounts.administrator.key();
        config.accepted_mint = ctx.accounts.accepted_mint.key();
        config.version = 1;
        Ok(())
    }

    /// Account layouts and seed validation are scaffolded, but no escrow is
    /// created until the order-identity and expiry rules are finalized.
    pub fn create_escrow(
        _ctx: Context<CreateEscrow>,
        _order_id: [u8; 32],
        _amount: u64,
        _expires_at: i64,
    ) -> Result<()> {
        err!(EscrowError::NotImplemented)
    }

    /// Placeholder: this instruction does not transfer tokens into the vault.
    pub fn fund_escrow(_ctx: Context<FundEscrow>) -> Result<()> {
        err!(EscrowError::NotImplemented)
    }

    /// Placeholder: this instruction does not release tokens to the seller.
    pub fn release_escrow(_ctx: Context<ReleaseEscrow>) -> Result<()> {
        err!(EscrowError::NotImplemented)
    }

    /// Placeholder: this instruction does not refund tokens to the buyer.
    pub fn refund_escrow(_ctx: Context<RefundEscrow>) -> Result<()> {
        err!(EscrowError::NotImplemented)
    }

    /// Placeholder: admin dispute powers and evidence rules are not implemented.
    pub fn resolve_dispute(_ctx: Context<ResolveDispute>, _refund_buyer: bool) -> Result<()> {
        err!(EscrowError::NotImplemented)
    }

    /// Placeholder: this instruction does not enforce timeout refunds.
    pub fn refund_after_timeout(_ctx: Context<RefundAfterTimeout>) -> Result<()> {
        err!(EscrowError::NotImplemented)
    }
}

#[derive(Accounts)]
pub struct Initialize<'info> {
    #[account(mut)]
    pub administrator: Signer<'info>,
    pub accepted_mint: Account<'info, Mint>,
    #[account(
        init,
        payer = administrator,
        space = 8 + MarketplaceConfig::INIT_SPACE,
        seeds = [CONFIG_SEED],
        bump
    )]
    pub config: Account<'info, MarketplaceConfig>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
#[instruction(order_id: [u8; 32])]
pub struct CreateEscrow<'info> {
    #[account(mut)]
    pub buyer: Signer<'info>,
    pub seller: SystemAccount<'info>,
    #[account(seeds = [CONFIG_SEED], bump, has_one = accepted_mint)]
    pub config: Account<'info, MarketplaceConfig>,
    pub accepted_mint: Account<'info, Mint>,
    #[account(
        init,
        payer = buyer,
        space = 8 + Escrow::INIT_SPACE,
        seeds = [ESCROW_SEED, order_id.as_ref()],
        bump
    )]
    pub escrow: Account<'info, Escrow>,
    #[account(init, payer = buyer, associated_token::mint = accepted_mint, associated_token::authority = escrow)]
    pub vault: Account<'info, TokenAccount>,
    pub token_program: Program<'info, Token>,
    pub associated_token_program: Program<'info, AssociatedToken>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct FundEscrow<'info> {
    #[account(mut, seeds = [ESCROW_SEED, escrow.order_id.as_ref()], bump = escrow.bump, has_one = buyer, has_one = token_mint)]
    pub escrow: Account<'info, Escrow>,
    pub buyer: Signer<'info>,
    pub token_mint: Account<'info, Mint>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = escrow)]
    pub vault: Account<'info, TokenAccount>,
    #[account(mut, token::mint = token_mint, token::authority = buyer)]
    pub buyer_token_account: Account<'info, TokenAccount>,
    pub token_program: Program<'info, Token>,
}

#[derive(Accounts)]
pub struct ReleaseEscrow<'info> {
    #[account(mut, seeds = [ESCROW_SEED, escrow.order_id.as_ref()], bump = escrow.bump, has_one = buyer, has_one = seller, has_one = token_mint)]
    pub escrow: Account<'info, Escrow>,
    pub buyer: Signer<'info>,
    /// The buyer confirms receipt; the escrow program would sign for the vault.
    #[account(mut)]
    pub seller: SystemAccount<'info>,
    pub token_mint: Account<'info, Mint>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = escrow)]
    pub vault: Account<'info, TokenAccount>,
    #[account(mut, token::mint = token_mint, token::authority = seller)]
    pub seller_token_account: Account<'info, TokenAccount>,
    pub token_program: Program<'info, Token>,
}

#[derive(Accounts)]
pub struct RefundEscrow<'info> {
    #[account(mut, seeds = [ESCROW_SEED, escrow.order_id.as_ref()], bump = escrow.bump, has_one = buyer, has_one = token_mint)]
    pub escrow: Account<'info, Escrow>,
    #[account(seeds = [CONFIG_SEED], bump, has_one = administrator)]
    pub config: Account<'info, MarketplaceConfig>,
    pub buyer: SystemAccount<'info>,
    pub administrator: Signer<'info>,
    pub token_mint: Account<'info, Mint>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = escrow)]
    pub vault: Account<'info, TokenAccount>,
    #[account(mut, token::mint = token_mint, token::authority = buyer)]
    pub buyer_token_account: Account<'info, TokenAccount>,
    pub token_program: Program<'info, Token>,
}

#[derive(Accounts)]
pub struct ResolveDispute<'info> {
    #[account(mut, seeds = [ESCROW_SEED, escrow.order_id.as_ref()], bump = escrow.bump, has_one = buyer, has_one = seller, has_one = token_mint)]
    pub escrow: Account<'info, Escrow>,
    #[account(seeds = [CONFIG_SEED], bump, has_one = administrator)]
    pub config: Account<'info, MarketplaceConfig>,
    pub administrator: Signer<'info>,
    pub buyer: SystemAccount<'info>,
    pub seller: SystemAccount<'info>,
    pub token_mint: Account<'info, Mint>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = escrow)]
    pub vault: Account<'info, TokenAccount>,
    #[account(mut, token::mint = token_mint, token::authority = buyer)]
    pub buyer_token_account: Account<'info, TokenAccount>,
    #[account(mut, token::mint = token_mint, token::authority = seller)]
    pub seller_token_account: Account<'info, TokenAccount>,
    pub token_program: Program<'info, Token>,
}

#[derive(Accounts)]
pub struct RefundAfterTimeout<'info> {
    #[account(mut, seeds = [ESCROW_SEED, escrow.order_id.as_ref()], bump = escrow.bump, has_one = buyer, has_one = token_mint)]
    pub escrow: Account<'info, Escrow>,
    pub buyer: SystemAccount<'info>,
    pub token_mint: Account<'info, Mint>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = escrow)]
    pub vault: Account<'info, TokenAccount>,
    #[account(mut, token::mint = token_mint, token::authority = buyer)]
    pub buyer_token_account: Account<'info, TokenAccount>,
    pub token_program: Program<'info, Token>,
}

#[account]
#[derive(InitSpace)]
pub struct MarketplaceConfig {
    pub administrator: Pubkey,
    pub accepted_mint: Pubkey,
    /// Data layout version for future migrations.
    pub version: u8,
}

#[account]
#[derive(InitSpace)]
pub struct Escrow {
    pub order_id: [u8; 32],
    pub buyer: Pubkey,
    pub seller: Pubkey,
    pub token_mint: Pubkey,
    pub amount: u64,
    pub status: EscrowStatus,
    pub created_at: i64,
    pub expires_at: i64,
    pub bump: u8,
}

#[derive(AnchorSerialize, AnchorDeserialize, Clone, Copy, PartialEq, Eq, InitSpace)]
pub enum EscrowStatus {
    Created,
    Funded,
    Released,
    Refunded,
    Disputed,
}

#[error_code]
pub enum EscrowError {
    #[msg("The requested escrow instruction is scaffolding and is not implemented yet")]
    NotImplemented,
    #[msg("The escrow is not in the state required for this operation")]
    InvalidEscrowState,
    #[msg("The escrow deadline has not passed")]
    EscrowNotExpired,
    #[msg("The configured mint does not match the escrow mint")]
    InvalidTokenMint,
}
