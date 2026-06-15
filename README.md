# Tiquin
A solution for a small money bucket

# Asset Tokenization Backend with Go and Solana

This project is a demonstration backend for asset tokenization (such as fractional shares) on the Solana blockchain, built in Go. It uses PostgreSQL for internal data persistence, implements a user-signed transaction flow (with the private key remaining in the user's wallet), and includes a blockchain event listener for data synchronization. The project is dockerized to simplify setup and development.

## Features

* **User Management:** Creation and retrieval of users with their Solana public keys.
* **Asset Tokenization:** Creation of new assets (e.g., company shares) represented as SPL tokens on Solana.
* **Token Transfer:** A two-step flow where the backend prepares the transaction and the frontend (simulated in tests) signs it with the user's private key.
* **Blockchain Listener:** A background service that listens for events on Solana to keep the internal database synchronized with the on-chain token state.
* **Data Persistence:** Uses PostgreSQL to store information about users, assets, and tokens.
* **Database Migrations:** Schema management with Flyway (via `sql-migrate`).
* **Dockerization:** Isolated development and testing environment with Docker Compose.
* **Tests:** Unit and integration tests.

## Technologies Used

* **Go (Golang):** Backend programming language.
* **Solana:** Blockchain for asset tokenization and registration.
* **SPL Token Program:** Solana's fungible token standard.
* **`github.com/gagliardetto/solana-go`:** Go SDK for interacting with Solana.
* **PostgreSQL:** Relational database.
* **`github.com/jmoiron/sqlx`:** Extension for the `database/sql` package to simplify struct mapping.
* **`github.com/lib/pq`:** PostgreSQL driver for Go.
* **`github.com/rubenv/sql-migrate`:** Database migration manager (Flyway standard).
* **`github.com/go-chi/chi`:** Lightweight and modular HTTP router for Go.
* **`github.com/stretchr/testify`:** Testing toolkit for Go.
* **Docker & Docker Compose:** For containerization and service orchestration.

## Prerequisites

* [**Docker Desktop**](https://www.docker.com/products/docker-desktop) (or Docker Engine and Docker Compose installed)
* [**Go**](https://golang.org/doc/install) (version 1.22 or higher)
* **Solana CLI** (optional, for generating test keys and airdropping SOL on devnet):
    `sh -c "$(curl -sSfL https://raw.githubusercontent.com/solana-labs/solana/master/install/install-init.sh)"`

## Environment Setup

1.  **Clone the Repository:**
    ```bash
    git clone [https://github.com/your-username/tokenization-backend.git](https://github.com/your-username/tokenization-backend.git)
    cd tokenization-backend
    ```

2.  **Create and Configure the `.env` File:**
    At the project root, create a file named `.env` and fill it with your configuration:

    ```ini
    DB_NAME=tokenizationdb
    DB_USER=user
    DB_PASSWORD=password
    SOLANA_RPC_URL=[https://api.devnet.solana.com](https://api.devnet.solana.com)
    SOLANA_FEE_PAYER_PRIVATE_KEY=YOUR_BASE58_PRIVATE_KEY_HERE
    ```

    * `DB_NAME`, `DB_USER`, `DB_PASSWORD`: Credentials for your PostgreSQL database.
    * `SOLANA_RPC_URL`: URL for the Solana RPC endpoint. For development, `https://api.devnet.solana.com` is recommended. For production, use a dedicated RPC provider (QuickNode, Alchemy, Helius).
    * `SOLANA_FEE_PAYER_PRIVATE_KEY`: **CRITICAL!** This is the private key (in Base58 format) of a Solana wallet that your backend will use to pay gas fees (`SOL`) and act as the `Mint Authority` (who can mint new tokens).
        * **For Testing:** You can generate a new key with the Solana CLI: `solana-keygen new --no-passphrase`. After creation, use `solana-keygen pubkey <path_to_your_keypair.json> --with-private-key` to get the private key in Base58.
        * **Fund the Wallet:** Send some SOL to the public address of this key using a devnet faucet (e.g., `solana airdrop 10`).
        * **SECURITY:** **Never use a real production private key directly in `.env`!** For production, use a secrets management service (AWS Secrets Manager, HashiCorp Vault) or an HSM.

3.  **Install Go Dependencies:**
    ```bash
    go mod tidy
    ```

## Running the Application (Development)

To start the backend and PostgreSQL using Docker Compose:

```bash
docker-compose up --build
