#!/bin/bash
# Script para limpar histórico de conversação de um paciente
# Uso: ./scripts/clear-conversation.sh <phone_number>
# Exemplo: ./scripts/clear-conversation.sh 557581849068

set -e

if [ -z "$1" ]; then
    echo "Uso: $0 <phone_number>"
    echo "Exemplo: $0 557581849068"
    exit 1
fi

PHONE=$1
CLINIC_ID="ddb73e89-93ff-4309-b507-64bc635782aa"

echo "🧹 Limpando histórico de conversa para: $PHONE"

# 1. Deletar do Redis
echo "📦 Deletando do Redis..."
REDIS_KEY="conversation:phone:${CLINIC_ID}:${PHONE}"
DELETED=$(docker exec -i aseno-redis redis-cli DEL "$REDIS_KEY" 2>&1)
echo "   ✓ Redis: $DELETED chave(s) deletada(s)"

# 2. Deletar do PostgreSQL
echo "🗄️  Deletando do PostgreSQL..."
PG_RESULT=$(docker exec -i aseno-postgres psql -U aseno aseno_db -t -c "DELETE FROM conversation_contexts WHERE patient_phone = '$PHONE'; SELECT ROW_COUNT();" 2>&1 | grep -oP '\d+' | head -1)
echo "   ✓ PostgreSQL: $PG_RESULT registro(s) deletado(s)"

# 3. Deletar logs de tool calls também (opcional)
echo "🔧 Deletando logs de tool calls..."
TOOL_LOGS=$(docker exec -i aseno-postgres psql -U aseno aseno_db -t -c "DELETE FROM tool_call_logs WHERE conversation_id IN (SELECT id FROM conversation_contexts WHERE patient_phone = '$PHONE'); SELECT ROW_COUNT();" 2>&1 | grep -oP '\d+' | head -1 || echo "0")
echo "   ✓ Tool logs: $TOOL_LOGS registro(s) deletado(s)"

echo ""
echo "✅ Histórico limpo com sucesso!"
echo "   Próxima mensagem do paciente iniciará conversa nova."
