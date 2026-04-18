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
	r.registerCatalogTools()

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
		Description: "Lista profissionais ativos da clínica com suas especialidades e horário de trabalho semanal (working_hours). Use para responder perguntas como 'vocês têm fisioterapeuta?', 'quando a Thaiane atende?' ou 'quem atende segunda de manhã?'. Retorna array com: id, name, specialties[{id,name}], working_hours{monday,tuesday,...}. Para filtrar por specialty, passe o NOME EXATO retornado por list_specialties (case-insensitive). NUNCA passe termos aproximados sem consultar list_specialties antes — o filtro é estrito.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"specialty": {
					Type:        genai.TypeString,
					Description: "Filtrar por nome EXATO da especialidade (opcional), conforme retornado por list_specialties. Case-insensitive. Se o paciente usar termo aproximado (ex: 'fisioterapia'), descubra o nome canônico via list_specialties (ex: 'Fisioterapeuta') primeiro.",
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
				"professional_name": {
					Type:        genai.TypeString,
					Description: "Alternativa ao professional_id quando você NÃO tem o UUID certo. Passe fragmento do nome (ex: 'hamilton', 'Dra. Ana') — case-insensitive substring match. Se houver exatamente 1 match, a tool resolve sozinha. Prefira este parâmetro quando em dúvida sobre o UUID.",
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
	// quick_schedule — shortcut for "marcar com X para Y dia Z hora" when the
	// patient already provided everything. Picks the professional by name
	// fragment (case-insensitive substring), validates the exact slot, and
	// creates the appointment in one hop. No manual UUID juggling by the LLM.
	r.register(&Tool{
		Name:        "quick_schedule",
		Description: "ATALHO: cria um agendamento direto quando o paciente já disse profissional (ou parte do nome), dia e hora em uma única mensagem. Ex: 'marcar com hamilton segunda 16h'. Use esta tool antes de get_available_professionals/get_available_slots quando o paciente for específico — evita listagem desnecessária. Retorna sucesso se único match e horário disponível; retorna lista de candidatos ou horários disponíveis se ambíguo/indisponível.",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{"professional_name", "date", "start_time"},
			Properties: map[string]*genai.Schema{
				"professional_name": {
					Type:        genai.TypeString,
					Description: "Nome ou fragmento do nome do profissional (ex: 'hamilton', 'Dra. Ana'). Case-insensitive substring match. REQUIRED.",
				},
				"date": {
					Type:        genai.TypeString,
					Description: "Data no formato YYYY-MM-DD. Converta 'hoje', 'amanhã', 'segunda' etc para data absoluta antes. REQUIRED.",
				},
				"start_time": {
					Type:        genai.TypeString,
					Description: "Horário no formato HH:MM (24h). Ex: '16:00' para 4h da tarde. REQUIRED.",
				},
				"duration_minutes": {
					Type:        genai.TypeInteger,
					Description: "Duração em minutos. OMIT para usar padrão de 60 minutos.",
				},
			},
		},
	})

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
					Description: "The UUID from the \"id\" field returned by get_available_professionals (e.g. \"<UUID_FROM_PROFESSIONALS_LIST>\"). REQUIRED. MUST be a valid UUID in xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx format. NEVER pass names, text, or invented values.",
				},
				"start_time": {
					Type:        genai.TypeString,
					Description: "The EXACT start_time timestamp returned by get_available_slots (e.g. \"2026-04-10T10:00:00-03:00\"). REQUIRED. MUST be in ISO 8601 format. NEVER invent timestamps.",
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
					Description: "UUID of the appointment (e.g. '<UUID_FROM_APPOINTMENTS_LIST>'). OMIT THIS PARAMETER if there is only one pending appointment - system will auto-detect. MUST be valid UUID format.",
				},
			},
		},
	})

	// cancel_appointment
	r.register(&Tool{
		Name:        "cancel_appointment",
		Description: "Cancela um agendamento do paciente. Passe appointment_id se souber, OU passe time_filter quando o paciente disser o horário (ex: 'o das 17h') — isso evita listar tudo e pedir UUID ao paciente.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"appointment_id": {
					Type:        genai.TypeString,
					Description: "UUID of the appointment. OMIT if not known. Prefer time_filter when the patient specified the time.",
				},
				"time_filter": {
					Type:        genai.TypeString,
					Description: "Horário que o paciente mencionou (HH:MM, ex: '17:00'). Se o paciente disse 'cancela o das 17h' passe '17:00' aqui; a tool filtra entre os agendamentos futuros e cancela automaticamente se houver exatamente 1 match. OMIT se paciente não deu o horário.",
				},
				"reason": {
					Type:        genai.TypeString,
					Description: "Cancellation reason. OMIT if no specific reason was provided by the patient.",
				},
			},
		},
	})

	// quick_reschedule — shortcut when the patient gave new date + new time in
	// a single message. Resolves the current appointment automatically (by
	// time_filter or single-future-appt), validates the new slot, reschedules
	// in one hop. Much simpler than reschedule_appointment which requires a
	// known UUID upfront.
	r.register(&Tool{
		Name:        "quick_reschedule",
		Description: "Tool UNIFICADA para todo o fluxo de remarcação. Chame SEMPRE que o paciente quiser remarcar, mesmo se só disse 'quero remarcar' sem mais nada — a tool retorna a lista de agendamentos pra você mostrar. Quando paciente dá novo dia+hora, a tool valida e remarca direto. Use em vez de perguntar 'qual dia?' às cegas.",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{}, // all optional — tool decides what it needs
			Properties: map[string]*genai.Schema{
				"new_date": {
					Type:        genai.TypeString,
					Description: "Nova data no formato YYYY-MM-DD. OMIT se paciente ainda não disse — a tool vai retornar lista de agendamentos pra você mostrar e perguntar. Quando souber, chame de novo passando new_date + new_time.",
				},
				"new_time": {
					Type:        genai.TypeString,
					Description: "Novo horário HH:MM (ex: '10:00'). OMIT se paciente ainda não disse.",
				},
				"current_time_filter": {
					Type:        genai.TypeString,
					Description: "Horário do agendamento atual (HH:MM). Passe quando o paciente identificou qual remarcar (ex: 'o das 16h'). OMIT se só há 1 agendamento futuro.",
				},
				"appointment_id": {
					Type:        genai.TypeString,
					Description: "UUID do agendamento. OMIT e deixa a tool resolver sozinha — só passe se alguém já te deu o UUID explicitamente.",
				},
				"duration_minutes": {
					Type:        genai.TypeInteger,
					Description: "Duração em minutos. OMIT para usar padrão de 60.",
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
					Description: "UUID of the appointment to reschedule (e.g. '<UUID_FROM_APPOINTMENTS_LIST>'). REQUIRED. MUST be valid UUID format.",
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

	// update_patient_name
	r.register(&Tool{
		Name:        "update_patient_name",
		Description: "Atualiza o nome do paciente. Use quando o paciente informar que seu nome está errado ou pedir para corrigir/alterar o nome cadastrado.",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{"new_name"},
			Properties: map[string]*genai.Schema{
				"new_name": {
					Type:        genai.TypeString,
					Description: "O nome correto do paciente, conforme informado por ele.",
				},
			},
		},
	})
}

// registerCatalogTools registers tools that expose the clinic catalog
// (specialties, procedures with prices and duration) so the AI can answer
// pricing questions and discover the clinic's taxonomy at runtime.
func (r *Registry) registerCatalogTools() {
	// list_specialties
	r.register(&Tool{
		Name:        "list_specialties",
		Description: "Lista as especialidades ATIVAS cadastradas na clínica com id e nome exato. Use SEMPRE ANTES de chamar get_available_professionals ou list_procedures com filtro de specialty — os nomes retornados aqui são os únicos que o filtro dessas tools reconhece. Se o paciente falar um termo aproximado (ex: 'fisioterapia'), encontre a specialty mais próxima neste retorno (ex: 'Fisioterapeuta') e use esse nome exato no filtro.",
		Parameters: &genai.Schema{
			Type:       genai.TypeObject,
			Properties: map[string]*genai.Schema{},
		},
	})

	// list_procedures
	r.register(&Tool{
		Name:        "list_procedures",
		Description: "Lista os procedimentos/serviços oferecidos pela clínica com preço e duração. Use para responder perguntas como 'quanto custa uma massoterapia?', 'quais serviços vocês oferecem?', 'qual o valor da consulta de X?'. Retorna array com: id, name, price (em reais), duration_minutes, specialty_name. Para filtrar por specialty, passe o NOME EXATO retornado por list_specialties (chame list_specialties primeiro).",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"specialty": {
					Type:        genai.TypeString,
					Description: "Filtrar por nome EXATO da especialidade (opcional), conforme retornado por list_specialties. Case-insensitive. Se o paciente usar termo aproximado, descubra o nome canônico via list_specialties primeiro.",
				},
			},
		},
	})
}
