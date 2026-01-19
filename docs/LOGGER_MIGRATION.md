# Guia de Migração para o Logger Colorido

Este guia mostra como migrar o código existente para usar o novo logger colorido.

## Visão Geral

O logger colorido foi criado para adicionar cores automaticamente aos logs do projeto, especialmente para textos entre colchetes como `[RabbitMQ]`, `[Schedule]`, `[Reschedule]`, etc.

## Passo a Passo

### 1. Importar o Logger

**Antes:**
```go
import "log"
```

**Depois:**
```go
import "github.com/aseno/backend/internal/logger"
```

### 2. Inicializar no main.go (Opcional)

Adicione no início da função `main()`:

```go
func main() {
    logger.Init()  // Configura o logger
    // ... resto do código
}
```

### 3. Substituir Chamadas de Log

#### Padrão Simples

**Antes:**
```go
log.Printf("[RabbitMQ] Connected to queue: %s", queueName)
log.Printf("[Schedule] Processing appointment for patient ID: %s", patientID)
log.Printf("[Reschedule] Failed to auto-confirm appointment: %v", err)
```

**Depois:**
```go
logger.Printf("[RabbitMQ] Connected to queue: %s", queueName)
logger.Printf("[Schedule] Processing appointment for patient ID: %s", patientID)
logger.Printf("[Reschedule] Failed to auto-confirm appointment: %v", err)
```

#### Com Níveis de Log

**Antes:**
```go
log.Printf("[ERROR] Failed to connect: %v", err)
log.Printf("[WARN] Rate limit approaching")
```

**Depois (Opção 1 - Manter estilo original):**
```go
logger.Printf("[ERROR] Failed to connect: %v", err)
logger.Printf("[WARN] Rate limit approaching")
```

**Depois (Opção 2 - Usar funções auxiliares):**
```go
logger.Errorf("Failed to connect: %v", err)  // Adiciona [ERROR] automaticamente
logger.Warnf("Rate limit approaching")       // Adiciona [WARN] automaticamente
```

## Exemplos de Migração de Arquivos Reais

### Exemplo 1: main.go

**Antes:**
```go
package main

import (
    "log"
    // ... outros imports
)

func main() {
    // ... código de setup

    log.Printf("[Schedule] Starting appointment scheduler")
    if err := startScheduler(); err != nil {
        log.Fatalf("[Schedule] Failed to start scheduler: %v", err)
    }

    log.Printf("[Reschedule] Starting rescheduler worker")
    // ... resto do código
}
```

**Depois:**
```go
package main

import (
    "github.com/aseno/backend/internal/logger"
    // ... outros imports (remover "log")
)

func main() {
    logger.Init()  // Adicionar inicialização

    // ... código de setup

    logger.Printf("[Schedule] Starting appointment scheduler")
    if err := startScheduler(); err != nil {
        logger.Fatalf("[Schedule] Failed to start scheduler: %v", err)
    }

    logger.Printf("[Reschedule] Starting rescheduler worker")
    // ... resto do código
}
```

### Exemplo 2: rabbitmq.go

**Antes:**
```go
package queue

import (
    "log"
    amqp "github.com/rabbitmq/amqp091-go"
)

func Connect(url, queueName string) (*amqp.Connection, error) {
    conn, err := amqp.Dial(url)
    if err != nil {
        log.Printf("[RabbitMQ] Failed to connect: %v", err)
        return nil, err
    }
    log.Printf("[RabbitMQ] Connected successfully to queue: %s", queueName)
    return conn, nil
}
```

**Depois:**
```go
package queue

import (
    "github.com/aseno/backend/internal/logger"
    amqp "github.com/rabbitmq/amqp091-go"
)

func Connect(url, queueName string) (*amqp.Connection, error) {
    conn, err := amqp.Dial(url)
    if err != nil {
        logger.Printf("[RabbitMQ] Failed to connect: %v", err)
        return nil, err
    }
    logger.Printf("[RabbitMQ] Connected successfully to queue: %s", queueName)
    return conn, nil
}
```

### Exemplo 3: whatsapp_service.go

**Antes:**
```go
func (s *Service) ScheduleAppointment(ctx context.Context, req ScheduleRequest) error {
    log.Printf("[Schedule] Scheduling appointment for patient: %s", req.PatientPhone)

    patient, err := s.findOrCreatePatient(ctx, req)
    if err != nil {
        log.Printf("[Schedule] Failed to find/create patient: %v", err)
        return err
    }

    log.Printf("[Schedule] Successfully scheduled appointment for patient ID: %s", patient.ID)
    return nil
}
```

**Depois:**
```go
func (s *Service) ScheduleAppointment(ctx context.Context, req ScheduleRequest) error {
    logger.Printf("[Schedule] Scheduling appointment for patient: %s", req.PatientPhone)

    patient, err := s.findOrCreatePatient(ctx, req)
    if err != nil {
        logger.Printf("[Schedule] Failed to find/create patient: %v", err)
        return err
    }

    logger.Printf("[Schedule] Successfully scheduled appointment for patient ID: %s", patient.ID)
    return nil
}
```

## Estratégias de Migração

### Opção A: Migração Global (Rápida)

Use find & replace no seu editor ou IDE:

1. **Buscar:** `import "log"`
   **Substituir:** `import "github.com/aseno/backend/internal/logger"`

2. **Buscar:** `\blog\.Printf\(`
   **Substituir:** `logger.Printf(`

3. **Buscar:** `\blog\.Println\(`
   **Substituir:** `logger.Println(`

4. **Buscar:** `\blog\.Fatalf\(`
   **Substituir:** `logger.Fatalf(`

5. **Buscar:** `\blog\.Panicf\(`
   **Substituir:** `logger.Panicf(`

**Nota:** Use regex se seu editor suportar.

### Opção B: Migração Gradual (Segura)

Migre arquivo por arquivo:

1. Escolha um arquivo para migrar (comece com arquivos menores)
2. Adicione o import do logger
3. Substitua as chamadas `log.X` por `logger.X`
4. Teste o arquivo
5. Remova o import de `log` se não for mais necessário
6. Repita para outros arquivos

### Opção C: Migração Híbrida (Temporária)

Use ambos temporariamente:

```go
import (
    "log"  // Código antigo ainda usa
    "github.com/aseno/backend/internal/logger"  // Código novo usa
)

func oldFunction() {
    log.Printf("[OldCode] Still using old logger")  // Sem cores
}

func newFunction() {
    logger.Printf("[NewCode] Using new logger")  // Com cores!
}
```

## Checklist de Migração

- [ ] Instalar dependência: `go get github.com/fatih/color` (já instalado)
- [ ] Adicionar `logger.Init()` no `main.go`
- [ ] Migrar imports de `log` para `logger`
- [ ] Substituir `log.Printf` → `logger.Printf`
- [ ] Substituir `log.Println` → `logger.Println`
- [ ] Substituir `log.Print` → `logger.Print`
- [ ] Substituir `log.Fatalf` → `logger.Fatalf`
- [ ] Substituir `log.Panicf` → `logger.Panicf`
- [ ] (Opcional) Converter `log.Printf("[ERROR] ...")` → `logger.Errorf(...)`
- [ ] (Opcional) Converter `log.Printf("[WARN] ...")` → `logger.Warnf(...)`
- [ ] Testar aplicação
- [ ] Verificar logs no terminal (devem estar coloridos!)

## Arquivos Prioritários para Migração

Baseado na análise do código, estes arquivos têm mais logs e se beneficiariam mais da migração:

### Alta Prioridade (Muitos logs)
1. `/cmd/api/main.go` - Ponto de entrada principal
2. `/internal/queue/rabbitmq.go` - Logs de conexão
3. `/internal/redis/redis.go` - Logs de Redis
4. `/internal/worker/scheduled_message_worker.go` - Logs de worker
5. `/internal/worker/message_sender_worker.go` - Logs de mensagens
6. `/pkg/whatsapp/service/whatsapp_service.go` - Logs de agendamento

### Média Prioridade
7. `/internal/redis/rate_limiter.go`
8. `/pkg/appointment/service/appointment_service.go`
9. `/internal/database/migrations/migrate.go`
10. `/pkg/email/service/email_service.go`

### Baixa Prioridade
- Outros arquivos com poucos logs ou logs menos frequentes

## Testando a Migração

### Antes de Migrar
```bash
cd backend
go run cmd/api/main.go
# Logs aparecem sem cores
```

### Depois de Migrar
```bash
cd backend
go run examples/logger_demo.go  # Ver demo primeiro
go run cmd/api/main.go
# Logs aparecem coloridos! 🎨
```

## Comandos Úteis

```bash
# Ver exemplo de cores
go run examples/logger_demo.go

# Contar quantos arquivos usam log
grep -r "import \"log\"" --include="*.go" | wc -l

# Listar todos os arquivos que usam log
grep -r "import \"log\"" --include="*.go" -l

# Buscar todos os log.Printf no projeto
grep -r "log\.Printf" --include="*.go"

# Substituir em todos os arquivos .go (CUIDADO!)
find . -name "*.go" -exec sed -i 's/log\.Printf/logger.Printf/g' {} \;
```

## Problemas Comuns

### 1. Import não encontrado

**Erro:**
```
could not import github.com/aseno/backend/internal/logger
```

**Solução:**
```bash
cd backend
go mod tidy
```

### 2. Cores não aparecem

**Causas possíveis:**
- Rodando em ambiente não-TTY (pipe, arquivo)
- Variável de ambiente `NO_COLOR` está definida
- Rodando no Windows sem suporte ANSI

**Solução:**
```go
logger.EnableColor()  // Forçar cores
```

### 3. Conflito com package log

**Erro:**
```
ambiguous reference to Printf
```

**Solução:**
```go
import (
    stdlog "log"  // Renomear import
    "github.com/aseno/backend/internal/logger"
)

stdlog.Printf("...")   // Log padrão
logger.Printf("...")   // Logger colorido
```

## Dicas

1. **Começe pequeno**: Migre primeiro arquivos menores para ganhar confiança
2. **Use Git**: Faça commits incrementais durante a migração
3. **Teste frequentemente**: Execute a aplicação após cada arquivo migrado
4. **Verifique visualmente**: Abra o terminal e veja se as cores aparecem
5. **Documente**: Adicione comentários em componentes com cores customizadas

## Próximos Passos

Após migrar todos os arquivos:

1. Adicionar cores customizadas para novos componentes, se necessário
2. Considerar adicionar níveis de log (DEBUG, INFO, etc.) de forma consistente
3. Configurar logging estruturado para produção (opcional)
4. Adicionar logs em arquivos (opcional, mantendo cores no terminal)

## Suporte

- Documentação completa: `backend/internal/logger/README.md`
- Exemplo de uso: `backend/examples/logger_demo.go`
- Cores disponíveis: Veja a tabela no README
