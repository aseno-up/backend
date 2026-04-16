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
