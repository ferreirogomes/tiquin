package blockchain_listener

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ferreirogomes/tiquin/models"
	"github.com/ferreirogomes/tiquin/storage"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws" // Para WebSockets
)

// BlockchainListener escuta por eventos na Solana para manter o DB sincronizado.
type BlockchainListener struct {
	RPCClient   *rpc.Client
	RPCEndpoint string
	DB          *storage.DB
	FeePayerPK  solana.PrivateKey // Chave do Fee Payer para identificar transações relevantes
}

// NewBlockchainListener cria uma nova instância do listener.
func NewBlockchainListener(rpcEndpoint string, db *storage.DB, feePayerKeyBase58 string) *BlockchainListener {
	rpcClient := rpc.New(rpcEndpoint)

	feePayer, err := solana.PrivateKeyFromBase58(feePayerKeyBase58)
	if err != nil {
		log.Fatalf("Falha ao carregar chave privada do Fee Payer para listener: %v", err)
	}

	return &BlockchainListener{
		RPCClient:   rpcClient,
		RPCEndpoint: rpcEndpoint,
		DB:          db,
		FeePayerPK:  feePayer,
	}
}

// StartListening inicia a escuta por eventos.
func (l *BlockchainListener) StartListening() {
	log.Println("Iniciando listener da blockchain...")

	for {
		err := l.listenLoop()
		if err != nil {
			log.Printf("Listener desconectado ou erro: %v. Tentando reconectar em 5 segundos...", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func (l *BlockchainListener) listenLoop() error {
	// 1. Conectar WebSocket
	wsClient, err := ws.Connect(context.Background(), l.RPCEndpoint)
	if err != nil {
		return fmt.Errorf("falha ao conectar WebSocket: %w", err)
	}
	defer wsClient.Close()

	// 2. Backfill: Processar transações recentes que podem ter sido perdidas
	l.backfillTransactions()

	// Exemplo: Subscrever a transações que envolvem o FeePayer (que cria Mints e assina transações)
	// Em um sistema real, você subscreveria a contas de tokens específicas ou a todos os Mints conhecidos.
	sub, err := wsClient.LogsSubscribeMentions(
		l.FeePayerPK.PublicKey(),
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return fmt.Errorf("falha ao subscrever a logs: %w", err)
	}
	defer sub.Unsubscribe()

	log.Println("Escutando por novas transações (logs)...")
	for {
		got, err := sub.Recv(context.Background())
		if err != nil {
			return fmt.Errorf("erro ao receber log: %w", err)
		}

		// Apenas processa se não houve erro relatado no log da transação
		if got.Value.Err == nil {
			log.Printf("Transação com FeePayer detectada (Signature: %s). Processando...", got.Value.Signature)
			l.ProcessTransaction(got.Value.Signature)
		} else {
			log.Printf("Transação %s falhou no log: %v", got.Value.Signature, got.Value.Err)
		}
	}
}

// backfillTransactions busca as últimas transações para garantir que nada foi perdido.
func (l *BlockchainListener) backfillTransactions() {
	log.Println("Executando backfill de transações...")
	limit := 50 // Buscar as últimas 50 transações
	signatures, err := l.RPCClient.GetSignaturesForAddressWithOpts(
		context.Background(),
		l.FeePayerPK.PublicKey(),
		&rpc.GetSignaturesForAddressOpts{Limit: &limit, Commitment: rpc.CommitmentFinalized},
	)
	if err != nil {
		log.Printf("Erro ao buscar histórico de transações para backfill: %v", err)
		return
	}

	// Processar do mais antigo para o mais novo para manter a ordem cronológica
	for i := len(signatures) - 1; i >= 0; i-- {
		sig := signatures[i]
		if sig.Err == nil {
			l.ProcessTransaction(sig.Signature)
		}
	}
}

// ProcessTransaction busca os detalhes de uma transação e atualiza o DB.
func (l *BlockchainListener) ProcessTransaction(signature solana.Signature) {
	log.Printf("Buscando detalhes da transação %s...", signature.String())

	// Obter detalhes completos da transação
	txResp, err := l.RPCClient.GetTransaction(context.Background(), signature, &rpc.GetTransactionOpts{
		Commitment: rpc.CommitmentFinalized,
		Encoding:   solana.EncodingJSONParsed, // Para obter detalhes de token
	})
	if err != nil {
		log.Printf("Falha ao obter detalhes da transação %s: %v", signature.String(), err)
		return
	}
	if txResp == nil || txResp.Transaction == nil {
		log.Printf("Detalhes da transação %s vazios.", signature.String())
		return
	}

	// Para buscar logs e transferências mais simples, em aplicações reais, usa-se sub.LogsSubscribe,
	// e depois extrai-se a transação com GetTransaction como já feito.

	log.Printf("Processamento avançado de parse de transação omitido na PoC para %s", signature.String())
}

// handleMintTo processa uma instrução MintTo.
func (l *BlockchainListener) handleMintTo(signature solana.Signature, info interface{}) {
	log.Println("Instrução 'mintTo' detectada.")
	// O 'info' é um mapa de interface. Precisamos fazer o type assertion.
	infoMap, ok := info.(map[string]interface{})
	if !ok {
		log.Println("Falha ao converter info para mapa para 'mintTo'.")
		return
	}

	mint := infoMap["mint"].(string)
	account := infoMap["account"].(string)
	amountFloat, ok := infoMap["amount"].(float64) // Pode vir como float ou string
	if !ok {
		amountStr, ok := infoMap["amount"].(string)
		if ok {
			var err error
			amountFloat, err = parseAmountFromString(amountStr)
			if err != nil {
				log.Printf("Falha ao parsear 'amount' de string para float: %v", err)
				return
			}
		} else {
			log.Println("Campo 'amount' em 'mintTo' não é float64 nem string.")
			return
		}
	}

	// Buscar o asset pelo mint_address para obter o asset.ID
	asset, foundAsset, err := l.DB.GetAssetByMintAddress(mint)
	if err != nil {
		log.Printf("Erro ao buscar ativo por MintAddress %s: %v", mint, err)
		return
	}
	if !foundAsset {
		log.Printf("Ativo para MintAddress %s não encontrado no DB interno. Ignorando.", mint)
		return
	}

	// Tentar encontrar o usuário dono da `account` (ATA)
	ownerUser, foundUser, err := l.DB.GetUserBySolanaPubKey(infoMap["owner"].(string))
	if err != nil {
		log.Printf("Erro ao buscar proprietário por SolanaPubKey %s: %v", infoMap["owner"].(string), err)
		return
	}
	if !foundUser {
		log.Printf("Usuário proprietário para ATA %s não encontrado no DB interno. Pode ser uma ATA de um usuário externo.", account)
		// Você pode decidir criar um registro para usuários externos ou ignorar.
		// Por simplicidade, vamos pular se o usuário não for interno.
		return
	}

	// Idempotência: Verificar se já processamos esta transação
	txExists, err := l.DB.TransactionExists(signature.String())
	if err != nil {
		log.Printf("Erro ao verificar idempotência da transação no banco: %v", err)
		return
	}
	if txExists {
		log.Printf("Transação %s já processada para MintTo. Ignorando.", signature.String())
		return
	}

	// Criar ou atualizar o registro de token para refletir o cunho
	tokenID, _ := solana.NewRandomPrivateKey()
	tokenRecord := models.Token{
		ID:                  tokenID.PublicKey().String(), // ID aleatório para o registro interno
		AssetID:             asset.ID,
		OwnerID:             ownerUser.ID,
		Amount:              amountFloat / 1e9, // Converter de volta de unidades atômicas (se 9 decimais)
		SmartContractRules:  asset.Name + " rules",
		IsTradable:          true,
		MintAddress:         mint,
		TokenAccountAddress: account,
		TransactionID:       signature.String(),
		CreatedAt:           time.Now(),
	}
	if err := l.DB.SaveToken(tokenRecord); err != nil { // SaveToken faz ON CONFLICT UPDATE
		log.Printf("Falha ao salvar/atualizar registro de token para MintTo %s: %v", signature.String(), err)
	} else {
		log.Printf("Token cunhado (mintTo) para Asset %s, OwnerID %s, Amount %f, TxID %s", asset.Symbol, ownerUser.ID, tokenRecord.Amount, signature.String())
	}
}

// handleTransfer processa uma instrução Transfer.
func (l *BlockchainListener) handleTransfer(signature solana.Signature, info interface{}) {
	log.Println("Instrução 'transfer' detectada.")
	infoMap, ok := info.(map[string]interface{})
	if !ok {
		log.Println("Falha ao converter info para mapa para 'transfer'.")
		return
	}

	destination := infoMap["destination"].(string)
	amountFloat, ok := infoMap["amount"].(float64)
	if !ok {
		amountStr, ok := infoMap["amount"].(string)
		if ok {
			var err error
			amountFloat, err = parseAmountFromString(amountStr)
			if err != nil {
				log.Printf("Falha ao parsear 'amount' de string para float: %v", err)
				return
			}
		} else {
			log.Println("Campo 'amount' em 'transfer' não é float64 nem string.")
			return
		}
	}

	// Para uma transferência, precisaríamos identificar o MintAddress do token transferido.
	// Isso pode ser obtido buscando a conta 'source' ou 'destination' e vendo seu 'mint'.
	// Para simplificar, vamos assumir que a informação do mint está disponível via a conta de token.
	mintAddress := "MockMintAddress"

	asset, foundAsset, err := l.DB.GetAssetByMintAddress(mintAddress)
	if err != nil {
		log.Printf("Erro ao buscar ativo por MintAddress %s: %v", mintAddress, err)
		return
	}
	if !foundAsset {
		log.Printf("Ativo para MintAddress %s não encontrado no DB interno. Ignorando transferência.", mintAddress)
		return
	}

	// Identificar remetente e destinatário no nosso DB
	// Isso é um pouco complexo, pois GetTokenAccountsByOwner retorna ATAs, não diretamente usuários.
	// A melhor forma é buscar o owner da ATA na própria Solana, e então mapear para nosso DB.
	fromOwnerPubKey := infoMap["authority"].(string)      // Quem assinou a transação de transferência
	toOwnerPubKey := infoMap["destinationOwner"].(string) // Quem é o owner da conta de destino

	fromUser, foundFromUser, err := l.DB.GetUserBySolanaPubKey(fromOwnerPubKey)
	if err != nil {
		log.Printf("Erro ao buscar usuário remetente por SolanaPubKey %s: %v", fromOwnerPubKey, err)
		return
	}
	toUser, foundToUser, err := l.DB.GetUserBySolanaPubKey(toOwnerPubKey)
	if err != nil {
		log.Printf("Erro ao buscar usuário destinatário por SolanaPubKey %s: %v", toOwnerPubKey, err)
		return
	}

	if !foundFromUser || !foundToUser {
		log.Printf("Remetente ou destinatário (ou ambos) não encontrados no DB interno para TxID %s. De %s para %s. Ignorando.",
			signature.String(), fromOwnerPubKey, toOwnerPubKey)
		return
	}

	// Idempotência: Verificar se já processamos esta transação
	txExists, err := l.DB.TransactionExists(signature.String())
	if err != nil {
		log.Printf("Erro ao verificar idempotência da transação no banco: %v", err)
		return
	}
	if txExists {
		log.Printf("Transação %s já processada para Transfer. Ignorando.", signature.String())
		return
	}

	// Agora que identificamos os usuários internos, podemos atualizar o banco de dados.
	// Em um sistema real, você não "cria um novo token" para cada transferência.
	// Você atualiza o saldo de tokens que um usuário possui.
	// Para este exemplo, vamos simplificar a atualização dos registros.

	// Lógica para atualizar a posse:
	// 1. Encontrar o registro de token "original" associado ao asset_id e fromUser.ID
	//    e subtrair a quantidade.
	// 2. Encontrar o registro de token "original" associado ao asset_id e toUser.ID
	//    e adicionar a quantidade.
	// Isso exigiria métodos no storage.DB para buscar e atualizar saldos por (AssetID, OwnerID).

	// Exemplo simplificado de como registrar a transferência para fins de histórico interno:
	// Cria um novo registro de token que representa a transferência para o novo proprietário.
	// Em produção, a lógica seria mais complexa para gerenciar saldos por usuário/ativo.
	tokenID, _ := solana.NewRandomPrivateKey()
	transferredTokenRecord := models.Token{
		ID:                  tokenID.PublicKey().String(),
		AssetID:             asset.ID,
		OwnerID:             toUser.ID, // O novo proprietário
		Amount:              amountFloat / 1e9,
		SmartContractRules:  "Transferido via blockchain",
		IsTradable:          true,
		MintAddress:         mintAddress,
		TokenAccountAddress: destination, // A conta de destino
		TransactionID:       signature.String(),
		CreatedAt:           time.Now(),
	}

	if err := l.DB.SaveToken(transferredTokenRecord); err != nil {
		log.Printf("Falha ao salvar registro de token para Transferência %s: %v", signature.String(), err)
	} else {
		log.Printf("Token transferido (transfer) de %s para %s. Asset: %s, Amount: %f, TxID: %s",
			fromUser.ID, toUser.ID, asset.Symbol, transferredTokenRecord.Amount, signature.String())
	}
}

// parseAmountFromString tenta parsear um valor de string para float64.
// Útil porque o campo 'amount' pode vir como string em algumas instruções RPC.
func parseAmountFromString(s string) (float64, error) {
	var f big.Float
	_, _, err := f.Parse(s, 10)
	if err != nil {
		return 0, fmt.Errorf("falha ao parsear string para float: %w", err)
	}
	val, _ := f.Float64()
	return val, nil
}

// ... Outras funções auxiliares se necessário ...

// Para o parserAmountFromString
//import (
//    "math/big"
//)
