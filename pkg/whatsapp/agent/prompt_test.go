package agent

import (
	"strings"
	"testing"

	"github.com/aseno/backend/pkg/whatsapp/conversation"
)

func TestBuildUserPrompt_NoRolePrefixes(t *testing.T) {
	ctx := &conversation.ConversationContext{
		History: conversation.ConversationHistory{
			{Role: "user", Content: "quero marcar consulta"},
			{Role: "assistant", Content: "Claro! Vou buscar os horários disponíveis."},
			{Role: "user", Content: "pode ser amanhã de manhã"},
		},
	}

	result := BuildUserPrompt(ctx, "às 10h por favor")

	// The prompt must NOT contain "Paciente:" or "Você:" prefixes
	// because the AI model echoes them back in its responses
	if strings.Contains(result, "Paciente:") {
		t.Errorf("BuildUserPrompt should not contain 'Paciente:' prefix, got:\n%s", result)
	}
	if strings.Contains(result, "Você:") {
		t.Errorf("BuildUserPrompt should not contain 'Você:' prefix, got:\n%s", result)
	}
}

func TestBuildUserPrompt_HistoryUsesInternalLabels(t *testing.T) {
	ctx := &conversation.ConversationContext{
		History: conversation.ConversationHistory{
			{Role: "user", Content: "oi"},
			{Role: "assistant", Content: "Olá!"},
		},
	}

	result := BuildUserPrompt(ctx, "quero agendar")

	// Should contain the history content
	if !strings.Contains(result, "oi") {
		t.Error("BuildUserPrompt should contain user message content")
	}
	if !strings.Contains(result, "Olá!") {
		t.Error("BuildUserPrompt should contain assistant message content")
	}
	if !strings.Contains(result, "quero agendar") {
		t.Error("BuildUserPrompt should contain current message")
	}
}

func TestCorePersonality_ContainsNoPrefixInstruction(t *testing.T) {
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil)
	systemPrompt := builder.Build()

	// System prompt must instruct the AI to never include role prefixes
	if !strings.Contains(systemPrompt, "Paciente:") || !strings.Contains(systemPrompt, "Você:") {
		// The system prompt should mention these prefixes in the context of
		// telling the AI NOT to use them
	}

	// Check that there's an explicit instruction to not use prefixes
	if !strings.Contains(systemPrompt, "NUNCA use prefixos") {
		t.Error("System prompt should contain explicit instruction to never use role prefixes like 'Paciente:' or 'Você:'")
	}
}

func TestBuild_IncludesBusinessInfoWhenSet(t *testing.T) {
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil).
		WithBusinessInfo("Atendemos só particular. Aceitamos Pix e cartão.")
	prompt := builder.Build()

	if !strings.Contains(prompt, "Informações da clínica") {
		t.Error("Expected prompt to contain 'Informações da clínica' header when BusinessInfo is set")
	}
	if !strings.Contains(prompt, "Atendemos só particular. Aceitamos Pix e cartão.") {
		t.Error("Expected prompt to contain the business info content")
	}
}

func TestBuild_IncludesFAQWhenSet(t *testing.T) {
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil).
		WithFAQ("Q: Quanto tempo dura a sessão? A: 50 minutos.")
	prompt := builder.Build()

	if !strings.Contains(prompt, "Dúvidas frequentes") {
		t.Error("Expected prompt to contain 'Dúvidas frequentes' header when FAQ is set")
	}
	if !strings.Contains(prompt, "Quanto tempo dura a sessão") {
		t.Error("Expected prompt to contain the FAQ content")
	}
}

func TestBuild_OmitsSectionsWhenEmpty(t *testing.T) {
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil)
	prompt := builder.Build()

	if strings.Contains(prompt, "Informações da clínica") {
		t.Error("Prompt should NOT contain 'Informações da clínica' header when BusinessInfo is empty")
	}
	if strings.Contains(prompt, "Dúvidas frequentes") {
		t.Error("Prompt should NOT contain 'Dúvidas frequentes' header when FAQ is empty")
	}
}

func TestBuild_EmptyStringsAreTreatedAsUnset(t *testing.T) {
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil).
		WithBusinessInfo("   ").
		WithFAQ("")
	prompt := builder.Build()

	if strings.Contains(prompt, "Informações da clínica") {
		t.Error("Whitespace-only BusinessInfo should not emit section header")
	}
	if strings.Contains(prompt, "Dúvidas frequentes") {
		t.Error("Empty FAQ should not emit section header")
	}
}

func TestBuild_ListingRule_RequiresAllSubstringMatches(t *testing.T) {
	// Real-world bug: clinic has "RPG" AND "RPG — Avaliação" — patient asks
	// "vocês trabalham com RPG?" and AI lists only one. Same for "avaliação"
	// — clinic has 4, AI listed 3 (missed "RPG Avaliação"). Prompt must
	// instruct the LLM to include ALL items whose name contains the term.
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil)
	prompt := builder.Build()

	if !strings.Contains(strings.ToLower(prompt), "todos") {
		t.Error("Listing rule must explicitly demand including ALL matches, not the first/best")
	}
	// Must mention substring/contains semantics so the LLM knows to be permissive.
	if !strings.Contains(strings.ToLower(prompt), "contém") && !strings.Contains(strings.ToLower(prompt), "substring") {
		t.Error("Listing rule must reference substring/contains matching semantics")
	}
}

// Real-world bug: patient asks "RPG?" → AI lists 2 RPGs; patient says
// "quero agendar uma avaliação" → AI ignored the RPG context and listed 4
// generic avaliações. The fix is structured: list_procedures persists its
// last filter+names as conversation metadata, and the prompt builder injects
// that as a hint so the LLM doesn't have to re-read its own past text.

func TestBuild_RecentTopicHint_RendersWhenMetadataPresent(t *testing.T) {
	ctx := &conversation.ConversationContext{
		Metadata: conversation.ConversationMetadata{
			"last_procedure_filter": map[string]interface{}{
				"filter": []interface{}{"RPG"},
				"names":  []interface{}{"RPG", "RPG — Avaliação"},
			},
		},
	}
	builder := NewSystemPromptBuilder("Clínica Teste", "João", ctx)
	prompt := builder.Build()

	if !strings.Contains(prompt, "TÓPICO RECENTE") {
		t.Errorf("Prompt must include the recent-topic hint header when last_procedure_filter metadata is set. Got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "RPG — Avaliação") {
		t.Error("Hint must list the actual procedure names so the LLM can reference them in the next turn")
	}
	if !strings.Contains(prompt, "CONFIRME") && !strings.Contains(prompt, "confirme") {
		t.Error("Hint must instruct the LLM to confirm with the patient before re-calling the tool")
	}
}

func TestBuild_RecentTopicHint_OmittedWhenNoMetadata(t *testing.T) {
	// Without prior tool calls, no hint should appear — otherwise the prompt
	// would prime the LLM with "RECENT TOPIC" sections that are empty/wrong.
	ctx := &conversation.ConversationContext{}
	builder := NewSystemPromptBuilder("Clínica Teste", "João", ctx)
	prompt := builder.Build()

	if strings.Contains(prompt, "TÓPICO RECENTE") {
		t.Error("No metadata = no hint section. Got the section anyway")
	}
}

func TestBuild_RecentTopicHint_OmittedWhenMetadataMalformed(t *testing.T) {
	// Defensive: corrupted metadata must not crash the prompt builder.
	ctx := &conversation.ConversationContext{
		Metadata: conversation.ConversationMetadata{
			"last_procedure_filter": "not a map",
		},
	}
	builder := NewSystemPromptBuilder("Clínica Teste", "João", ctx)
	prompt := builder.Build() // must not panic
	if strings.Contains(prompt, "TÓPICO RECENTE") {
		t.Error("Malformed metadata must result in no hint, not a broken hint")
	}
}

func TestBuild_FAQVerbatimRule_PresentInPrompt(t *testing.T) {
	// FAQ entries marked "MANDE EXATAMENTE NO FORMATO ABAIXO" must be
	// reproduced verbatim. Real-world bug: pilates section had payment
	// methods in the FAQ but AI cherry-picked the prices and skipped them.
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil).
		WithFAQ("anything")
	prompt := builder.Build()

	if !strings.Contains(strings.ToLower(prompt), "exatamente") &&
		!strings.Contains(strings.ToLower(prompt), "verbatim") &&
		!strings.Contains(strings.ToLower(prompt), "literal") {
		t.Error("Prompt must mention FAQ verbatim/literal rule so the LLM doesn't edit FAQ content")
	}
}

// ---------------------------------------------------------------------------
// FAQ-is-not-the-catalog rule. Real-world bug: patient asked "vocês trabalham
// com RPG?" and the AI answered "não temos" because the FAQ didn't mention
// RPG — but the clinic DOES offer RPG, just not in the FAQ. The prompt must
// teach the LLM that absence from FAQ ≠ not offered.
// ---------------------------------------------------------------------------

func TestBuild_ClinicKnowledge_StatesAbsenceIsNotNegation(t *testing.T) {
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil).
		WithFAQ("Q: Quanto custa pilates? A: R$ 130")
	prompt := builder.Build()

	// The clinic-knowledge section must explicitly tell the LLM that a
	// procedure NOT appearing in FAQ does NOT mean the clinic doesn't offer
	// it. Without this rule the LLM treats FAQ as exhaustive.
	required := []string{"ausência", "list_procedures"}
	for _, kw := range required {
		if !strings.Contains(strings.ToLower(prompt), strings.ToLower(kw)) {
			t.Errorf("Clinic knowledge section must mention %q so the LLM doesn't treat FAQ as exhaustive. Got:\n%s", kw, prompt)
		}
	}
}

func TestBuild_ProcedureListing_HasExplicitFormatRule(t *testing.T) {
	// Real-world need: when list_procedures returns multiple matches (e.g. 4
	// types of "avaliação"), the LLM must present each with name + duration
	// + price in a consistent format so the patient can choose. Without this
	// rule, the LLM might list only names, or only price, or skip duration.
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil)
	prompt := builder.Build()

	required := []string{"duration_minutes", "price", "name"}
	for _, kw := range required {
		if !strings.Contains(prompt, kw) {
			t.Errorf("Default prompt must reference %q so the LLM knows which fields from list_procedures to surface in the listing. Got:\n%s", kw, prompt)
		}
	}
	// Must show an example with R$ formatting (Brazilian currency) and "min"
	// for duration so the LLM doesn't dump raw seconds or USD.
	if !strings.Contains(prompt, "R$") || !strings.Contains(prompt, "min") {
		t.Error("Procedure listing format must show R$ and 'min' so values render correctly in WhatsApp")
	}
}

func TestBuild_QuestionFlow_RequiresCatalogCheckBeforeDenying(t *testing.T) {
	// "vocês trabalham com X?" / "fazem X?" / "tem X?" — these MUST go through
	// list_procedures/list_specialties before answering, even if FAQ is silent.
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil)
	prompt := builder.Build()

	// The Question flow must explicitly cover the "do you offer X" pattern,
	// not just "what are your prices" / "what are your services".
	if !strings.Contains(strings.ToLower(prompt), "trabalham com") &&
		!strings.Contains(strings.ToLower(prompt), "fazem") &&
		!strings.Contains(strings.ToLower(prompt), "vocês têm") {
		t.Errorf("Question flow must address 'vocês trabalham com X / fazem X / têm X' patterns explicitly. Got:\n%s", prompt)
	}
	// And explicitly forbid "no" answers without checking list_procedures.
	if !strings.Contains(prompt, "list_procedures") || !strings.Contains(prompt, "list_specialties") {
		t.Error("Question flow must reference both list_procedures AND list_specialties as required checks before denying a service")
	}
}

// ---------------------------------------------------------------------------
// Unified flow rules — single prompt path. When the clinic disabled write
// tools (AIToolSchedule/Cancel/etc), they're hidden via DisallowedTools and
// the LLM falls back to complete_handoff_to_human. The prompt must surface
// both the fallback rule and the working_hours warning so the LLM doesn't
// invent availability from working_hours when get_available_slots is hidden.
// ---------------------------------------------------------------------------

func TestBuild_UnifiedFlow_HasFallbackToHandoffWhenWriteToolMissing(t *testing.T) {
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil)
	prompt := builder.Build()

	if !strings.Contains(prompt, "complete_handoff_to_human") {
		t.Error("Unified prompt must mention complete_handoff_to_human as the fallback when a write tool is missing from the toolset")
	}
	// Must explain WHEN to fall back (write tool not in toolset).
	if !strings.Contains(strings.ToLower(prompt), "não estiver disponível") &&
		!strings.Contains(strings.ToLower(prompt), "não está no seu toolset") &&
		!strings.Contains(strings.ToLower(prompt), "não aparece") {
		t.Errorf("Unified prompt must explain the trigger for handoff fallback (write tool missing from toolset). Got:\n%s", prompt)
	}
}

func TestBuild_UnifiedFlow_HasConfirmationSummaryBeforeHandoff(t *testing.T) {
	// Real-world: AI was calling complete_handoff_to_human immediately after
	// collecting tipo + dia + profissional. The clinic wants a summary first
	// (so the human team picks up an explicit confirmation, and the patient
	// has a chance to correct). The summary lists procedimento + data +
	// horário + profissional and asks "Posso confirmar?".
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil)
	prompt := builder.Build()

	// Required fields in the summary template.
	required := []string{"*Procedimento:*", "*Data:*", "*Horário:*", "*Profissional:*", "Posso confirmar"}
	for _, kw := range required {
		if !strings.Contains(prompt, kw) {
			t.Errorf("Unified prompt must include %q in the confirmation summary template. Got:\n%s", kw, prompt)
		}
	}
	// The flow rule: summary BEFORE complete_handoff_to_human.
	if !strings.Contains(strings.ToLower(prompt), "antes de chamar complete_handoff_to_human") &&
		!strings.Contains(strings.ToLower(prompt), "antes de finalizar") {
		t.Errorf("Unified prompt must order: collect → summary → patient confirms → complete_handoff_to_human. Got:\n%s", prompt)
	}
}

func TestBuild_SchedulingFlow_CapsSlotsAtFour(t *testing.T) {
	// Real-world: AI listed 15 slots from get_available_slots — overwhelming.
	// The cap is 3-4 distributed across the day (morning/afternoon/late).
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil)
	prompt := builder.Build()

	if !strings.Contains(prompt, "MÁXIMO 3 OU 4 horários") {
		t.Errorf("Scheduling flow must cap slot listing at 3-4 explicitly (real-world bug: AI listed 15 slots). Got:\n%s", prompt)
	}
	// Distribution rule must be there.
	if !strings.Contains(strings.ToLower(prompt), "distribui") &&
		!strings.Contains(strings.ToLower(prompt), "manhã") {
		t.Errorf("Scheduling flow must require distributed slots (morning/afternoon/late) instead of consecutive ones. Got:\n%s", prompt)
	}
}

func TestBuild_UnifiedFlow_GatherListIsCappedToThreeItems(t *testing.T) {
	// Real-world bug: AI was asking "frequência semanal: 1x, 2x...?" and
	// "convênio ou particular?" for RPG — Avaliação, because the FAQ has
	// these fields under the Pilates section. The prompt must explicitly
	// limit the gather list to the 3 standard items and tell the LLM to
	// respect FAQ section guards.
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil)
	prompt := builder.Build()

	// Must enumerate exactly 3 gather items and forbid extras.
	must := []string{"tipo", "horário", "profissional"}
	for _, kw := range must {
		if !strings.Contains(strings.ToLower(prompt), kw) {
			t.Errorf("Unified prompt must mention %q in the 3-item gather list. Got:\n%s", kw, prompt)
		}
	}
	// Must reference the Pilates-FAQ-leaking-to-RPG anti-pattern explicitly.
	if !strings.Contains(prompt, "frequência semanal") || !strings.Contains(prompt, "Pilates") {
		t.Errorf("Unified prompt must call out the Pilates-only fields (frequência semanal, convênio) as NOT applicable to other procedures. Got:\n%s", prompt)
	}
	// Must mention respecting FAQ section guards.
	if !strings.Contains(strings.ToLower(prompt), "guard") &&
		!strings.Contains(strings.ToLower(prompt), "somente se") {
		t.Errorf("Unified prompt must instruct the LLM to respect FAQ section guards (\"use SOMENTE se ...\"). Got:\n%s", prompt)
	}
}

func TestBuild_UnifiedFlow_WorkingHoursIsNotAvailability(t *testing.T) {
	// Real-world bug: AI listed working_hours for Hamilton/Thaiane and claimed
	// "horário disponível" — but working_hours is just the schedule, not slot
	// availability. With get_available_slots hidden in handoff clinics, the
	// LLM must not invent.
	builder := NewSystemPromptBuilder("Clínica Teste", "João", nil)
	prompt := builder.Build()

	if !strings.Contains(prompt, "working_hours") {
		t.Error("Unified prompt must mention working_hours by name to anchor the rule")
	}
	if !strings.Contains(strings.ToLower(prompt), "não é disponibilidade") &&
		!strings.Contains(strings.ToLower(prompt), "não significa") &&
		!strings.Contains(strings.ToLower(prompt), "trabalha normalmente") {
		t.Errorf("Unified prompt must clarify working_hours != slot availability. Got:\n%s", prompt)
	}
}
