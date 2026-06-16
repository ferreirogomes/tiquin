-- V2__idempotency_and_balances.sql
-- Fix idempotency: add UNIQUE on transaction_id
-- Add balance tracking per (asset_id, owner_id) for transfer debit/credit

-- Ensure transaction_id is unique so ON CONFLICT works correctly
ALTER TABLE tokens ADD CONSTRAINT tokens_transaction_id_unique UNIQUE (transaction_id);

-- API keys table for authentication
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256 hash of the raw key
    description VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP WITH TIME ZONE
);

-- Index for fast key lookups
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys (key_hash);
