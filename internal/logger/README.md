# Logger Colorido / Colored Logger

Logger customizado que adiciona cores automaticamente ao texto dentro de colchetes `[...]` nos logs.

## Características

- 🎨 **Coloração automática**: Colore automaticamente qualquer texto entre `[colchetes]`
- 🔧 **API compatível**: Mesma interface do pacote `log` padrão do Go
- ⚡ **Fácil migração**: Apenas troque `log.Printf` por `logger.Printf`
- 🎯 **Cores predefinidas**: Cores específicas para componentes comuns (RabbitMQ, Redis, Schedule, etc.)
- 🌈 **Customizável**: Registre suas próprias cores para componentes específicos
- 📝 **Níveis de log**: Funções auxiliares para ERROR, WARN, INFO, SUCCESS, DEBUG

## Instalação

A dependência `github.com/fatih/color` já está instalada no projeto.

## Uso Básico

### 1. Importar o logger

```go
import "github.com/aseno/backend/internal/logger"
```

### 2. Inicializar (opcional, mas recomendado)

No seu `main.go`:

```go
func main() {
    logger.Init()
    // ... resto do código
}
```

### 3. Usar nos logs

**Antes:**
```go
import "log"

log.Printf("[RabbitMQ] Connected to queue: %s", queueName)
log.Printf("[Schedule] Failed to find patient: %v", err)
log.Printf("[Reschedule] Failed to auto-confirm appointment: %v", err)
```

**Depois:**
```go
import "github.com/aseno/backend/internal/logger"

logger.Printf("[RabbitMQ] Connected to queue: %s", queueName)
logger.Printf("[Schedule] Failed to find patient: %v", err)
logger.Printf("[Reschedule] Failed to auto-confirm appointment: %v", err)
```

## Funções Disponíveis

### Funções principais (compatíveis com `log`)

```go
logger.Printf(format, args...)   // Formatted print com brackets coloridos
logger.Println(args...)          // Print line com brackets coloridos
logger.Print(args...)            // Print com brackets coloridos
logger.Fatalf(format, args...)   // Fatal error (sai do programa)
logger.Panicf(format, args...)   // Panic
```

### Funções auxiliares (adicionam prefixo automático)

```go
logger.Errorf(format, args...)   // Adiciona [ERROR] vermelho
logger.Warnf(format, args...)    // Adiciona [WARN] amarelo
logger.Infof(format, args...)    // Adiciona [INFO] ciano
logger.Successf(format, args...) // Adiciona [SUCCESS] verde
logger.Debugf(format, args...)   // Adiciona [DEBUG] branco
```

## Cores Predefinidas

O logger vem com cores predefinidas para componentes comuns:

| Componente | Cor | Categoria |
|------------|-----|-----------|
| **Infraestrutura** |
| `[RabbitMQ]` | Ciano (bold) | Message queue |
| `[Redis]` | Magenta (bold) | Cache |
| `[PostgreSQL]`, `[Database]` | Azul (bold) | Database |
| `[Migration]` | Azul | Database migrations |
| **Workers** |
| `[ScheduledMessageWorker]` | Amarelo (bold) | Workers |
| `[MessageSender]` | Amarelo (bold) | Workers |
| `[RateLimiter]` | Amarelo | Rate limiting |
| **Business Logic** |
| `[Schedule]`, `[Reschedule]` | Verde (bold) | Appointments |
| `[Appointment]`, `[Booking]` | Verde | Appointments |
| `[Patient]` | Verde | Patient management |
| **Services** |
| `[EMAIL]`, `[BREVO]` | Ciano | Email services |
| `[WhatsApp]` | Verde | Messaging |
| `[AUTH]` | Magenta | Authentication |
| `[Settings]`, `[Calendar]` | Azul | Configuration |
| `[Payment]` | Amarelo | Payments |
| `[Analytics]` | Ciano | Analytics |
| **Níveis de Log** |
| `[ERROR]`, `[FATAL]` | Vermelho (bold) | Errors |
| `[WARN]`, `[WARNING]` | Amarelo (bold) | Warnings |
| `[INFO]` | Ciano | Information |
| `[SUCCESS]` | Verde (bold) | Success |
| `[DEBUG]` | Branco | Debug |

**Nota:** Componentes não listados usarão a cor padrão (ciano bold).

## Exemplos

### Exemplo 1: Logs básicos

```go
logger.Printf("[RabbitMQ] Connected to queue: %s", "notifications")
logger.Printf("[Schedule] Processing appointment for patient ID: %s", "12345")
logger.Printf("[Reschedule] Failed to auto-confirm appointment: %v", err)
```

### Exemplo 2: Usando funções auxiliares

```go
logger.Infof("Application started successfully")
logger.Successf("All %d appointments synced", 100)
logger.Warnf("Rate limit approaching: %d/%d requests", 95, 100)
logger.Errorf("Failed to process booking: %v", err)
logger.Debugf("Cache hit for key: %s", "user:12345")
```

### Exemplo 3: Múltiplos brackets

```go
logger.Printf("[Schedule] [INFO] Found %d available slots for [Appointment]", 5)
logger.Printf("[Booking] Creating booking for clinic [ClinicA] with patient [PatientB]")
```

## Customização

### Registrar cores customizadas

```go
import "github.com/fatih/color"

// Registrar cor customizada para um novo componente
logger.RegisterColor("MyComponent", color.New(color.FgRed, color.Bold))

// Agora usar
logger.Printf("[MyComponent] Custom colored message")
```

### Desabilitar cores

```go
// Útil para testes ou output para arquivos
logger.DisableColor()

// Para reabilitar
logger.EnableColor()
```

### Configurar output customizado

```go
import "os"

// Redirecionar para arquivo
file, _ := os.Create("app.log")
logger.SetOutput(file)
```

## Migração do Código Existente

### Opção 1: Buscar e Substituir (Recomendado)

Use find & replace no seu editor:

1. **Substituir imports:**
   - Buscar: `import "log"`
   - Substituir: `import "github.com/aseno/backend/internal/logger"`

2. **Substituir chamadas:**
   - Buscar: `log\.Printf\(`
   - Substituir: `logger.Printf(`

   - Buscar: `log\.Println\(`
   - Substituir: `logger.Println(`

   - Buscar: `log\.Fatalf\(`
   - Substituir: `logger.Fatalf(`

### Opção 2: Gradual (Arquivo por Arquivo)

Migre um arquivo por vez:

1. Adicione o import do logger
2. Mantenha `log` como import também (temporariamente)
3. Vá substituindo `log.X` por `logger.X`
4. Quando terminar o arquivo, remova `import "log"`

### Opção 3: Usar alias (Compatibilidade temporária)

```go
import (
    logger "github.com/aseno/backend/internal/logger"
    "log" // Manter temporariamente
)

// Código novo usa logger
logger.Printf("[NewFeature] Something")

// Código antigo ainda funciona
log.Printf("[OldFeature] Something")  // Sem cores
```

## Demo

Execute o exemplo de demonstração:

```bash
cd backend
go run examples/logger_demo.go
```

Isso mostrará todos os componentes com suas cores.

## Variáveis de Ambiente

- `NO_COLOR`: Desabilita cores automaticamente (útil em CI/CD)

```bash
NO_COLOR=1 go run main.go
```

## Desempenho

O logger adiciona overhead mínimo:
- Apenas processa regex quando há texto entre colchetes
- Usa cache de cores (map lookup O(1))
- Detecção automática de TTY (desabilita cores se não for terminal)

## Suporte

O logger detecta automaticamente se está rodando em um terminal:
- ✅ Terminal: Cores habilitadas
- ❌ Arquivo/Pipe: Cores desabilitadas automaticamente
- ❌ CI/CD: Cores desabilitadas se `NO_COLOR` estiver definido
