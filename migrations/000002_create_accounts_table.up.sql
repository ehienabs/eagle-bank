-- Migration: Create accounts table
-- Description: Bank accounts linked to users

CREATE TABLE IF NOT EXISTS accounts (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    account_number VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    account_type VARCHAR(20) NOT NULL DEFAULT 'personal',
    sort_code VARCHAR(10) NOT NULL DEFAULT '10-10-10',
    currency VARCHAR(3) NOT NULL DEFAULT 'GBP',
    balance BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INTEGER NOT NULL DEFAULT 1,

    CONSTRAINT chk_balance_non_negative CHECK (balance >= 0),
    CONSTRAINT chk_account_type CHECK (account_type IN ('personal')),
    CONSTRAINT chk_currency CHECK (currency IN ('GBP'))
);

-- Indexes for common queries
CREATE INDEX idx_accounts_user_id ON accounts(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_accounts_account_number ON accounts(account_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_accounts_created_at ON accounts(created_at);
CREATE INDEX idx_accounts_deleted_at ON accounts(deleted_at) WHERE deleted_at IS NOT NULL;

-- Trigger for updated_at
CREATE TRIGGER update_accounts_updated_at
    BEFORE UPDATE ON accounts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE accounts IS 'Bank accounts belonging to users';
COMMENT ON COLUMN accounts.balance IS 'Balance stored in cents to avoid floating point issues';
COMMENT ON COLUMN accounts.account_number IS 'Unique 8-digit account number starting with 01';
COMMENT ON COLUMN accounts.version IS 'Optimistic locking version number for concurrent updates';
