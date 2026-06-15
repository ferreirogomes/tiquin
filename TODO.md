# TODO: Tiquin ↔ ANBIMA Adaptation (Innovation Network / Drex)

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
### Next Steps (Solana Wallet)
- [ ] Generate a Solana wallet for testing (`solana-keygen new`).
- [ ] Obtain the Base58 private key.
- [ ] Request a devnet SOL airdrop.
- [ ] Fill in the credentials in the local `.env_true` file.
