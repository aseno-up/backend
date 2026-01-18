# Docker Setup - Aseno Backend

## 🏗️ Arquitetura

O projeto está dividido em duas partes:

### **Raiz do Projeto** (`/home/is_me/go/src/github.com/aseno/`)
Gerencia a **infraestrutura compartilhada** via `Makefile`:
- **aseno-postgres** - PostgreSQL (porta 5433)
- **aseno-redis** - Redis (porta 6379)
- **aseno-rabbitmq** - RabbitMQ (portas 5672, 15672)
- **aseno-evolution** - Evolution API WhatsApp (porta 8081)
- **aseno-waha** - WAHA WhatsApp (porta 3002)

### **Pasta Backend** (`/home/is_me/go/src/github.com/aseno/backend/`)
Apenas o **backend da aplicação**:
- Pode rodar com `go run` (recomendado para dev)
- Ou com `docker-compose up` (opcional)

**Todos os containers rodam na rede `aseno-network`**

## 🚀 Fluxo Recomendado de Desenvolvimento

### 1. Subir a Infraestrutura (Uma vez por dia)

```bash
cd /home/is_me/go/src/github.com/aseno
make up
```

Isso sobe PostgreSQL, Redis, RabbitMQ, Evolution e WAHA. Deixe rodando o dia todo.

### 2. Rodar o Backend (Múltiplas vezes por dia)

```bash
cd backend
go run cmd/api/main.go
```

O backend conecta automaticamente nos serviços da raiz (`.env` já configurado).

### 3. Para parar apenas o backend

```bash
# Pressione Ctrl+C no terminal
# Shutdown agora demora ~10 segundos (antes eram 30s)
```

### 4. Para parar toda a infraestrutura (fim do dia)

```bash
cd /home/is_me/go/src/github.com/aseno
make down
```

## 🐳 Rodando Backend no Docker (Opcional)

Se preferir rodar o backend no Docker ao invés de `go run`:

```bash
# 1. Certifique-se que a infra está rodando
cd /home/is_me/go/src/github.com/aseno
make up

# 2. Suba o backend no Docker
cd backend
docker-compose up --build
```

**IMPORTANTE:** O `docker-compose.yml` do backend **NÃO** sobe PostgreSQL, Redis, RabbitMQ, etc. Ele assume que esses serviços já estão rodando via `make up`.

## 📊 Portas e Serviços

| Serviço | Porta Local | Nome do Container |
|---------|-------------|-------------------|
| Backend | 4901 | aseno_backend |
| PostgreSQL | 5433 | aseno-postgres |
| Redis | 6379 | aseno-redis |
| RabbitMQ | 5672 | aseno-rabbitmq |
| RabbitMQ Management | 15672 | aseno-rabbitmq |
| Evolution API | 8081 | aseno-evolution |
| WAHA | 3002 | aseno-waha |

## 🔧 Configuração de Variáveis de Ambiente

### Quando rodar com `go run` (Desenvolvimento Local)

O `.env` já está configurado:

```env
DATABASE_URL=postgres://aseno:aseno_password@localhost:5433/aseno_db?sslmode=disable
RABBITMQ_URL=amqp://aseno:aseno_password@localhost:5672/aseno
EVOLUTION_API_URL=http://localhost:8081
WHATSAPP_WEBHOOK_URL=http://host.docker.internal:4901/api/v1/whatsapp/webhook
```

**Nota:** `host.docker.internal` permite que Evolution (Docker) chame o backend (host).

### Quando rodar com Docker Compose

O `docker-compose.yml` ajusta automaticamente:

```env
DATABASE_URL=postgres://aseno:aseno_password@aseno-postgres:5432/aseno_db?sslmode=disable
RABBITMQ_URL=amqp://aseno:aseno_password@aseno-rabbitmq:5672/aseno
EVOLUTION_API_URL=http://aseno-evolution:8080
WHATSAPP_WEBHOOK_URL=http://aseno_backend:8080/api/v1/whatsapp/webhook
```

## 🛠️ Comandos Úteis (Raiz do Projeto)

```bash
cd /home/is_me/go/src/github.com/aseno

# Subir toda a infraestrutura
make up

# Subir apenas alguns serviços
make db redis rabbitmq

# Parar tudo
make down

# Ver logs
make logs-db
make logs-redis
make logs-rabbitmq
make logs-evolution
make logs-waha

# Conectar ao banco
make db-shell

# Ver usuários no banco
make db-users

# pgAdmin (interface gráfica)
make pgadmin
# Acesse: http://localhost:5050

# Limpar volumes (CUIDADO: apaga dados!)
make clean
```

## 🧪 Testando WhatsApp

1. Certifique-se que a infraestrutura está rodando:
   ```bash
   cd /home/is_me/go/src/github.com/aseno
   make up
   ```

2. Rode o backend:
   ```bash
   cd backend
   go run cmd/api/main.go
   ```

3. Envie mensagem no WhatsApp e observe os logs

## 🔍 Troubleshooting

### Porta já alocada (Erro: port is already allocated)

**Problema:** Você rodou `docker-compose up` no backend sem antes parar os containers conflitantes da raiz.

**Solução:**
```bash
# Pare containers do backend
cd backend
docker-compose down

# Use apenas os containers da raiz
cd /home/is_me/go/src/github.com/aseno
make up

# Rode backend com go run
cd backend
go run cmd/api/main.go
```

### Backend não conecta no banco

**Problema:** PostgreSQL não está rodando ou está na porta errada.

**Solução:**
```bash
# Verifique se está rodando
docker ps | grep aseno-postgres

# Verifique a porta (deve ser 5433, não 5432)
docker ps | grep aseno-postgres

# Se não estiver rodando
cd /home/is_me/go/src/github.com/aseno
make db
```

### Backend não conecta no RabbitMQ

```bash
# Verifique se RabbitMQ está rodando
docker ps | grep aseno-rabbitmq

# Inicie se necessário
cd /home/is_me/go/src/github.com/aseno
make rabbitmq

# Acesse painel de gerenciamento
open http://localhost:15672
# User: aseno / Password: aseno_password
```

### Mensagens do WhatsApp não chegam

1. Verifique se Evolution está rodando:
   ```bash
   docker ps | grep aseno-evolution
   ```

2. Verifique os logs da Evolution:
   ```bash
   make logs-evolution
   ```

3. O `.env` já está configurado com `host.docker.internal:4901`

### Shutdown lento

O timeout foi reduzido de 30s para 10s em `cmd/api/main.go:599`.

Se ainda estiver lento:
- Verifique conexões abertas ao RabbitMQ
- Verifique workers presos

## 📚 Diferenças entre Raiz e Backend

| Aspecto | Raiz | Backend |
|---------|------|---------|
| O que gerencia | Infraestrutura (DB, Redis, RabbitMQ, etc) | Apenas o backend |
| Como subir | `make up` | `go run` ou `docker-compose up` |
| Frequência | Uma vez por dia | Múltiplas vezes |
| Porta PostgreSQL | 5433 (externa) | Conecta em localhost:5433 |
| Banco de dados | Mesmo banco compartilhado | Conecta no banco da raiz |

## 🎯 Resumo

**Para desenvolvimento diário:**

1. **Manhã** - Suba a infra na raiz: `make up`
2. **O dia todo** - Rode backend: `go run cmd/api/main.go`
3. **Fim do dia** - Pare a infra: `make down`

**Vantagens:**
- ✅ Backend reinicia rápido (~segundos)
- ✅ Infra fica rodando o dia todo
- ✅ Não duplica containers
- ✅ Banco de dados compartilhado entre tudo
