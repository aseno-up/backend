package tools

import (
	"github.com/google/generative-ai-go/genai"
)

// Registry holds all available tools for the AI agent
type Registry struct {
	tools map[string]*Tool
}

// Tool represents a single tool/function that can be called by the AI
type Tool struct {
	Name        string
	Description string
	Parameters  *genai.Schema // Gemini function schema
	Handler     ToolHandler
}

// ToolHandler is a function that executes a tool
type ToolHandler func(params map[string]interface{}) (interface{}, error)

// NewRegistry creates a new tool registry with all available tools
func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]*Tool),
	}

	// Register all tools
	r.registerSlotManagementTools()
	r.registerAppointmentManagementTools()
	r.registerInformationTools()

	return r
}

// GetTool retrieves a tool by name
func (r *Registry) GetTool(name string) (*Tool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// GetAllTools returns all registered tools
func (r *Registry) GetAllTools() []*Tool {
	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetGeminiFunctionDeclarations returns Gemini-compatible function declarations
func (r *Registry) GetGeminiFunctionDeclarations() []*genai.FunctionDeclaration {
	declarations := make([]*genai.FunctionDeclaration, 0, len(r.tools))

	for _, tool := range r.tools {
		declarations = append(declarations, &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		})
	}

	return declarations
}

// register adds a tool to the registry
func (r *Registry) register(tool *Tool) {
	r.tools[tool.Name] = tool
}

// registerSlotManagementTools registers tools for managing available slots
func (r *Registry) registerSlotManagementTools() {
	// get_available_professionals
	r.register(&Tool{
		Name:        "get_available_professionals",
		Description: "Lista profissionais disponíveis da clínica, opcionalmente filtrando por especialidade (fisioterapia, psicologia, nutrição, etc)",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"specialty": {
					Type:        genai.TypeString,
					Description: "Filtrar por especialidade (opcional). Exemplos: fisioterapia, psicologia, nutrição",
				},
			},
		},
	})

	// get_available_slots
	r.register(&Tool{
		Name:        "get_available_slots",
		Description: "Busca horários disponíveis para agendamento. Retorna até 5 opções de horários livres.",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{}, // All parameters are optional
			Properties: map[string]*genai.Schema{
				"professional_id": {
					Type:        genai.TypeString,
					Description: "UUID do profissional (opcional, se não fornecido busca para todos)",
				},
				"date": {
					Type:        genai.TypeString,
					Description: "Data inicial para busca no formato YYYY-MM-DD (opcional, padrão: hoje)",
				},
				"duration_minutes": {
					Type:        genai.TypeInteger,
					Description: "Duração da consulta em minutos (opcional, padrão: 60 minutos)",
				},
				"days_ahead": {
					Type:        genai.TypeInteger,
					Description: "Quantos dias à frente buscar (opcional, padrão: 7)",
				},
			},
		},
	})
}

// registerAppointmentManagementTools registers tools for managing appointments
func (r *Registry) registerAppointmentManagementTools() {
	// create_appointment
	r.register(&Tool{
		Name:        "create_appointment",
		Description: "Cria um novo agendamento para o paciente",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{"professional_id", "start_time", "duration_minutes"},
			Properties: map[string]*genai.Schema{
				"professional_id": {
					Type:        genai.TypeString,
					Description: "UUID do profissional",
				},
				"start_time": {
					Type:        genai.TypeString,
					Description: "Data e hora de início no formato ISO 8601 (YYYY-MM-DDTHH:MM:SS)",
				},
				"duration_minutes": {
					Type:        genai.TypeInteger,
					Description: "Duração da consulta em minutos",
				},
				"notes": {
					Type:        genai.TypeString,
					Description: "Observações adicionais (opcional)",
				},
			},
		},
	})

	// confirm_appointment
	r.register(&Tool{
		Name:        "confirm_appointment",
		Description: "Confirma um agendamento pendente do paciente. Se appointment_id não for fornecido, confirma automaticamente se houver apenas um agendamento pendente.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"appointment_id": {
					Type:        genai.TypeString,
					Description: "UUID do agendamento (opcional se houver apenas um pendente)",
				},
			},
		},
	})

	// cancel_appointment
	r.register(&Tool{
		Name:        "cancel_appointment",
		Description: "Cancela um agendamento do paciente",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"appointment_id": {
					Type:        genai.TypeString,
					Description: "UUID do agendamento (opcional se houver apenas um futuro)",
				},
				"reason": {
					Type:        genai.TypeString,
					Description: "Motivo do cancelamento (opcional)",
				},
			},
		},
	})

	// reschedule_appointment
	r.register(&Tool{
		Name:        "reschedule_appointment",
		Description: "Remarca um agendamento para nova data/hora",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{"appointment_id", "new_start_time"},
			Properties: map[string]*genai.Schema{
				"appointment_id": {
					Type:        genai.TypeString,
					Description: "UUID do agendamento a remarcar",
				},
				"new_start_time": {
					Type:        genai.TypeString,
					Description: "Nova data e hora no formato ISO 8601 (YYYY-MM-DDTHH:MM:SS)",
				},
				"new_professional_id": {
					Type:        genai.TypeString,
					Description: "Novo profissional (opcional, mantém o mesmo se não fornecido)",
				},
			},
		},
	})

	// get_patient_appointments
	r.register(&Tool{
		Name:        "get_patient_appointments",
		Description: "Lista os agendamentos do paciente",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"status": {
					Type:        genai.TypeString,
					Description: "Filtrar por status: pending, confirmed, completed, cancelled (opcional)",
				},
				"limit": {
					Type:        genai.TypeInteger,
					Description: "Quantidade máxima de resultados (opcional, padrão: 10)",
				},
			},
		},
	})

	// search_appointment_history
	r.register(&Tool{
		Name:        "search_appointment_history",
		Description: "Busca no histórico de agendamentos do paciente por período (ex: 'mês passado', 'último trimestre', 'ano passado', '2024'). Útil para responder perguntas sobre agendamentos antigos.",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{"period"},
			Properties: map[string]*genai.Schema{
				"period": {
					Type:        genai.TypeString,
					Description: "Período para busca. Pode ser: 'last_week', 'last_month', 'last_3_months', 'last_6_months', 'last_year', 'year_2024', 'year_2023', etc. ou uma data específica no formato YYYY-MM-DD",
				},
				"professional_id": {
					Type:        genai.TypeString,
					Description: "Filtrar por profissional específico (UUID, opcional)",
				},
				"status": {
					Type:        genai.TypeString,
					Description: "Filtrar por status: confirmed, completed, cancelled, no_show (opcional)",
				},
				"limit": {
					Type:        genai.TypeInteger,
					Description: "Quantidade máxima de resultados (opcional, padrão: 20)",
				},
			},
		},
	})
}

// registerInformationTools registers tools for retrieving information
func (r *Registry) registerInformationTools() {
	// get_clinic_info
	r.register(&Tool{
		Name:        "get_clinic_info",
		Description: "Retorna informações da clínica (nome, endereço, horário de funcionamento, telefone)",
		Parameters: &genai.Schema{
			Type:       genai.TypeObject,
			Properties: map[string]*genai.Schema{},
		},
	})

	// get_professional_info
	r.register(&Tool{
		Name:        "get_professional_info",
		Description: "Retorna informações detalhadas de um profissional específico",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{"professional_id"},
			Properties: map[string]*genai.Schema{
				"professional_id": {
					Type:        genai.TypeString,
					Description: "UUID do profissional",
				},
			},
		},
	})

	// get_patient_info
	r.register(&Tool{
		Name:        "get_patient_info",
		Description: "Retorna informações do paciente atual (nome, telefone, email se disponível)",
		Parameters: &genai.Schema{
			Type:       genai.TypeObject,
			Properties: map[string]*genai.Schema{},
		},
	})
}
