-- V1__initial_schema.sql

-- Criação da extensão pgcrypto para gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Tabela de usuários
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    solana_pub_key VARCHAR(64) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabela de ativos
CREATE TABLE IF NOT EXISTS assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(10) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    total_shares NUMERIC(20, 9) NOT NULL,
    mint_address VARCHAR(64) UNIQUE, -- Pode ser NULL até a tokenização
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabela de tokens (registros internos do que foi tokenizado/transferido)
CREATE TABLE IF NOT EXISTS tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id UUID NOT NULL REFERENCES assets(id),
    owner_id UUID NOT NULL REFERENCES users(id),
    amount NUMERIC(20, 9) NOT NULL,
    smart_contract_rules TEXT,
    is_tradable BOOLEAN DEFAULT TRUE,
    mint_address VARCHAR(64) NOT NULL,
    token_account_address VARCHAR(64) NOT NULL,
    transaction_id VARCHAR(100) NOT NULL, -- Signature da transação Solana
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Índices para otimização de consultas
CREATE INDEX IF NOT EXISTS idx_tokens_asset_owner ON tokens (asset_id, owner_id);
CREATE INDEX IF NOT EXISTS idx_tokens_owner_id ON tokens (owner_id);
CREATE INDEX IF NOT EXISTS idx_assets_mint_address ON assets (mint_address);