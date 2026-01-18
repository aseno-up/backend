#!/bin/bash
# Script para verificar qual modelo Gemini foi usado
# e quantas tentativas de fallback aconteceram
# Uso: ./scripts/check-model-quota.sh [num_linhas]

LINES=${1:-100}

echo "📊 Análise de uso de modelos Gemini (últimas $LINES linhas de log)"
echo ""

# Verificar modelo primário configurado
PRIMARY_MODEL=$(grep "AI_AGENT_MODEL=" .env | cut -d'=' -f2)
echo "🎯 Modelo primário configurado: $PRIMARY_MODEL"
echo ""

# Buscar logs do backend
echo "📈 Modelos usados (com sucesso):"
docker compose logs backend --tail $LINES 2>/dev/null | \
    grep -oP "Success with fallback model: \K[^\s]+" | \
    sort | uniq -c | sort -rn | \
    awk '{printf "   %s: %s vezes\n", $2, $1}'

echo ""
echo "⚠️  Modelos que falharam:"
docker compose logs backend --tail $LINES 2>/dev/null | \
    grep -oP "Fallback model \K[^\s]+ failed" | \
    sed 's/ failed//' | \
    sort | uniq -c | sort -rn | \
    awk '{printf "   %s: %s falhas\n", $2, $1}'

echo ""
echo "🔄 Modelo primário usado:"
PRIMARY_SUCCESS=$(docker compose logs backend --tail $LINES 2>/dev/null | \
    grep -c "Generated response with.*model: $PRIMARY_MODEL" || echo "0")
echo "   $PRIMARY_MODEL: $PRIMARY_SUCCESS vezes"

echo ""
echo "💡 Dica: Se muitos modelos estão falhando, considere:"
echo "   1. Adicionar mais modelos ao AI_AGENT_FALLBACK_MODELS"
echo "   2. Verificar quotas na Google AI Studio"
echo "   3. Aguardar reset de quota (diário ou por minuto)"
