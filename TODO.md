# TODO: Adequação Tiquin ↔ ANBIMA (Rede de Inovação / Drex)

1. **Interoperabilidade (Solana ↔ EVM/Drex)**
   - Expandir `blockchain_listener` para escutar eventos cross-chain ou interagir com bridges que conectem a Solana ao ambiente EVM (Hyperledger Besu) do Drex.

2. **Suporte Multi-chain / Compatibilidade EVM**
   - Avaliar a abstração do módulo SPL Token para suportar padrões EVM (ERC-20, ERC-1155, ERC-3643).
   - Permitir que os ativos originais possam ser liquidados (DvP) na infraestrutura do Drex se exigido pela regulação.

3. **Gestão de Identidade e Permissonamento (KYC/AML - Open Finance)**
   - Incorporar uma camada de validação e identidade digital ao fluxo de criação de usuários.
   - Garantir que as carteiras (wallets) participantes sejam autorizadas por instituições financeiras ou nós regulamentados.

4. **Programabilidade e Compliance On-Chain (CVM)**
   - Implementar regras de *compliance* e bloqueios programáticos nos contratos (ex: retenção de impostos, bloqueio de transferência para carteiras não verificadas).
   - Substituir processos discricionários por regras determinísticas baseadas em Smart Contracts/Tokens.

5. **Participação na Rede ANBIMA de Inovação**
   - Cadastrar o projeto Tiquin (como um caso de uso para tokenização fracionada em blockchain) na "Discovery IA" ou nas trilhas direcionadas ao mercado.

---
### Próximos Passos (Carteira Solana)
- [ ] Gerar carteira Solana para testes (`solana-keygen new`).
- [ ] Obter chave privada base58.
- [ ] Solicitar airdrop de devnet SOL.
- [ ] Preencher as credenciais no arquivo local `.env_true`.
