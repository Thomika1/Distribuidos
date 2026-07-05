# Chargeback API com Controle de Concorrência 2PL

API REST para validação de chargebacks com middleware de controle de concorrência baseado no protocolo **Two-Phase Locking (2PL)**.

## Regras de Negócio

| Card Brand | Reason Code | Decision |
|---|---|---|
| Visa | `10.x` | `reject` (fraude) |
| Mastercard | `48xx` | `reject` (fraude) |
| Outros | Qualquer | `""` (vazio) |

> **Reject** significa rejeitar a disputa do chargeback e lutar pelo reembolso do emissor, pois o reason code indica fraude com alta probabilidade de o acquirer vencer.

## Modelo de Dados

```
Chargeback {
  ID              uint
  TransactionDate time.Time    (RFC3339)
  TransactionValue decimal.Decimal
  CardBrand       string
  ReasonCode      string
  Decision        string       ("accept" | "reject" | "")
  CreatedAt       time.Time
  UpdatedAt       time.Time
}
```

## Endpoints

| Método | Rota | Descrição | Lock |
|---|---|---|---|
| POST | `/api/v1/chargebacks` | Cria chargeback (aplica decision) | Exclusive |
| GET | `/api/v1/chargebacks` | Lista todos | Shared |
| GET | `/api/v1/chargebacks/:id` | Busca por ID | Shared |
| POST | `/api/v1/chargebacks/validate` | Valida sem persistir | Exclusive |
| POST | `/api/v1/chargebacks/nolock` | Cria sem middleware 2PL (comparação) | Nenhum |
| GET | `/api/v1/health` | Health check | Nenhum |

## Two-Phase Locking (2PL)

O middleware implementa o protocolo 2PL com duas fases:

1. **Growing Phase**: a transação adquire todos os locks necessários
2. **Shrinking Phase**: a transação libera os locks (após o primeiro unlock, não pode adquirir novos)

### Tipos de Lock

- **Shared**: múltiplas transações podem ler o mesmo recurso
- **Exclusive**: apenas uma transação acessa o recurso por vez

### Headers de Observabilidade

```
X-Lock-Acquired: true/false
X-Lock-Wait-Ms: 1847
X-Lock-Resource: /api/v1/chargebacks
X-Lock-Type: exclusive/shared
X-Lock-TxID: 20260705...
```

## Como Rodar

```bash
# Subir app + PostgreSQL
docker-compose up -d

# Rodar testes
make test-demo   # 5 VUs - demonstra serialização
make test        # 50 VUs - teste de carga com resumo
make test-tp     # 20 VUs - comparação de throughput

# Testes unitários do Go
make test-unit

# Limpar banco
make db-clean

# Derrubar containers
make down
```

## Conexão ao PostgreSQL (DBeaver)

```
Host: localhost
Port: 5433
User: postgres
Password: postgres
Database: chargebacks
```

## Variáveis de Ambiente

| Variável | Default | Descrição |
|---|---|---|
| `DB_DSN` | `host=localhost port=5432 ...` | String de conexão PostgreSQL |
| `PORT` | `8080` | Porta do servidor HTTP |
| `LOCK_TIMEOUT_MS` | `5000` | Timeout do lock em ms |
| `PROCESSING_DELAY_MS` | `0` | Delay simulado de processamento em ms |

## Estrutura do Projeto

```
Distribuidos/
├── cmd/main.go                      # HTTP server, DB init, rotas
├── pkg/
│   ├── lock/twopl.go                # Protocolo 2PL
│   ├── lock/twopl_test.go           # Testes do 2PL
│   ├── models/chargeback.go         # Model GORM
│   ├── models/chargeback_logic.go   # Validação de fraude
│   ├── handler/chargeback.go        # REST handlers
│   └── middleware/concurrency.go    # Middleware 2PL
├── tests/
│   ├── load-test.js                 # k6 - teste de carga
│   ├── demo-test.js                 # k6 - demo de serialização
│   └── throughput-test.js           # k6 - comparação de throughput
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## Stack

- **Go** + **Gin** (HTTP framework) + **GORM** (ORM) + **PostgreSQL**
- **Docker** + **Docker Compose**
- **shopspring/decimal** (valores decimais precisos)
- **Grafana k6** (testes de carga e concorrência)