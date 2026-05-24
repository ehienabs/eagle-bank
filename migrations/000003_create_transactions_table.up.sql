-- Migration: Create transactions table
-- Description: Financial transactions (deposits, withdrawals)

CREATE TABLE IF NOT EXISTS transactions (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) NOT NULL REFERENCES accounts(id),
    user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    type VARCHAR(20) NOT NULL,
    amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'GBP',
    reference VARCHAR(255),
    description TEXT,
    balance_before BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_transaction_type CHECK (type IN ('deposit', 'withdrawal')),
    CONSTRAINT chk_transaction_amount CHECK (amount > 0),
    CONSTRAINT chk_currency CHECK (currency IN ('GBP')),
    CONSTRAINT chk_transaction_status CHECK (status IN ('pending', 'completed', 'failed'))
);

-- Indexes for common queries
CREATE INDEX idx_transactions_account_id ON transactions(account_id);
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_type ON transactions(type);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);
CREATE INDEX idx_transactions_reference ON transactions(reference) WHERE reference IS NOT NULL;

-- Composite index for account transaction history
CREATE INDEX idx_transactions_account_created ON transactions(account_id, created_at DESC);

-- Trigger for updated_at
CREATE TRIGGER update_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE transactions IS 'Financial transactions for accounts';
COMMENT ON COLUMN transactions.amount IS 'Transaction amount in cents';
COMMENT ON COLUMN transactions.balance_before IS 'Account balance before transaction in cents';
COMMENT ON COLUMN transactions.balance_after IS 'Account balance after transaction in cents';
