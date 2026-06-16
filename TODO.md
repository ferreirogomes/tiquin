# TODO: Tiquin — Asset Tokenization Backend (Go + Solana)
# Internet Capital Markets — Brazilian RWA Platform

---

## 🔴 Priority Fixes (from qwen3.6 architectural review)

- [x] P1 — Implement real `GetTokenAccountBalance` on Solana RPC (balance check was always returning 0)
- [x] P2 — Wire `handleMintTo` / `handleTransfer` into `ProcessTransaction` + fix `MockMintAddress`
- [x] P3 — Fix transfer balance accounting (debit sender, credit recipient in DB)
- [x] P4 — Add API-key auth middleware to all routes
- [x] P5 — Implement `CreateMintAndTokenAccount` + `MintTokensToAccount` on Solana
- [x] QW1 — Fix `context.WithTimeout` cancel leak in `main.go`
- [x] QW2 — Add `UNIQUE` constraint on `tokens.transaction_id` (migration V2)
- [x] QW3 — Add stop channel to `BlockchainListener` for graceful shutdown on SIGTERM
- [x] QW4 — Add `db_balance` column to tokens table for sender debit tracking

---

## 🟡 Next: ANBIMA / Drex Adaptation (Innovation Network)

1. **Interoperability (Solana ↔ EVM/Drex)**
   - Expand `blockchain_listener` to listen for cross-chain events or interact with bridges connecting Solana to the EVM environment (Hyperledger Besu) of Drex.

2. **Multi-chain Support / EVM Compatibility**
   - Evaluate abstracting the SPL Token module to support EVM standards (ERC-20, ERC-1155, ERC-3643).
   - Allow original assets to be settled (DvP) on the Drex infrastructure if required by regulation.

3. **Identity Management and Permissioning (KYC/AML - Open Finance)**
   - Incorporate a digital identity validation layer into the user creation flow.
   - Ensure that participating wallets are authorized by financial institutions or regulated nodes.

4. **On-Chain Programmability and Compliance (CVM)**
   - Implement compliance rules and programmatic restrictions in contracts (e.g., tax withholding, transfer block to unverified wallets).
   - Replace discretionary processes with deterministic rules based on Smart Contracts/Tokens.

---

## 🟢 Recommended Wallet for Development & Testing

### ✅ PRIMARY: Solflare (https://www.solflare.com/)
**Why Solflare for tiquin:**
- Solana-native (built exclusively for Solana, unlike Phantom/MetaMask)
- Supports Token Extensions (SPL Token-2022) — required for compliance rules (CVM transfer restrictions)
- Non-custodial / self-custodial: matches tiquin's user-signs-tx flow
- Browser extension + mobile → perfect for testing the `PrepareTransfer` → frontend signs → `CompleteTransfer` flow
- Built-in devnet support (switch networks easily for testing)
- Supports hardware wallet (Solflare Shield) for FeePayer key management in production

### 🏦 INSTITUTIONAL: Crossmint Custodial (https://www.crossmint.com/)
**Why for institutional users of tiquin:**
- Enterprise custodial wallet-as-a-service with MPC key management
- Built-in compliance tools — aligns with CVM/ANBIMA requirements
- API-first: integrates directly into tiquin's backend without requiring users to manage wallets
- Ideal for onboarding institutional investors who won't manage their own keys

### 🔧 FEE PAYER KEY: Keystone Hardware Wallet (https://keyst.one/)
**Why for the FeePayer:**
- Air-gapped, QR-code signing — never exposes private key to network
- Open-source firmware
- Ideal for securing the FeePayer/MintAuthority key in a production deployment

---

## 🔵 Next Steps (Wallet)

- [ ] Install Solflare browser extension
- [ ] Generate a Solana wallet for testing (`solana-keygen new` OR via Solflare UI)
- [ ] Switch to Devnet in Solflare settings
- [ ] Request a devnet SOL airdrop: `solana airdrop 10 <YOUR_PUBKEY> --url devnet`
- [ ] Export the FeePayer Base58 private key and add to `.env_true`
- [ ] Test the full flow: Create Asset → Mint → PrepareTransfer (Solflare signs) → CompleteTransfer
