CREATE TABLE IF NOT EXISTS accounts (
    id VARCHAR(36) PRIMARY KEY,
    owner_name VARCHAR(255) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    balance NUMERIC(18, 4) NOT NULL DEFAULT 0.0000,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS exchange_rates (
    id VARCHAR(36) PRIMARY KEY,
    from_currency VARCHAR(10) NOT NULL,
    to_currency VARCHAR(10) NOT NULL,
    rate NUMERIC(18, 6) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_currency_pair UNIQUE (from_currency, to_currency)
);

CREATE TABLE IF NOT EXISTS quotes (
    id VARCHAR(36) PRIMARY KEY,
    from_currency VARCHAR(10) NOT NULL,
    to_currency VARCHAR(10) NOT NULL,
    rate NUMERIC(18, 6) NOT NULL,
    from_amount NUMERIC(18, 4) NOT NULL,
    to_amount NUMERIC(18, 4) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'LOCKED', -- LOCKED, EXPIRED, USED
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS transactions (
    id VARCHAR(36) PRIMARY KEY,
    sender_account_id VARCHAR(36) NOT NULL REFERENCES accounts(id),
    receiver_account_id VARCHAR(36) NOT NULL REFERENCES accounts(id),
    quote_id VARCHAR(36) REFERENCES quotes(id),
    from_amount NUMERIC(18, 4) NOT NULL,
    to_amount NUMERIC(18, 4) NOT NULL,
    from_currency VARCHAR(10) NOT NULL,
    to_currency VARCHAR(10) NOT NULL,
    status VARCHAR(20) NOT NULL, -- PENDING, COMPLETED, FAILED
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ledger (
    id VARCHAR(36) PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL REFERENCES transactions(id),
    account_id VARCHAR(36) NOT NULL REFERENCES accounts(id),
    entry_type VARCHAR(10) NOT NULL, -- DEBIT, CREDIT
    amount NUMERIC(18, 4) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for query performance
CREATE INDEX IF NOT EXISTS idx_accounts_owner ON accounts(owner_name);
CREATE INDEX IF NOT EXISTS idx_transactions_sender ON transactions(sender_account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_receiver ON transactions(receiver_account_id);
CREATE INDEX IF NOT EXISTS idx_ledger_transaction ON ledger(transaction_id);
CREATE INDEX IF NOT EXISTS idx_ledger_account ON ledger(account_id);

-- Seed initial exchange rates
INSERT INTO exchange_rates (id, from_currency, to_currency, rate, updated_at)
VALUES 
    (gen_random_uuid()::text, 'USD', 'EUR', 0.920000, CURRENT_TIMESTAMP),
    (gen_random_uuid()::text, 'EUR', 'USD', 1.086957, CURRENT_TIMESTAMP),
    (gen_random_uuid()::text, 'USD', 'GBP', 0.790000, CURRENT_TIMESTAMP),
    (gen_random_uuid()::text, 'GBP', 'USD', 1.265823, CURRENT_TIMESTAMP),
    (gen_random_uuid()::text, 'USD', 'JPY', 155.500000, CURRENT_TIMESTAMP),
    (gen_random_uuid()::text, 'JPY', 'USD', 0.006431, CURRENT_TIMESTAMP),
    (gen_random_uuid()::text, 'EUR', 'GBP', 0.858696, CURRENT_TIMESTAMP),
    (gen_random_uuid()::text, 'GBP', 'EUR', 1.164557, CURRENT_TIMESTAMP),
    (gen_random_uuid()::text, 'USD', 'USD', 1.000000, CURRENT_TIMESTAMP),
    (gen_random_uuid()::text, 'EUR', 'EUR', 1.000000, CURRENT_TIMESTAMP),
    (gen_random_uuid()::text, 'GBP', 'GBP', 1.000000, CURRENT_TIMESTAMP)
ON CONFLICT (from_currency, to_currency) DO UPDATE SET rate = EXCLUDED.rate;
