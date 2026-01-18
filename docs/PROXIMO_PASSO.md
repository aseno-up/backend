# 🎯 Próximo Passo - AI Agent

## ✅ O que FOI FEITO

**Fase 1 & 2 - COMPLETAS** (100%)

1. ✅ **16 arquivos criados** com toda a base do AI Agent
2. ✅ **Conversation management** - Redis + PostgreSQL
3. ✅ **10 tools** implementadas
4. ✅ **Gemini agent** com function calling
5. ✅ **AutoMigrate** configurado
6. ✅ **WhatsApp service** preparado
7. ✅ **Configurações** adicionadas ao .env
8. ✅ **Documentação completa**

## 🔧 O que FALTA (simples)

### 1. Configurar Redis (15 minutos)

O código está pronto, mas precisa de Redis configurado.

**Opção A: Docker (recomendado)**
```bash
# Adicionar ao docker-compose.yml
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

**Opção B: Redis local**
```bash
# Ubuntu/Debian
sudo apt install redis-server
sudo systemctl start redis

# MacOS
brew install redis
brew services start redis
```

**Verificar:**
```bash
redis-cli ping
# Deve retornar: PONG
```

### 2. Inicializar Redis no código (5 minutos)

No `cmd/api/main.go`, adicionar antes da linha 120:

```go
// Initialize Redis
import redisClient "github.com/aseno/backend/internal/redis"

redis, err := redisClient.NewRedisClient(cfg.GetRedisURL())
if err != nil {
	log.Fatalf("Failed to connect to Redis: %v", err)
}
defer redis.Close()
log.Println("Redis connected successfully")
```

### 3. Descomentar inicialização do AI Agent (2 minutos)

No `cmd/api/main.go` linha 334, **descomentar** o bloco:

```go
// Remover o /* e o */ ao redor deste código:
if redis != nil {
	clinicService := clinic_service.NewClinicService(clinicRepo)
	aiAgentService, _, _, err := initializeAIAgent(
		cfg,
		db,
		redis,
		professionalService,
		appointmentService,
		patientService,
		clinicService,
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize AI Agent: %v", err)
	} else if aiAgentService != nil {
		whatsappService.SetAIAgentService(aiAgentService)
		log.Println("AI Agent service connected to WhatsApp")
	}
}
```

### 4. Habilitar no .env (1 minuto)

Editar `.env`:

```bash
AI_AGENT_ENABLED=true
AI_AGENT_SCHEDULING_ENABLED=true
```

### 5. Testar! (5 minutos)

```bash
# Rebuild e reiniciar
go run cmd/api/main.go

# Nos logs, deve aparecer:
# [INFO] Redis connected successfully
# [INFO] AI Agent initialized: provider=gemini model=gemini-2.0-flash-exp
# [INFO] AI Agent service connected to WhatsApp
```

## 📋 Checklist Rápido

- [ ] Redis instalado e rodando
- [ ] Código Redis adicionado no main.go
- [ ] Bloco AI Agent descomentado
- [ ] AI_AGENT_ENABLED=true no .env
- [ ] App compilando sem erros
- [ ] Logs mostram "AI Agent initialized"

## 🎯 Total de tempo: ~30 minutos

## 💡 Dica

Se não quiser configurar Redis agora, o sistema continua funcionando normalmente com o fluxo tradicional (templates). O AI Agent é **adicional**, não quebra nada se não estiver ativo.

## 📞 Suporte

- **Redis não conecta?** Verifique se está na porta 6379
- **Erros de compilação?** Execute `go mod tidy`
- **AI Agent não aparece nos logs?** Verifique `AI_AGENT_ENABLED=true`

---

**Resumo**: Só falta configurar Redis e descomentar 1 bloco de código! 🚀
