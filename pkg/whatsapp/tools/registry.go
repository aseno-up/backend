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
	r.registerHandoffTools()
	r.registerGroupClassTools()

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

	// Convert properties recursively. OpenAI's strict tool-schema validation
	// requires every "type": "object" to declare a "properties" key — even
	// when empty. Gemini is lenient about this; OpenRouter forwards as-is and
	// OpenAI rejects with "object schema missing properties", silently flipping
	// all traffic to the fallback model.
	if schema.Type == genai.TypeObject {
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
		Description: "Lista profissionais ativos da clínica com suas especialidades e horário de trabalho semanal (working_hours). Use para responder perguntas como 'vocês têm fisioterapeuta?', 'quando a Thaiane atende?' ou 'quem atende segunda de manhã?'. Quando o paciente já escolheu um procedimento específico, SEMPRE passe procedure_id pra ver SÓ os profissionais qualificados — sem isso, a tool retorna todos da clínica e você não sabe quem realmente faz aquele procedimento. Retorna array com: id, name, specialties[{id,name}], working_hours{monday,tuesday,...}.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"specialty": {
					Type:        genai.TypeString,
					Description: "Filtrar por nome EXATO da especialidade (opcional), conforme retornado por list_specialties. Case-insensitive. Se o paciente usar termo aproximado (ex: 'fisioterapia'), descubra o nome canônico via list_specialties (ex: 'Fisioterapeuta') primeiro.",
				},
				"procedure_id": {
					Type:        genai.TypeString,
					Description: "UUID de um procedimento (do campo 'id' de list_procedures). Quando passado, filtra a resposta para conter SÓ os profissionais que têm associação ativa com aquele procedimento. Use SEMPRE que o paciente já escolheu o procedimento (ex: 'Avaliação de RPG') — sem esse filtro a tool retorna todos os profissionais da clínica e você pode listar quem não faz aquele procedimento. Se vier vazio, ninguém na clínica faz aquele procedimento — diga isso ao paciente em vez de inventar nomes.",
				},
			},
		},
	})

	// get_available_slots
	r.register(&Tool{
		Name:        "get_available_slots",
		Description: "Busca horários disponíveis. Sem time_filter retorna AMOSTRA de até 4 horários distribuídos pelo dia (não a lista completa). Quando o paciente perguntar por um horário ESPECÍFICO (ex: 'tem 11h amanhã?'), passe date + time_filter — a tool valida AQUELE horário exato no banco em vez de você adivinhar pela amostra anterior.",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{}, // All parameters are optional
			Properties: map[string]*genai.Schema{
				"professional_id": {
					Type:        genai.TypeString,
					Description: "UUID do profissional (do campo 'id' de get_available_professionals). OMIT pra buscar em todos. NUNCA invente UUIDs.",
				},
				"professional_name": {
					Type:        genai.TypeString,
					Description: "Alternativa ao professional_id quando você NÃO tem o UUID certo. Passe fragmento do nome (ex: 'hamilton', 'Dra. Ana') — case-insensitive substring match. Se houver exatamente 1 match, a tool resolve sozinha. Prefira este parâmetro quando em dúvida sobre o UUID.",
				},
				"date": {
					Type:        genai.TypeString,
					Description: "Data início no formato YYYY-MM-DD. OMIT pra buscar a partir de hoje. Use sempre que o paciente disser 'amanhã', 'sexta', etc — converta pra YYYY-MM-DD antes.",
				},
				"time_filter": {
					Type:        genai.TypeString,
					Description: "Horário específico HH:MM que o paciente pediu (ex: '11:00', '14:30'). Quando passado, a tool retorna SOMENTE esse horário se estiver livre, ou vazio se ocupado/fora do expediente. Use SEMPRE que o paciente perguntar 'tem HH:MM?' ou der um horário exato — em vez de procurar na amostra anterior (que pode ter cortado o slot pedido por causa do sample distribuído). Combine com date pra fazer a checagem cirúrgica do dia/hora pedido.",
				},
				"procedure_id": {
					Type:        genai.TypeString,
					Description: "UUID do procedimento (do campo 'id' de list_procedures). Quando passado, a tool retorna APENAS slots de profissionais qualificados pra esse procedimento (via professional_procedures). USE SEMPRE que o paciente já escolheu o procedimento — sem isso a tool retorna slots de qualquer profissional, inclusive os que não fazem aquele procedimento (alucinação). Se ninguém faz, retorna vazio — diga ao paciente que não tem profissional cadastrado em vez de inventar.",
				},
				"duration_minutes": {
					Type:        genai.TypeInteger,
					Description: "Duração da consulta em minutos. OMIT pra usar 60. Use o valor de duration_minutes do procedimento (vem em list_procedures).",
				},
				"days_ahead": {
					Type:        genai.TypeInteger,
					Description: "Quantos dias buscar à frente. OMIT pra usar 7. Quando passar time_filter + date, omita esse param (a busca é cirúrgica num dia só).",
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
					Description: "The UUID from the \"id\" field returned by get_available_professionals. REQUIRED. MUST be a valid UUID in xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx format. NEVER pass names, text, or invented values.",
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
					Description: "UUID of the appointment. OMIT THIS PARAMETER if there is only one pending appointment - system will auto-detect. MUST be valid UUID format.",
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
					Description: "UUID of the appointment to reschedule. REQUIRED. MUST be valid UUID format.",
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

// registerHandoffTools registers tools used in handoff mode (clinic has the
// action tool — schedule/reschedule/cancel — disabled). The only tool here
// is complete_handoff_to_human, which marks the existing per-patient takeover
// after the LLM has gathered enough context for the human team to act.
func (r *Registry) registerHandoffTools() {
	r.register(&Tool{
		Name: "complete_handoff_to_human",
		Description: "Para CALAR a IA. Tem dois usos, controlados pelo parâmetro mode. NENHUM dos dois envia mensagem ao paciente — sua resposta de texto neste turno será DESCARTADA pelo sistema. A única diferença entre os modos é se a conversa fica pausada por horas (handoff) ou se só este turno é pulado (silent).\n\n" +
			"mode=\"handoff\" (padrão, omita o parâmetro): pausa o atendimento da IA por algumas horas (configurável por clínica). Toda mensagem que o paciente mandar enquanto pausado é registrada mas NÃO respondida pela IA — fica para o time humano assumir pelo WhatsApp. Use quando o paciente claramente terminou de conversar com você OU pediu pra falar com alguém da clínica OU você terminou de coletar contexto pra um agendamento que o humano vai fechar.\n\n" +
			"mode=\"silent\": pula APENAS este turno, sem pausar nada. A próxima mensagem que o paciente mandar volta a ser respondida normalmente pela IA. Use quando a última mensagem do paciente NÃO é endereçada a você (não é uma pergunta, não responde a uma pergunta SUA pendente) e responder qualquer coisa seria confuso. Exemplos onde silent é OBRIGATÓRIO (não use handoff aqui — pausar a conversa por horas por causa de uma única mensagem ambígua é destrutivo):\n" +
			"• paciente manda só \"sim\", \"não\", \"ok\", \"blz\", \"vlw\", emoji vazio, etc — E você NÃO tem nenhuma pergunta sua pendente no histórico recente (sua última mensagem foi uma despedida, agradecimento ou fechamento)\n" +
			"• paciente confirma um horário, profissional ou procedimento específico que VOCÊ não propôs (provavelmente combinado pelo atendente humano antes — procure por linhas [human_agent] no histórico)\n" +
			"• paciente manda mensagem que claramente continua uma conversa que ele teve com o time humano (linhas [human_agent] no histórico) e você não tem contexto pra ajudar\n\n" +
			"NUNCA use silent se: (a) o paciente cumprimenta, (b) faz pergunta real, (c) compartilha informação que muda o contexto (\"meu nome é X\", \"meu telefone é Y\"), (d) sua última mensagem foi uma pergunta direta — nesses casos RESPONDA normalmente.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"mode": {
					Type:        genai.TypeString,
					Enum:        []string{"handoff", "silent"},
					Description: "\"handoff\" (padrão) pausa a IA por horas pro time humano assumir. \"silent\" pula só este turno sem pausar. Ambos NÃO enviam texto ao paciente. Para mensagens monossilábicas (\"sim\", \"não\", \"ok\") sem pergunta sua pendente, USE silent — handoff seria destrutivo (pausa horas por uma única mensagem ambígua).",
				},
			},
		},
	})
}

// registerGroupClassTools registers the tools used to enroll patients in
// recurring group session plans (Pilates, etc.) — distinct from the 1-on-1
// appointment flow. The slot tool is read-only and always available; the
// enroll tool is gated by AIToolEnrollClass via DisallowedTools, so a clinic
// that wants the human team to confirm matrículas keeps the AI on the
// handoff path while still letting it gather context.
func (r *Registry) registerGroupClassTools() {
	r.register(&Tool{
		Name:        "get_group_class_slots",
		Description: "Valida disponibilidade de UM dia (e opcionalmente horário) de turma em grupo. 🚨 weekday é OBRIGATÓRIO — listar a grade inteira sem filtro vira spam pra qualquer turma. Use APÓS o paciente dizer qual dia/horário quer: passe weekday + time_filter pra checar aquele slot, OU só weekday (sem time_filter) pra listar alternativas daquele dia quando o pedido original tá ocupado. Slots retornados refletem vaga real (capacity − matrículas ativas). NUNCA use get_available_slots pra turma em grupo — aquela calcula horários virtuais por profissional e inventa slots que não existem. professional_name pode vir null — NÃO invente nome.",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{"weekday"},
			Properties: map[string]*genai.Schema{
				"procedure_id": {
					Type:        genai.TypeString,
					Description: "UUID do procedimento (do campo 'id' de list_procedures). RECOMENDADO sempre que o paciente já escolheu o procedimento, pra não misturar turmas de modalidades diferentes.",
				},
				"weekday": {
					Type:        genai.TypeInteger,
					Description: "Dia da semana (0=domingo, 1=segunda, ..., 6=sábado). REQUIRED. Converta o que o paciente disse ('segunda', 'amanhã', etc) pra esse número antes de chamar.",
				},
				"time_filter": {
					Type:        genai.TypeString,
					Description: "Horário específico HH:MM que o paciente pediu. Quando passado, retorna SOMENTE esse horário se livre, ou vazio. Omita pra ver TODOS os horários daquele weekday e oferecer alternativas.",
				},
			},
		},
	})

	r.register(&Tool{
		Name:        "list_partner_plans",
		Description: "Lista convênios/passes ativos da clínica (ex: Gympass Silver, Total Pass Tp2, Namu Pass, Gogood Básico III). Retorna id, network_name, name e max_sessions_per_week. Use quando o paciente disser que tem um convênio para resolver o nome em partner_plan_id antes de chamar enroll_patient_in_class. Se o convênio do paciente não aparecer aqui, a clínica não aceita esse passe — diga isso em vez de inventar.",
		Parameters: &genai.Schema{
			Type:       genai.TypeObject,
			Properties: map[string]*genai.Schema{},
		},
	})

	r.register(&Tool{
		Name:        "confirm_class_attendance",
		Description: "Confirma a participação do paciente atual na PRÓXIMA aula da turma em grupo onde ele está matriculado (Pilates etc.). Sem parâmetros — o sistema resolve qual é a próxima aula a partir das matrículas ativas do paciente. Use SOMENTE quando o paciente confirmar explicitamente (ex: 'sim', 'vou', 'confirmo') que vai à próxima aula. NUNCA chame de forma especulativa. A tool retorna data/horário/turma — repasse esses valores na sua resposta. Se a tool não estiver no seu toolset (clínica desativou) ou retornar erro (sem matrícula ativa, sem aula futura), use complete_handoff_to_human ou explique ao paciente que precisa falar com a recepção.",
		Parameters: &genai.Schema{
			Type:       genai.TypeObject,
			Properties: map[string]*genai.Schema{},
		},
	})

	r.register(&Tool{
		Name:        "present_enrollment_summary",
		Description: "Monta a mensagem de confirmação de matrícula em turma (Pilates etc). VOCÊ DEVE usar esta tool em vez de escrever o resumo como texto livre — o sistema valida que cada slot_id realmente existe na turma E está ativo, formata o markdown server-side, e devolve em summary_text. Sua única tarefa após esta tool: enviar o summary_text VERBATIM ao paciente, sem reescrever, sem adicionar nada antes ou depois (exceto se faltar 'Posso confirmar?' na ponta — nesse caso adicione). Se a tool retornar erro 'slot X não encontrado', você inventou o slot — volte ao passo (3) e re-pergunte ao paciente quais dias/horários ele quer.",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{"plan_id", "slot_ids"},
			Properties: map[string]*genai.Schema{
				"plan_id": {
					Type:        genai.TypeString,
					Description: "UUID da turma (campo 'plan_id' de get_group_class_slots). REQUIRED.",
				},
				"slot_ids": {
					Type:        genai.TypeArray,
					Items:       &genai.Schema{Type: genai.TypeString},
					Description: "Array com os UUIDs dos slots escolhidos (campo 'slot_id' das chamadas anteriores de get_group_class_slots). REQUIRED. Cada slot é (dia + horário) que o paciente confirmou querer. NÃO invente UUIDs — devem ter vindo de validações reais.",
				},
				"partner_plan_id": {
					Type:        genai.TypeString,
					Description: "UUID do convênio do paciente (campo 'id' de list_partner_plans). Opcional — passe quando o paciente disse que tem convênio. Sem isso, a matrícula é particular.",
				},
				"monthly_amount": {
					Type:        genai.TypeNumber,
					Description: "Valor mensal em reais (do FAQ, conforme frequência escolhida). Use SOMENTE pra particular. Convênio: omita.",
				},
				"billing_day": {
					Type:        genai.TypeInteger,
					Description: "Dia do mês 1-31 pra cobrança da mensalidade. Necessário pra particular mensal. Convênio: omita.",
				},
			},
		},
	})

	r.register(&Tool{
		Name:        "enroll_patient_in_class",
		Description: "Matricula o paciente atual numa turma em grupo (Pilates, etc.) nos slots escolhidos. Chame SOMENTE depois de ter (1) confirmado a modalidade com o paciente, (2) escolhido os slots reais via get_group_class_slots, e (3) recebido confirmação explícita do paciente sobre os horários e o valor mensal. patient_id NÃO é parâmetro — é resolvido automaticamente pelo paciente da conversa. Se a tool não estiver no seu toolset (clínica desativou), use complete_handoff_to_human depois de enviar o resumo bold.",
		Parameters: &genai.Schema{
			Type:     genai.TypeObject,
			Required: []string{"plan_id", "slot_ids"},
			Properties: map[string]*genai.Schema{
				"plan_id": {
					Type:        genai.TypeString,
					Description: "UUID da turma (campo 'plan_id' de get_group_class_slots). REQUIRED.",
				},
				"slot_ids": {
					Type:        genai.TypeArray,
					Items:       &genai.Schema{Type: genai.TypeString},
					Description: "Array com os UUIDs dos slots escolhidos (campo 'slot_id' de get_group_class_slots). REQUIRED. Cada slot é (dia da semana + horário). Mínimo 1.",
				},
				"partner_plan_id": {
					Type:        genai.TypeString,
					Description: "UUID do convênio do paciente (campo 'id' de list_partner_plans). Opcional — passe quando o paciente disse que tem Gympass/Total Pass/etc. Sem esse campo, a matrícula é particular (paga direto).",
				},
				"monthly_amount": {
					Type:        genai.TypeNumber,
					Description: "Valor mensal em reais (ex: 195 para 3x/sem na tabela do FAQ). Use quando a matrícula for particular (sem partner_plan_id). Para matrículas via convênio, omita.",
				},
				"billing_day": {
					Type:        genai.TypeInteger,
					Description: "Dia do mês (1-31) em que a mensalidade é cobrada. Necessário quando a matrícula é mensal direto (sem convênio). Pergunte ao paciente qual dia prefere ou use o dia da matrícula.",
				},
			},
		},
	})
}

// FilteredOpenRouterFunctionDeclarations returns OpenRouter-compatible
// declarations after applying the (optional) allow whitelist and (optional)
// disallow blacklist. Order: whitelist first (or all if empty), then remove
// blacklisted names. Used by per-request tool gating — handoff mode passes
// an explicit whitelist, every request passes the clinic-flag blacklist.
func (r *Registry) FilteredOpenRouterFunctionDeclarations(allowed, disallowed []string) []OpenRouterTool {
	disallowSet := setOf(disallowed)
	allowSet := setOf(allowed)
	useWhitelist := len(allowed) > 0

	out := make([]OpenRouterTool, 0, len(r.tools))
	for _, tool := range r.tools {
		if useWhitelist {
			if _, ok := allowSet[tool.Name]; !ok {
				continue
			}
		}
		if _, ok := disallowSet[tool.Name]; ok {
			continue
		}
		out = append(out, OpenRouterTool{
			Type: "function",
			Function: OpenRouterFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  convertGeminiSchemaToOpenRouter(tool.Parameters),
			},
		})
	}
	return out
}

// FilteredGeminiFunctionDeclarations is the Gemini variant — same semantics.
func (r *Registry) FilteredGeminiFunctionDeclarations(allowed, disallowed []string) []*genai.FunctionDeclaration {
	disallowSet := setOf(disallowed)
	allowSet := setOf(allowed)
	useWhitelist := len(allowed) > 0

	out := make([]*genai.FunctionDeclaration, 0, len(r.tools))
	for _, tool := range r.tools {
		if useWhitelist {
			if _, ok := allowSet[tool.Name]; !ok {
				continue
			}
		}
		if _, ok := disallowSet[tool.Name]; ok {
			continue
		}
		out = append(out, &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		})
	}
	return out
}

func setOf(names []string) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}
	s := make(map[string]struct{}, len(names))
	for _, n := range names {
		s[n] = struct{}{}
	}
	return s
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
		Description: "Lista os procedimentos/serviços oferecidos pela clínica com preço e duração. Use para responder perguntas como 'quanto custa uma massoterapia?', 'quais serviços vocês oferecem?', 'qual o valor da consulta de X?'. Retorna array com: id, name, price (em reais), duration_minutes, specialty_name. Pra perguntas tipo 'vocês têm/fazem X?' onde X pode ser nome de procedimento OU especialidade, prefira o parâmetro name_contains (filtra por substring no nome) — é mais robusto que specialty pois pega procedimentos com nomes parecidos em qualquer especialidade.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"specialty": {
					Type:        genai.TypeString,
					Description: "Filtrar por nome EXATO da especialidade (opcional), conforme retornado por list_specialties. Use SÓ pra perguntas claramente sobre uma área inteira (ex: 'todos procedimentos de fisioterapia'). NÃO use pra perguntas tipo 'vocês têm RPG?' — use name_contains pra esse caso.",
				},
				"name_contains": {
					Type: genai.TypeArray,
					Items: &genai.Schema{
						Type: genai.TypeString,
					},
					Description: "Lista de termos pra filtrar por substring no nome do procedimento. Case-insensitive E accent-insensitive: 'avaliacao' casa com 'Avaliação'. OR semântico: um procedimento entra se qualquer termo da lista estiver no nome. Use pra cobrir sigla + nome completo numa só chamada (ex: ['RPG', 'Reeducação Postural Global']) ou variações que o paciente pode usar (ex: ['drenagem', 'linfática']). Pode passar 1 termo só (['RPG']) se for específico. Retorna TODOS os procedimentos que casam, em qualquer especialidade. Mais robusto que specialty pra termos ambíguos.",
				},
			},
		},
	})
}
