# Estágio de build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copia os arquivos de go.mod e go.sum para gerenciar as dependências
COPY go.mod ./
COPY go.sum ./

# Baixa as dependências
RUN go mod download

# Copia o restante do código fonte
COPY . .

# Constrói o executável estaticamente
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Estágio final (imagem menor)
FROM alpine:latest

WORKDIR /root/

# Copia o executável do estágio de build
COPY --from=builder /app/main .

# Copia o certificado SSL CA para Alpine para que o http.Client possa verificar certificados
# Útil para conexões SSL com APIs RPC da Solana.
RUN apk --no-cache add ca-certificates

# Expõe a porta que o aplicativo Go irá escutar
EXPOSE 8080

# Comando para executar o aplicativo
CMD ["./main"]