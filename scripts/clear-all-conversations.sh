#!/bin/bash
# Script para limpar TODAS as conversações (útil para desenvolvimento/testes)
# Uso: ./scripts/clear-all-conversations.sh

set -e

echo "⚠️  ATENÇÃO: Você está prestes a deletar TODAS as conversas!"
echo "   Isso vai limpar Redis e PostgreSQL completamente."
echo ""
read -p "Tem certeza? (digite 'sim' para confirmar): " CONFIRM

if [ "$CONFIRM" != "sim" ]; then
    echo "❌ Operação cancelada."
    exit 0
fi

echo ""
echo "🧹 Limpando TODAS as conversas..."

# 1. Limpar Redis (todas as chaves de conversação)
echo "📦 Limpando Redis..."
REDIS_DELETED=$(docker exec -i aseno-redis redis-cli KEYS "conversation:*" | wc -l)
docker exec -i aseno-redis redis-cli --scan --pattern "conversation:*" | xargs -r docker exec -i aseno-redis redis-cli DEL > /dev/null 2>&1 || true
echo "   ✓ Redis: ~$REDIS_DELETED chave(s) deletada(s)"

# 2. Limpar PostgreSQL (todas as conversas)
echo "🗄️  Limpando PostgreSQL..."
PG_CONVERSATIONS=$(docker exec -i aseno-postgres psql -U aseno aseno_db -t -c "SELECT COUNT(*) FROM conversation_contexts;" 2>&1 | tr -d ' ')
docker exec -i aseno-postgres psql -U aseno aseno_db -c "TRUNCATE TABLE tool_call_logs CASCADE;" > /dev/null 2>&1
docker exec -i aseno-postgres psql -U aseno aseno_db -c "TRUNCATE TABLE conversation_contexts CASCADE;" > /dev/null 2>&1
echo "   ✓ PostgreSQL: $PG_CONVERSATIONS conversas deletadas"

echo ""
echo "✅ Todas as conversas foram limpas!"
echo "   Sistema resetado para estado inicial."
