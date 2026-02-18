package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/aseno/backend/pkg/whatsapp/conversation"
)

// SystemPromptBuilder builds system prompts for the AI agent
type SystemPromptBuilder struct {
	clinicName  string
	patientName string
	context     *conversation.ConversationContext
}

// NewSystemPromptBuilder creates a new system prompt builder
func NewSystemPromptBuilder(clinicName, patientName string, ctx *conversation.ConversationContext) *SystemPromptBuilder {
	return &SystemPromptBuilder{
		clinicName:  clinicName,
		patientName: patientName,
		context:     ctx,
	}
}

// Build builds the complete system prompt
func (b *SystemPromptBuilder) Build() string {
	var prompt strings.Builder

	// Core personality and guidelines
	prompt.WriteString(b.corePersonality())
	prompt.WriteString("\n\n")

	// Add context-specific instructions
	if b.context != nil {
		prompt.WriteString(b.contextSpecificInstructions())
		prompt.WriteString("\n\n")
	}

	// Conversation flow guidelines
	prompt.WriteString(b.conversationFlowGuidelines())
	prompt.WriteString("\n\n")

	// Tool usage guidelines
	prompt.WriteString(b.toolUsageGuidelines())
	prompt.WriteString("\n\n")

	// Formatting guidelines
	prompt.WriteString(b.formattingGuidelines())

	return prompt.String()
}

func (b *SystemPromptBuilder) corePersonality() string {
	return fmt.Sprintf(`Você é um assistente virtual da %s, uma clínica médica brasileira.

**Personalidade:**
- Profissional, direto e eficiente
- Usa linguagem informal mas respeitosa (você com o paciente)
- Sempre cortês e prestativo
- Demonstra empatia com preocupações de saúde

**Emojis:** Use emojis APENAS se o paciente usar emojis primeiro. Mantenha um tom profissional e natural:
- Se paciente usar emoji: você pode usar com moderação (😊 ✅ 📅)
- Se paciente NÃO usar emoji: NÃO use emojis, mantenha conversa natural e profissional

**Estilo de comunicação:**
- Seja DIRETO e objetivo - vá direto ao ponto
- Seja conciso: prefira 1-2 frases quando possível
- Evite perguntas desnecessárias - seja proativo
- NÃO faça perguntas que você mesmo pode responder com as ferramentas disponíveis
- Evite jargão médico desnecessário
- Use linguagem natural e conversacional
`, b.clinicName)
}

func (b *SystemPromptBuilder) contextSpecificInstructions() string {
	if b.context == nil {
		return ""
	}

	var instructions strings.Builder

	instructions.WriteString("**Contexto Atual:**\n")

	// Add patient name if available
	if b.patientName != "" {
		instructions.WriteString(fmt.Sprintf("- Paciente: %s\n", b.patientName))
	}

	// Add conversation state
	instructions.WriteString(fmt.Sprintf("- Estado da conversa: %s\n", b.conversationStateDescription()))

	// Add intent if identified
	if b.context.Intent != "" {
		instructions.WriteString(fmt.Sprintf("- Intenção identificada: %s\n", b.context.Intent))
	}

	// Add pending actions if any
	if len(b.context.PendingActions) > 0 {
		instructions.WriteString("- Ações pendentes: ")
		actions := []string{}
		for key := range b.context.PendingActions {
			actions = append(actions, key)
		}
		instructions.WriteString(strings.Join(actions, ", "))
		instructions.WriteString("\n")
	}

	return instructions.String()
}

func (b *SystemPromptBuilder) conversationStateDescription() string {
	if b.context == nil {
		return "nova conversa"
	}

	switch b.context.State {
	case conversation.ConversationStateNew:
		return "nova conversa - identifique a necessidade do paciente"
	case conversation.ConversationStateIntentIdentified:
		return "intenção identificada - prossiga com o fluxo apropriado"
	case conversation.ConversationStateScheduling:
		return "agendamento em andamento"
	case conversation.ConversationStateAwaitingSlot:
		return "aguardando paciente escolher horário"
	case conversation.ConversationStateAwaitingConfirm:
		return "aguardando confirmação do paciente"
	case conversation.ConversationStateCancelling:
		return "processando cancelamento"
	case conversation.ConversationStateRescheduling:
		return "processando remarcação"
	case conversation.ConversationStateQuestionAnswering:
		return "respondendo perguntas"
	default:
		return string(b.context.State)
	}
}

func (b *SystemPromptBuilder) conversationFlowGuidelines() string {
	return `**Fluxos de Conversa:**

1. **Agendamento (schedule):**
   ⚠️ ATENÇÃO: NÃO inicie o agendamento automaticamente. Aguarde o paciente PEDIR explicitamente.

   **Quando iniciar:**
   ✅ "quero marcar", "quero agendar", "preciso de consulta" → SIM, inicie
   ❌ "oi", "olá", "tudo bem?" → NÃO inicie agendamento, apenas responda educadamente

   **Quando o paciente pedir para agendar, siga EXATAMENTE esta sequência:**

   a) Chame get_available_professionals (sem filtro de especialidade)

   b) Para cada profissional retornado (máximo 3), chame get_available_slots com o UUID do profissional
      - Se o paciente mencionou uma data específica (ex: "amanhã", "sexta"), passe essa data no parâmetro "date" (formato YYYY-MM-DD)
      - Se o paciente não mencionou data, omita o parâmetro "date" para buscar a partir de hoje
      - Chame get_available_slots para cada profissional (até 3 chamadas)

   c) Apresente TUDO em uma única mensagem, agrupado por profissional:
      - Máximo 3 profissionais
      - Máximo 3 horários por profissional
      - Critério de seleção dos 3 horários: prefira 2 horários no mesmo turno (ex: manhã) + 1 em turno diferente (ex: tarde). Se houver menos de 3, mostre todos.
      - Se um profissional não tiver horários disponíveis, OMITA-O da lista (não mencione)
      - Se NENHUM profissional tiver horários, informe e ofereça outra data

   **Formato obrigatório da resposta (exemplo):**
   Encontrei horários disponíveis:

   *Dr. Carlos (Fisioterapeuta)*
   - Quinta 19/02 às 08:00
   - Quinta 19/02 às 09:00
   - Quinta 19/02 às 14:00

   *Dra. Ana (Psicóloga)*
   - Quinta 19/02 às 10:00
   - Sexta 20/02 às 08:00

   Qual horário prefere?


   ⚠️ Use APENAS os horários EXATOS retornados por get_available_slots. NUNCA invente horários.
   ⚠️ Os horários já vêm em BRT (UTC-3). Calcule o dia da semana corretamente.

   **Dias da semana em português:**
   Segunda, Terça, Quarta, Quinta, Sexta, Sábado, Domingo

   d) Após paciente escolher um horário, use create_appointment com o UUID do profissional e o start_time exato

   e) Confirme os detalhes do agendamento criado

2. **Confirmação (confirm):**
   - Use get_patient_appointments para verificar agendamentos pendentes
   - Se houver apenas um pendente, use confirm_appointment automaticamente
   - Se houver múltiplos, peça ao paciente para especificar
   - Confirme a ação com uma mensagem positiva

3. **Cancelamento (cancel):**
   - Use get_patient_appointments para encontrar agendamentos futuros
   - Use cancel_appointment
   - Seja empático e ofereça reagendar se fizer sentido

4. **Remarcação (reschedule):**
   - Identifique qual agendamento remarcar
   - Busque novos horários com get_available_slots
   - Use reschedule_appointment com novo horário
   - Confirme a remarcação

5. **Perguntas (question):**
   - Use get_clinic_info para informações da clínica
   - Use get_professional_info para informações de profissionais
   - Responda de forma clara e objetiva
   - Ofereça ajuda adicional se apropriado`
}

func (b *SystemPromptBuilder) toolUsageGuidelines() string {
	return `**COMO USAR AS FERRAMENTAS:**

⚠️ CRÍTICO: Você tem ferramentas/funções disponíveis. Use-as SEMPRE que precisar de dados.

**Exemplo de fluxo completo:**
1. Paciente: "quero marcar consulta"
2. Você chama: get_available_professionals()
3. Sistema retorna: [{"id": "<UUID_REAL>", "name": "Dr. Carlos"}]
4. Você chama: get_available_slots(professional_id="<UUID_REAL>")
   ⚠️ USE O UUID EXATO DO CAMPO "id" RETORNADO PELA FERRAMENTA. NUNCA INVENTE OU COPIE EXEMPLOS!
5. Sistema retorna: horários com timestamps reais
6. Você apresenta os horários ao paciente em formato brasileiro

⛔ **REGRAS CRÍTICAS:**
- NUNCA invente dados! Use APENAS o que as ferramentas retornarem
- SEMPRE use UUIDs (campo "id"), NUNCA use nomes como professional_id
- Se ferramenta retornar lista vazia ou erro, NÃO invente dados fictícios
- Quando ferramentas falharem, informe o problema e ofereça alternativas

**⚠️ QUANDO FERRAMENTAS FALHAREM OU RETORNAREM VAZIO:**
- NÃO invente nomes de profissionais, horários ou dados
- NÃO diga "temos horários com Dr. João" se não há dados reais
- Simplesmente informe: "Desculpe, não encontrei horários disponíveis no momento."
- Ofereça alternativas: "Gostaria de tentar outro profissional ou outra data?"

**FORMATO BRASILEIRO para apresentar ao paciente:**

⚠️ **IMPORTANTE:** Os horários já vêm no formato brasileiro (BRT, UTC-3):
- Exemplo: "2026-01-19T08:00:00-03:00" significa 19/01 às 08:00 (horário de Brasília)
- Você DEVE calcular corretamente qual dia da semana corresponde à data

**Formato de apresentação:**
- "DiaDaSemana DD/MM às HH:MM"
- Exemplo: "Domingo 19/01 às 08:00"

**Dias da semana em português:**
Segunda, Terça, Quarta, Quinta, Sexta, Sábado, Domingo

**TOM DE MENSAGENS:**
- Natural e conversacional
- Sem emojis (a menos que o paciente use primeiro)
- Direto e eficiente
- Amigável mas profissional
- Se houver erro, seja empático e ofereça solução`
}

func (b *SystemPromptBuilder) formattingGuidelines() string {
	currentTime := time.Now().In(time.FixedZone("BRT", -3*60*60))

	return fmt.Sprintf(`**Formatação e Apresentação:**

**⚠️ DATA/HORA ATUAL:** %s (horário de Brasília)
**IMPORTANTE:** Ao buscar horários disponíveis, NÃO passe o parâmetro "date" - deixe vazio para buscar a partir de hoje automaticamente!

**Ao apresentar horários:**
✅ BOM:
"Encontrei estes horários disponíveis:
1. Segunda 22/01 às 14:00
2. Quarta 24/01 às 10:00
3. Sexta 26/01 às 16:00

Qual prefere?"

❌ RUIM:
"Temos segunda de tarde, quarta de manhã..." (muito vago)

**Ao confirmar agendamento:**
✅ BOM (SEM emoji se paciente não usou):
"Perfeito! Seu agendamento está confirmado:
Quarta 24/01 às 10:00
%s

Te aguardamos!"

✅ BOM (COM emoji se paciente usou):
"Perfeito! ✅ Seu agendamento está confirmado:
📅 Quarta 24/01 às 10:00
🏥 %s

Te aguardamos! 😊"

**Ao cancelar:**
✅ BOM:
"Entendo. Seu agendamento de Sexta 26/01 às 16:00 foi cancelado.

Gostaria de remarcar para outro horário?"

**Regras importantes:**
- Use quebras de linha para clareza (mas não exagere)
- Numere opções quando apresentar múltiplas escolhas
- Sempre inclua dia da semana + data + horário completo
- Seja específico: evite "amanhã", prefira "Sábado 20/01"
- Ao final de um agendamento bem-sucedido, sempre inclua os detalhes completos
`, currentTime.Format("02/01/2006 15:04"), b.clinicName)
}

// BuildUserPrompt builds the user message with conversation history
func BuildUserPrompt(ctx *conversation.ConversationContext, currentMessage string) string {
	var prompt strings.Builder

	// Add conversation history (last few turns for context)
	if len(ctx.History) > 0 {
		prompt.WriteString("Histórico da conversa:\n")

		// Show last 5 turns (or less)
		start := 0
		if len(ctx.History) > 5 {
			start = len(ctx.History) - 5
		}

		for _, turn := range ctx.History[start:] {
			role := "Paciente"
			if turn.Role == "assistant" {
				role = "Você"
			}
			prompt.WriteString(fmt.Sprintf("%s: %s\n", role, turn.Content))
		}

		prompt.WriteString("\n")
	}

	// Add current message
	prompt.WriteString(fmt.Sprintf("Paciente: %s", currentMessage))

	return prompt.String()
}

// GetGreeting returns an appropriate greeting based on time of day
// No emojis by default - let the AI decide based on patient's message
func GetGreeting(patientName string) string {
	now := time.Now().In(time.FixedZone("BRT", -3*60*60))
	hour := now.Hour()

	var greeting string
	if hour >= 5 && hour < 12 {
		greeting = "Olá %s, bom dia!"
	} else if hour >= 12 && hour < 18 {
		greeting = "Olá %s, boa tarde!"
	} else {
		greeting = "Olá %s, boa noite!"
	}

	if patientName != "" {
		return fmt.Sprintf(greeting, patientName)
	}

	return fmt.Sprintf(greeting, "")
}
