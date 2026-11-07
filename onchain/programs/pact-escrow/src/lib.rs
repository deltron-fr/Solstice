use anchor_lang::prelude::*;
use anchor_spl::associated_token::AssociatedToken;
use anchor_spl::token::{self, Mint, Token, TokenAccount, TransferChecked};

declare_id!("GDSz1GPfxqK5yioYqQRbubENibvvaZDkkcRYDzkhzG4A");

pub const CONFIG_SEED: &[u8] = b"marketplace_config";
pub const ESCROW_SEED: &[u8] = b"escrow";
pub const FUNDING_WINDOW_SECONDS: i64 = 30 * 60;
pub const FULFILLMENT_WINDOW_SECONDS: i64 = 7 * 24 * 60 * 60;

#[program]
pub mod pact_escrow {
    use super::*;

    /// Creates the singleton config PDA. The accepted mint is selected during setup.
    pub fn initialize(ctx: Context<Initialize>) -> Result<()> {
        let config = &mut ctx.accounts.config;
        config.administrator = ctx.accounts.administrator.key();
        config.accepted_mint = ctx.accounts.accepted_mint.key();
        config.version = 1;
        Ok(())
    }

    /// Creates one escrow PDA and its canonical token account for an order.
    pub fn create_escrow(
        ctx: Context<CreateEscrow>,
        order_id: [u8; 32],
        amount: u64,
        funding_expires_at: i64,
    ) -> Result<()> {
        let now = Clock::get()?.unix_timestamp;
        require!(amount > 0, EscrowError::InvalidAmount);
        require!(ctx.accounts.buyer.key() != ctx.accounts.seller.key(), EscrowError::BuyerAndSellerMustDiffer);
        require!(
            funding_expires_at > now
                && funding_expires_at <= now + FUNDING_WINDOW_SECONDS + 60,
            EscrowError::InvalidFundingDeadline
        );

        let escrow = &mut ctx.accounts.escrow;
        escrow.order_id = order_id;
        escrow.buyer = ctx.accounts.buyer.key();
        escrow.seller = ctx.accounts.seller.key();
        escrow.token_mint = ctx.accounts.accepted_mint.key();
        escrow.amount = amount;
        escrow.status = EscrowStatus::Created;
        escrow.created_at = now;
        escrow.funding_expires_at = funding_expires_at;
        escrow.funded_at = 0;
        escrow.expires_at = 0;
        escrow.bump = ctx.bumps.escrow;
        Ok(())
    }

    /// Moves the exact agreed amount from the buyer into the escrow vault.
    pub fn fund_escrow(ctx: Context<FundEscrow>) -> Result<()> {
        let now = Clock::get()?.unix_timestamp;
        let escrow = &ctx.accounts.escrow;
        require!(escrow.status == EscrowStatus::Created, EscrowError::InvalidEscrowState);
        require!(now <= escrow.funding_expires_at, EscrowError::FundingWindowExpired);

        let cpi_accounts = TransferChecked {
            from: ctx.accounts.buyer_token_account.to_account_info(),
            mint: ctx.accounts.token_mint.to_account_info(),
            to: ctx.accounts.vault.to_account_info(),
            authority: ctx.accounts.buyer.to_account_info(),
        };
        let cpi_ctx = CpiContext::new(ctx.accounts.token_program.key(), cpi_accounts);
        token::transfer_checked(cpi_ctx, escrow.amount, ctx.accounts.token_mint.decimals)?;

        let escrow = &mut ctx.accounts.escrow;
        escrow.status = EscrowStatus::Funded;
        escrow.funded_at = now;
        escrow.expires_at = now
            .checked_add(FULFILLMENT_WINDOW_SECONDS)
            .ok_or(EscrowError::TimestampOverflow)?;
        Ok(())
    }

    /// The buyer confirms receipt; the escrow PDA signs the token transfer to seller.
    pub fn release_escrow(ctx: Context<ReleaseEscrow>) -> Result<()> {
        let now = Clock::get()?.unix_timestamp;
        let escrow = &ctx.accounts.escrow;
        require!(escrow.status == EscrowStatus::Funded, EscrowError::InvalidEscrowState);
        require!(now <= escrow.expires_at, EscrowError::RefundWindowExpired);

        let order_id = escrow.order_id;
        let bump = [escrow.bump];
        let signer_seeds: &[&[u8]] = &[ESCROW_SEED, order_id.as_ref(), &bump];
        let signer = &[signer_seeds];
        let cpi_accounts = TransferChecked {
            from: ctx.accounts.vault.to_account_info(),
            mint: ctx.accounts.token_mint.to_account_info(),
            to: ctx.accounts.seller_token_account.to_account_info(),
            authority: ctx.accounts.escrow.to_account_info(),
        };
        let cpi_ctx = CpiContext::new_with_signer(
            ctx.accounts.token_program.key(),
            cpi_accounts,
            signer,
        );
        token::transfer_checked(cpi_ctx, escrow.amount, ctx.accounts.token_mint.decimals)?;
        ctx.accounts.escrow.status = EscrowStatus::Released;
        Ok(())
    }

    /// Administrator-authorized refund hook, reserved for a future dispute workflow.
    pub fn refund_escrow(_ctx: Context<RefundEscrow>) -> Result<()> {
        err!(EscrowError::NotImplemented)
    }

    /// Administrator dispute resolution is intentionally deferred from this iteration.
    pub fn resolve_dispute(_ctx: Context<ResolveDispute>, _refund_buyer: bool) -> Result<()> {
        err!(EscrowError::NotImplemented)
    }

    /// Anyone may trigger a refund once the buyer's funding or fulfillment window expires.
    pub fn refund_after_timeout(ctx: Context<RefundAfterTimeout>) -> Result<()> {
        let now = Clock::get()?.unix_timestamp;
        let escrow = &ctx.accounts.escrow;
        match escrow.status {
            EscrowStatus::Created => {
                require!(now > escrow.funding_expires_at, EscrowError::EscrowNotExpired);
            }
            EscrowStatus::Funded => {
                require!(now > escrow.expires_at, EscrowError::EscrowNotExpired);
                require!(
                    ctx.accounts.vault.amount == escrow.amount,
                    EscrowError::InvalidVaultBalance
                );
                let order_id = escrow.order_id;
                let bump = [escrow.bump];
                let signer_seeds: &[&[u8]] = &[ESCROW_SEED, order_id.as_ref(), &bump];
                let signer = &[signer_seeds];
                let cpi_accounts = TransferChecked {
                    from: ctx.accounts.vault.to_account_info(),
                    mint: ctx.accounts.token_mint.to_account_info(),
                    to: ctx.accounts.buyer_token_account.to_account_info(),
                    authority: ctx.accounts.escrow.to_account_info(),
                };
                let cpi_ctx = CpiContext::new_with_signer(
                    ctx.accounts.token_program.key(),
                    cpi_accounts,
                    signer,
                );
                token::transfer_checked(cpi_ctx, escrow.amount, ctx.accounts.token_mint.decimals)?;
            }
            _ => return err!(EscrowError::InvalidEscrowState),
        }
        ctx.accounts.escrow.status = EscrowStatus::Refunded;
        Ok(())
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
    /// CHECK: only stored as the destination wallet; no account data is read.
    pub seller: UncheckedAccount<'info>,
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
    #[account(mut)]
    pub buyer: Signer<'info>,
    /// CHECK: constrained by the escrow's stored seller key and used as the ATA owner.
    #[account(address = escrow.seller)]
    pub seller: UncheckedAccount<'info>,
    pub token_mint: Account<'info, Mint>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = escrow)]
    pub vault: Account<'info, TokenAccount>,
    #[account(init_if_needed, payer = buyer, associated_token::mint = token_mint, associated_token::authority = seller)]
    pub seller_token_account: Account<'info, TokenAccount>,
    pub associated_token_program: Program<'info, AssociatedToken>,
    pub token_program: Program<'info, Token>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct RefundEscrow<'info> {
    #[account(mut, seeds = [ESCROW_SEED, escrow.order_id.as_ref()], bump = escrow.bump, has_one = buyer, has_one = token_mint)]
    pub escrow: Account<'info, Escrow>,
    #[account(seeds = [CONFIG_SEED], bump, has_one = administrator)]
    pub config: Account<'info, MarketplaceConfig>,
    /// CHECK: constrained by the escrow's stored buyer key and used as the ATA owner.
    #[account(address = escrow.buyer)]
    pub buyer: UncheckedAccount<'info>,
    pub administrator: Signer<'info>,
    pub token_mint: Account<'info, Mint>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = escrow)]
    pub vault: Account<'info, TokenAccount>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = buyer)]
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
    /// CHECK: constrained by the escrow's stored public keys.
    #[account(address = escrow.buyer)]
    pub buyer: UncheckedAccount<'info>,
    /// CHECK: constrained by the escrow's stored public keys.
    #[account(address = escrow.seller)]
    pub seller: UncheckedAccount<'info>,
    pub token_mint: Account<'info, Mint>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = escrow)]
    pub vault: Account<'info, TokenAccount>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = buyer)]
    pub buyer_token_account: Account<'info, TokenAccount>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = seller)]
    pub seller_token_account: Account<'info, TokenAccount>,
    pub token_program: Program<'info, Token>,
}

#[derive(Accounts)]
pub struct RefundAfterTimeout<'info> {
    /// Pays transaction fees; it does not control the refund destination.
    #[account(mut)]
    pub caller: Signer<'info>,
    #[account(mut, seeds = [ESCROW_SEED, escrow.order_id.as_ref()], bump = escrow.bump, has_one = buyer, has_one = token_mint)]
    pub escrow: Account<'info, Escrow>,
    /// CHECK: constrained by the escrow's stored buyer public key.
    #[account(address = escrow.buyer)]
    pub buyer: UncheckedAccount<'info>,
    pub token_mint: Account<'info, Mint>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = escrow)]
    pub vault: Account<'info, TokenAccount>,
    #[account(mut, associated_token::mint = token_mint, associated_token::authority = buyer)]
    pub buyer_token_account: Account<'info, TokenAccount>,
    pub token_program: Program<'info, Token>,
}

#[account]
#[derive(InitSpace)]
pub struct MarketplaceConfig {
    pub administrator: Pubkey,
    pub accepted_mint: Pubkey,
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
    pub funding_expires_at: i64,
    pub funded_at: i64,
    /// Set to funded_at + FULFILLMENT_WINDOW_SECONDS when funding succeeds.
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
    #[msg("This administrator escrow instruction is not implemented yet")]
    NotImplemented,
    #[msg("The escrow is not in the state required for this operation")]
    InvalidEscrowState,
    #[msg("The escrow deadline has not passed")]
    EscrowNotExpired,
    #[msg("The escrow funding deadline has passed")]
    FundingWindowExpired,
    #[msg("The funded escrow refund deadline has passed")]
    RefundWindowExpired,
    #[msg("The escrow amount must be greater than zero")]
    InvalidAmount,
    #[msg("The funding deadline must be within the allowed order window")]
    InvalidFundingDeadline,
    #[msg("The escrow vault does not contain the exact funded amount")]
    InvalidVaultBalance,
    #[msg("Timestamp arithmetic overflowed")]
    TimestampOverflow,
    #[msg("Buyer and seller must be different wallets")]
    BuyerAndSellerMustDiffer,
}
