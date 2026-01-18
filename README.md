# Aseno Backend

Backend em Go para a plataforma Aseno.

## 🚀 Quick Start

### Desenvolvimento (Recomendado)

**1. Na raiz do projeto, inicie a infraestrutura:**
```bash
cd /home/is_me/go/src/github.com/aseno
make up
```

Isso sobe:
- PostgreSQL (porta 5433)
- Redis (porta 6379)
- RabbitMQ (portas 5672, 15672)
- Evolution API (porta 8081)
- WAHA (porta 3002)

**2. Inicie o backend:**
```bash
cd backend
go run cmd/api/main.go
```

O backend estará rodando em `http://localhost:4901`

### Parar o Backend

Pressione `Ctrl+C` no terminal do backend. O shutdown agora demora ~10 segundos (antes eram 30s).

### Parar a Infraestrutura

```bash
cd /home/is_me/go/src/github.com/aseno
make down
```

## 📁 Estrutura

```
backend/
├── cmd/api/           # Entry point da aplicação
├── internal/          # Código interno (config, database, queue, worker)
├── pkg/              # Packages da aplicação (appointment, patient, etc)
├── .env              # Variáveis de ambiente (configurado para go run)
├── Dockerfile        # Para build Docker
├── docker-compose.yml # Para rodar backend no Docker (opcional)
└── README.md         # Este arquivo
```

## 🐳 Usando Docker (Opcional)

Se preferir rodar o backend no Docker ao invés de `go run`:

```bash
# 1. Certifique-se que a infra está rodando na raiz
cd /home/is_me/go/src/github.com/aseno
make up

# 2. Volte para a pasta backend e suba o container
cd backend
docker-compose up --build
```

**Nota:** O `docker-compose.yml` do backend **não** sobe PostgreSQL, Redis, RabbitMQ, etc. Ele assume que esses serviços já estão rodando via `make up` na raiz.

## ⚙️ Variáveis de Ambiente

O arquivo `.env` está configurado para usar os serviços da raiz:

- `DATABASE_URL` - Conecta em `localhost:5433` (PostgreSQL da raiz)
- `RABBITMQ_URL` - Conecta em `localhost:5672` (RabbitMQ da raiz)
- `EVOLUTION_API_URL` - Conecta em `localhost:8081` (Evolution da raiz)
- `WAHA_API_URL` - Conecta em `localhost:3002` (WAHA da raiz)

## 🧪 Testando WhatsApp

Após iniciar o backend:

1. Envie uma mensagem no WhatsApp
2. Observe os logs no terminal
3. Você deve ver logs de webhook, classificador de IA, e processamento

**Mudança importante:** O classificador de IA agora permite linguagem natural para seleção de horários:

- ✅ "quero na terça"
- ✅ "o primeiro"
- ✅ "às 14h"
- ✅ "com Dr. Silva"
- ✅ "2" (números ainda funcionam)

## 📊 Acessando Serviços

| Serviço | URL | Credenciais |
|---------|-----|-------------|
| Backend API | http://localhost:4901 | - |
| PostgreSQL | localhost:5433 | user: aseno / pass: aseno_password |
| Redis | localhost:6379 | - |
| RabbitMQ Management | http://localhost:15672 | user: aseno / pass: aseno_password |
| Evolution API | http://localhost:8081 | - |
| WAHA | http://localhost:3002 | API Key: seu_api_key_aqui_123456 |

## 🛠️ Comandos Úteis

```bash
# Ver logs do PostgreSQL
make logs-db

# Ver logs do Redis
make logs-redis

# Ver logs do RabbitMQ
make logs-rabbitmq

# Ver logs do Evolution
make logs-evolution

# Conectar ao banco via psql
make db-shell

# Ver usuários no banco
make db-users
```

## 🔧 Troubleshooting

### Backend não conecta no banco
```bash
# Verifique se o PostgreSQL está rodando na raiz
cd /home/is_me/go/src/github.com/aseno
docker ps | grep aseno-postgres

# Veja os logs
make logs-db
```

### Backend não conecta no RabbitMQ
```bash
# Verifique se o RabbitMQ está rodando
docker ps | grep aseno-rabbitmq

# Veja os logs
make logs-rabbitmq

# Acesse o painel de gerenciamento
open http://localhost:15672
```

### Mensagens do WhatsApp não chegam
1. Verifique se a Evolution API está rodando: `docker ps | grep aseno-evolution`
2. Verifique o webhook configurado na Evolution
3. O `.env` já está configurado com `host.docker.internal:4901` para permitir que a Evolution (Docker) chame o backend (host)

## 📚 Mais Informações

- Veja `DOCKER.md` para detalhes sobre Docker
- As migrations rodam automaticamente ao iniciar o backend
- O banco de dados é compartilhado entre todos os projetos (raiz, backend, frontend)
