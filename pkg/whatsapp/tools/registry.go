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

// OpenRouterTool represents a tool in OpenRouter/OpenAI format
type OpenRouterTool struct {
	Type     string             `json:"type"`
	Function OpenRouterFunction `json:"function"`
}

// OpenRouterFunction represents a function in OpenRouter/OpenAI format
type OpenRouterFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// GetOpenRouterFunctionDeclarations returns OpenRouter/OpenAI-compatible function declarations
func (r *Registry) GetOpenRouterFunctionDeclarations() []OpenRouterTool {
	tools := make([]OpenRouterTool, 0, len(r.tools))

	for _, tool := range r.tools {
		tools = append(tools, OpenRouterTool{
			Type: "function",
			Function: OpenRouterFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  convertGeminiSchemaToOpenRouter(tool.Parameters),
			},
		})
	}

	return tools
}

// convertGeminiSchemaToOpenRouter converts a Gemini schema to OpenRouter/OpenAI JSON schema format
func convertGeminiSchemaToOpenRouter(schema *genai.Schema) map[string]interface{} {
	if schema == nil {
		return map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"additionalProperties": false,
		}
	}

	result := make(map[string]interface{})

	// Convert type
	switch schema.Type {
	case genai.TypeObject:
		result["type"] = "object"
		result["additionalProperties"] = false
	case genai.TypeString:
		result["type"] = "string"
	case genai.TypeInteger:
		result["type"] = "integer"
	case genai.TypeNumber:
		result["type"] = "number"
	case genai.TypeBoolean:
		result["type"] = "boolean"
	case genai.TypeArray:
		result["type"] = "array"
	default:
		result["type"] = "object"
		result["additionalProperties"] = false
	}

	// Add description if present
	if schema.Description != "" {
		result["description"] = schema.Description
	}

	// Add required fields if present
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	// Convert properties recursively
	if len(schema.Properties) > 0 {
		properties := make(map[string]interface{})
		for key, propSchema := range schema.Properties {
			properties[key] = convertGeminiSchemaToOpenRouter(propSchema)
		}
		result["properties"] = properties
	}

	// Convert items for arrays
	if schema.Items != nil {
		result["items"] = convertGeminiSchemaToOpenRouter(schema.Items)
	}

	// Add enum if present
	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}

	return result
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
		Description: "Busca horários disponíveis para agendamento. Busca automaticamente nos próximos 7 dias até encontrar horários. Retorna até 5 opções de horários livres. NUNCA retorna vazio se houver slots disponíveis na semana.",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{}, // All parameters are optional
			Properties: map[string]*genai.Schema{
				"professional_id": {
					Type:        genai.TypeString,
					Description: "UUID of the professional. OMIT THIS PARAMETER to search across all professionals. MUST be a valid UUID from get_available_professionals result, never use names or invent values.",
				},
				"date": {
					Type:        genai.TypeString,
					Description: "Start date in YYYY-MM-DD format. OMIT THIS PARAMETER to search from today automatically. Only provide if you need a specific future date. Never use past dates.",
				},
				"duration_minutes": {
					Type:        genai.TypeInteger,
					Description: "Consultation duration in minutes. OMIT THIS PARAMETER to use default of 60 minutes. Only provide if a different duration is needed.",
				},
				"days_ahead": {
					Type:        genai.TypeInteger,
					Description: "How many days ahead to search. OMIT THIS PARAMETER to use default of 7 days. Only provide if a different range is needed.",
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
					Description: "UUID of the professional. REQUIRED. MUST be valid UUID from get_available_professionals result, never use names or invent values.",
				},
				"start_time": {
					Type:        genai.TypeString,
					Description: "Start date and time in ISO 8601 format (YYYY-MM-DDTHH:MM:SS or YYYY-MM-DDTHH:MM:SS-03:00). REQUIRED. Use the EXACT timestamp from get_available_slots result.",
				},
				"duration_minutes": {
					Type:        genai.TypeInteger,
					Description: "Consultation duration in minutes. REQUIRED. Use the value from the slot or 60 as default.",
				},
				"notes": {
					Type:        genai.TypeString,
					Description: "Additional notes or observations. OMIT THIS PARAMETER if patient did not provide any specific notes. Only include if patient mentioned specific requests or observations.",
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
					Description: "UUID of the appointment (e.g. '7b8e3c2a-5f1d-4e9a-b3c4-1a2b3c4d5e6f'). OMIT THIS PARAMETER if there is only one pending appointment - system will auto-detect. MUST be valid UUID format.",
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
					Description: "UUID of the appointment (e.g. '7b8e3c2a-5f1d-4e9a-b3c4-1a2b3c4d5e6f'). OMIT THIS PARAMETER if there is only one future appointment - system will auto-detect. MUST be valid UUID format.",
				},
				"reason": {
					Type:        genai.TypeString,
					Description: "Cancellation reason. OMIT THIS PARAMETER if no specific reason was provided by the patient. Only include if patient explicitly states a reason.",
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
					Description: "UUID of the appointment to reschedule (e.g. '7b8e3c2a-5f1d-4e9a-b3c4-1a2b3c4d5e6f'). REQUIRED. MUST be valid UUID format.",
				},
				"new_start_time": {
					Type:        genai.TypeString,
					Description: "New date and time in ISO 8601 format (YYYY-MM-DDTHH:MM:SS or YYYY-MM-DDTHH:MM:SS-03:00). REQUIRED.",
				},
				"new_professional_id": {
					Type:        genai.TypeString,
					Description: "UUID of new professional. OMIT THIS PARAMETER to keep the same professional. Only provide if patient wants to change professional. MUST be valid UUID from get_available_professionals result.",
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
					Description: "Filter by status: pending, confirmed, completed, cancelled. OMIT THIS PARAMETER to get all appointments regardless of status. Only provide if filtering for specific status is needed.",
				},
				"limit": {
					Type:        genai.TypeInteger,
					Description: "Maximum number of results. OMIT THIS PARAMETER to use default of 10. Only provide if a different limit is needed.",
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
					Description: "Search period. REQUIRED. Can be: 'last_week', 'last_month', 'last_3_months', 'last_6_months', 'last_year', 'year_2024', 'year_2023', etc. or a specific date in YYYY-MM-DD format.",
				},
				"professional_id": {
					Type:        genai.TypeString,
					Description: "UUID of specific professional to filter. OMIT THIS PARAMETER to search across all professionals. MUST be valid UUID from get_available_professionals result.",
				},
				"status": {
					Type:        genai.TypeString,
					Description: "Filter by status: confirmed, completed, cancelled, no_show. OMIT THIS PARAMETER to search all statuses. Only provide if filtering for specific status is needed.",
				},
				"limit": {
					Type:        genai.TypeInteger,
					Description: "Maximum number of results. OMIT THIS PARAMETER to use default of 20. Only provide if a different limit is needed.",
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
					Description: "UUID of the professional. REQUIRED. MUST be valid UUID from get_available_professionals result, never use names.",
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
