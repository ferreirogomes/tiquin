# Tiquin
Uma solução para um bucadin de dinherim 

# Backend de Tokenização de Ativos com Go e Solana

Este projeto é um backend de demonstração para tokenização de ativos (como ações fracionadas) na blockchain Solana, construído em Go. Ele utiliza PostgreSQL para persistência de dados internos, implementa um fluxo de assinatura de transações pelo usuário (com a chave privada ficando na carteira do usuário), e inclui um listener para eventos da blockchain para sincronização de dados. O projeto é dockerizado para facilitar a configuração e o desenvolvimento.

## Funcionalidades

* **Gerenciamento de Usuários:** Criação e consulta de usuários com suas chaves públicas Solana.
* **Tokenização de Ativos:** Criação de novos ativos (ex: ações de empresas) que são representados por tokens SPL na Solana.
* **Transferência de Tokens:** Fluxo de duas etapas onde o backend prepara a transação e o frontend (simulado nos testes) a assina com a chave privada do usuário.
* **Listener da Blockchain:** Um serviço em segundo plano que escuta eventos na Solana para manter o banco de dados interno sincronizado com o estado on-chain dos tokens.
* **Persistência de Dados:** Utiliza PostgreSQL para armazenar informações sobre usuários, ativos e tokens.
* **Migrações de Banco de Dados:** Gerenciamento de schema com Flyway (via `sql-migrate`).
* **Dockerização:** Ambiente de desenvolvimento e teste isolado com Docker Compose.
* **Testes:** Testes de unidade e integração.

## Tecnologias Utilizadas

* **Go (Golang):** Linguagem de programação do backend.
* **Solana:** Blockchain para a tokenização e registro de ativos.
* **SPL Token Program:** Padrão de token fungível da Solana.
* **`github.com/gagliardetto/solana-go`:** SDK Go para interagir com a Solana.
* **PostgreSQL:** Banco de dados relacional.
* **`github.com/jmoiron/sqlx`:** Extensão para o pacote `database/sql` para facilitar o mapeamento de structs.
* **`github.com/lib/pq`:** Driver PostgreSQL para Go.
* **`github.com/rubenv/sql-migrate`:** Gerenciador de migrações de banco de dados (padrão Flyway).
* **`github.com/go-chi/chi`:** Roteador HTTP leve e modular para Go.
* **`github.com/stretchr/testify`:** Toolkit de teste para Go.
* **Docker & Docker Compose:** Para conteinerização e orquestração de serviços.

## Pré-requisitos

* [**Docker Desktop**](https://www.docker.com/products/docker-desktop) (ou Docker Engine e Docker Compose instalados)
* [**Go**](https://golang.org/doc/install) (versão 1.22 ou superior)
* **Solana CLI** (opcional, para gerar chaves de teste e fazer airdrop de SOL na devnet):
    `sh -c "$(curl -sSfL https://raw.githubusercontent.com/solana-labs/solana/master/install/install-init.sh)"`

## Configuração do Ambiente

1.  **Clone o Repositório:**
    ```bash
    git clone [https://github.com/seu-usuario/tokenization-backend.git](https://github.com/seu-usuario/tokenization-backend.git)
    cd tokenization-backend
    ```

2.  **Crie e Configure o Arquivo `.env`:**
    Na raiz do projeto, crie um arquivo chamado `.env` e preencha-o com suas configurações:

    ```ini
    DB_NAME=tokenizationdb
    DB_USER=user
    DB_PASSWORD=password
    SOLANA_RPC_URL=[https://api.devnet.solana.com](https://api.devnet.solana.com)
    SOLANA_FEE_PAYER_PRIVATE_KEY=SUA_CHAVE_PRIVADA_BASE58_AQUI
    ```

    * `DB_NAME`, `DB_USER`, `DB_PASSWORD`: Credenciais para seu banco de dados PostgreSQL.
    * `SOLANA_RPC_URL`: URL do endpoint RPC da Solana. Para desenvolvimento, `https://api.devnet.solana.com` é recomendado. Para produção, use um provedor RPC dedicado (QuickNode, Alchemy, Helius).
    * `SOLANA_FEE_PAYER_PRIVATE_KEY`: **CRÍTICO!** Esta é a chave privada (em formato Base58) de uma carteira Solana que seu backend usará para pagar as taxas de gás (`SOL`) e agir como `Mint Authority` (quem pode cunhar novos tokens).
        * **Para Testes:** Você pode gerar uma nova chave com a Solana CLI: `solana-keygen new --no-passphrase`. Após a criação, use `solana-keygen pubkey <path_to_your_keypair.json> --with-private-key` para obter a chave privada em Base58.
        * **Financie a Carteira:** Envie alguns SOL para o endereço público dessa chave usando uma faucet da devnet (ex: `solana airdrop 10`).
        * **SEGURANÇA:** **Nunca use uma chave privada real de produção diretamente em `.env`!** Para produção, utilize um serviço de gerenciamento de segredos (AWS Secrets Manager, HashiCorp Vault) ou um HSM.

3.  **Instale as Dependências Go:**
    ```bash
    go mod tidy
    ```

## Executando a Aplicação (Desenvolvimento)

Para iniciar o backend e o PostgreSQL usando Docker Compose:

```bash
docker-compose up --build
