-- V1__initial_schema.sql

-- Creation of pgcrypto extension for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    solana_pub_key VARCHAR(64) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Assets table
CREATE TABLE IF NOT EXISTS assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(10) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    total_shares NUMERIC(20, 9) NOT NULL,
    mint_address VARCHAR(64) UNIQUE, -- Can be NULL until tokenization
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tokens table (internal records of what was tokenized/transferred)
CREATE TABLE IF NOT EXISTS tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id UUID NOT NULL REFERENCES assets(id),
    owner_id UUID NOT NULL REFERENCES users(id),
    amount NUMERIC(20, 9) NOT NULL,
    smart_contract_rules TEXT,
    is_tradable BOOLEAN DEFAULT TRUE,
    mint_address VARCHAR(64) NOT NULL,
    token_account_address VARCHAR(64) NOT NULL,
    transaction_id VARCHAR(100) NOT NULL, -- Solana transaction signature
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for query optimization
CREATE INDEX IF NOT EXISTS idx_tokens_asset_owner ON tokens (asset_id, owner_id);
CREATE INDEX IF NOT EXISTS idx_tokens_owner_id ON tokens (owner_id);
CREATE INDEX IF NOT EXISTS idx_assets_mint_address ON assets (mint_address);