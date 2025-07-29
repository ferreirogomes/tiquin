package blockchain_listener

import (
	"context"
	"log"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws" // Para WebSockets
	"tokenization-backend/models"
	"tokenization-backend/storage"
)

// BlockchainListener escuta por eventos na Solana para manter o DB sincronizado.
type BlockchainListener struct {
	RPCClient  *rpc.Client
	WSClient   *ws.Client // Cliente WebSocket para subscrições
	DB         *storage.DB
	FeePayerPK solana.PrivateKey // Chave do Fee Payer para identificar transações relevantes
}

// NewBlockchainListener cria uma nova instância do listener.
func NewBlockchainListener(rpcEndpoint string, db *storage.DB, feePayerKeyBase58 string) *BlockchainListener {
	rpcClient := rpc.New(rpcEndpoint)
	wsClient, err := ws.Connect(context.Background(), rpcEndpoint) // Assume ws:// ou wss://
	if err != nil {
		log.Fatalf("Falha ao conectar ao WebSocket Solana: %v", err)
	}

	feePayer, err := solana.PrivateKeyFromBase58(feePayerKeyBase58)
	if err != nil {
		log.Fatalf("Falha ao carregar chave privada do Fee Payer para listener: %v", err)
	}

	return &BlockchainListener{
		RPCClient:  rpcClient,
		WSClient:   wsClient,
		DB:         db,
		FeePayerPK: feePayer,
	}
}

// StartListening inicia a escuta por eventos.
func (l *BlockchainListener) StartListening() {
	log.Println("Iniciando listener da blockchain...")

	// Exemplo: Subscrever a transações que envolvem o FeePayer (que cria Mints e assina transações)
	// Em um sistema real, você subscreveria a contas de tokens específicas ou a todos os Mints conhecidos.
	sub, err := l.WSClient.SignatureSubscribe(
		l.FeePayerPK.PublicKey(),
		rpc.CommitmentFinalized,
	)
	if err != nil {
		log.Printf("Falha ao subscrever a assinaturas: %v", err)
		return
	}
	defer sub.Unsubscribe()

	for {
		got, err := sub.Recv()
		if err != nil {
			log.Printf("Erro ao receber assinatura: %v", err)
			time.Sleep(5 * time.Second) // Espera antes de tentar novamente
			continue
		}

		// Apenas processa se a transação for confirmada e bem-sucedida
		if got.Value.Err == nil && got.Value.ConfirmationStatus == rpc.CommitmentFinalized {
			log.Printf("Transação confirmada (Signature: %s). Processando...", got.Value.Signature)
			l.ProcessTransaction(got.Value.Signature)
		} else if got.Value.Err != nil {
			log.Printf("Transação %s falhou: %v", got.Value.Signature, got.Value.Err)
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

	// Análise simplificada: Procure por instruções de token (MintTo, Transfer)
	for _, instruction := range txResp.Transaction.Meta.InnerInstructions {
		for _, ix := range instruction.Instructions {
			if ix.ProgramID.Equals(token.ProgramID) {
				// Tente decodificar a instrução SPL Token
				if ix.Parsed == nil {
					log.Printf("Instrução SPL Token não parseada. Dados brutos: %v", ix.Data)
					continue
				}

				// Exemplo: Detectar MintTo ou Transfer e atualizar o DB
				switch ix.Parsed.Type {
				case "mintTo":
					l.handleMintTo(signature, ix.Parsed.Info)
				case "transfer":
					l.handleTransfer(signature, ix.Parsed.Info)
				default:
					log.Printf("Instrução SPL Token não tratada: %s", ix.Parsed.Type)
				}
			}
		}
	}
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

	// Criar ou atualizar o registro de token para refletir o cunho
	// Se já existe um token para este AssetID e OwnerID, atualiza. Caso contrário, cria.
	// Uma lógica mais robusta checaria se a "cunhagem inicial" já foi registrada.
	// Por simplicidade, vamos criar um novo registro para cada 'mintTo' para demonstrar.
	tokenRecord := models.Token{
		ID:                  solana.NewRandomPrivateKey().PublicKey().String(), // ID aleatório para o registro interno
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
		log.Printf("Token cunhado (mintTo) para Asset %s, Owner %s, Amount %f, TxID %s", asset.Symbol, ownerUser.Name, tokenRecord.Amount, signature.String())
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

	source := infoMap["source"].(string)
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
	// Uma forma de fazer isso é buscar a conta de token e obter o Mint dela.
	sourceAccountInfo, err := l.RPCClient.GetAccountInfo(context.Background(), solana.MustPublicKeyFromBase58(source))
	if err != nil {
		log.Printf("Falha ao obter info da conta de origem %s: %v", source, err)
		return
	}
	var sourceTokenAccount token.Account
	err = sourceTokenAccount.UnmarshalBinary(sourceAccountInfo.Value.Data.GetBinary())
	if err != nil {
		log.Printf("Falha ao decodificar conta de origem %s: %v", source, err)
		return
	}
	mintAddress := sourceTokenAccount.Mint.String()

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
	fromOwnerPubKey := infoMap["authority"].(string) // Quem assinou a transação de transferência
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
	transferredTokenRecord := models.Token{
		ID:                  solana.NewRandomPrivateKey().PublicKey().String(),
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
			fromUser.Name, toUser.Name, asset.Symbol, transferredTokenRecord.Amount, signature.String())
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
import (
    "math/big"
)