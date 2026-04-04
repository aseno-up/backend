package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/pkg/lead/crm"
	"github.com/aseno/backend/pkg/lead/model"
)

const notionAPIBase = "https://api.notion.com/v1"
const notionVersion = "2022-06-28"

type notionAdapter struct {
	apiKey     string
	databaseID string
	client     *http.Client
}

func NewNotionAdapter(apiKey, databaseID string) crm.CRMProvider {
	return &notionAdapter{
		apiKey:     apiKey,
		databaseID: databaseID,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (n *notionAdapter) CreatePage(ctx context.Context, lead *model.Lead) (string, error) {
	props := n.buildProperties(lead)

	body := map[string]interface{}{
		"parent": map[string]interface{}{
			"database_id": n.databaseID,
		},
		"properties": props,
	}

	if lead.InitialMessage != "" {
		body["children"] = buildPageContent(lead.InitialMessage)
	}

	respBody, err := n.doRequest(ctx, "POST", notionAPIBase+"/pages", body)
	if err != nil {
		logger.Printf("[Notion CRM] CreatePage error for lead %s: %v", lead.ID, err)
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		logger.Printf("[Notion CRM] Failed to parse CreatePage response: %v", err)
		return "", fmt.Errorf("failed to parse Notion response: %w", err)
	}

	logger.Printf("[Notion CRM] Created page %s for lead %s (%s)", result.ID, lead.ID, lead.ClinicName)
	return result.ID, nil
}

func (n *notionAdapter) UpdatePage(ctx context.Context, externalID string, lead *model.Lead) error {
	props := n.buildProperties(lead)

	body := map[string]interface{}{
		"properties": props,
	}

	_, err := n.doRequest(ctx, "PATCH", notionAPIBase+"/pages/"+externalID, body)
	if err != nil {
		logger.Printf("[Notion CRM] UpdatePage error for external_id %s: %v", externalID, err)
		return err
	}

	// Update page content (initial message) by replacing all blocks
	if lead.InitialMessage != "" {
		n.replacePageContent(ctx, externalID, lead.InitialMessage)
	}

	logger.Printf("[Notion CRM] Updated page %s for lead %s", externalID, lead.ID)
	return nil
}

func (n *notionAdapter) replacePageContent(ctx context.Context, pageID, message string) {
	// 1. Get existing blocks
	respBody, err := n.doRequest(ctx, "GET", notionAPIBase+"/blocks/"+pageID+"/children?page_size=100", nil)
	if err != nil {
		return
	}

	var blockResult struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &blockResult); err != nil {
		return
	}

	// 2. Delete old blocks
	for _, block := range blockResult.Results {
		_, _ = n.doRequest(ctx, "DELETE", notionAPIBase+"/blocks/"+block.ID, nil)
	}

	// 3. Append new content
	appendBody := map[string]interface{}{
		"children": buildPageContent(message),
	}
	_, _ = n.doRequest(ctx, "PATCH", notionAPIBase+"/blocks/"+pageID+"/children", appendBody)
}

func (n *notionAdapter) buildProperties(lead *model.Lead) map[string]interface{} {
	props := map[string]interface{}{}

	// Title: "Nome da clínica"
	props["Nome da clínica"] = titleProp(lead.ClinicName)

	// Phone: "Telefone"
	if lead.Phone != "" {
		props["Telefone"] = map[string]interface{}{
			"phone_number": lead.Phone,
		}
	}

	// Select: "Cidade"
	if lead.City != "" {
		props["Cidade"] = selectProp(lead.City)
	}

	// Select: "Tipo"
	if lead.Type != "" {
		props["Tipo"] = selectProp(mapLeadType(lead.Type))
	}

	// Select: "Canal de entrada"
	if lead.Channel != "" {
		props["Canal de entrada"] = selectProp(mapChannel(lead.Channel))
	}

	// Select: "Origem"
	if lead.Source != "" {
		props["Origem"] = selectProp(mapSource(lead.Source))
	}

	// Select: "Status"
	if lead.Status != "" {
		props["Status"] = selectProp(mapStatus(lead.Status))
	}

	// Select: "Score de dor"
	if lead.PainScore != "" {
		props["Score de dor"] = selectProp(mapPainScore(lead.PainScore))
	}

	// Select: "Plano de interesse"
	if lead.InterestPlan != "" {
		props["Plano de interesse"] = selectProp(lead.InterestPlan)
	}

	// Select: "Motivo cancelamento"
	if lead.CancellationReason != "" {
		props["Motivo cancelamento"] = selectProp(lead.CancellationReason)
	}

	// Date: "Data primeiro contato"
	if lead.FirstContactAt != nil {
		props["Data primeiro contato"] = dateProp(*lead.FirstContactAt)
	}

	// Date: "Data próximo follow-up"
	if lead.NextFollowUpAt != nil {
		props["Data próximo follow-up"] = dateProp(*lead.NextFollowUpAt)
	}

	// Date: "Data início trial"
	if lead.TrialStartAt != nil {
		props["Data início trial"] = dateProp(*lead.TrialStartAt)
	}

	// Date: "Data fim trial"
	if lead.TrialEndAt != nil {
		props["Data fim trial"] = dateProp(*lead.TrialEndAt)
	}

	// Number: "MRR gerado"
	if lead.MRR > 0 {
		props["MRR gerado"] = map[string]interface{}{
			"number": lead.MRR,
		}
	}

	// Rich text: "Notas"
	if lead.Notes != "" {
		props["Notas"] = richTextProp(lead.Notes)
	}

	// Rich text: "Responsável" (contact_name)
	if lead.ContactName != "" {
		props["Responsável"] = richTextProp(lead.ContactName)
	}

	// Rich text: "Site"
	if lead.Site != "" {
		props["Site"] = richTextProp(lead.Site)
	}

	// Rich text: "Instagram"
	if lead.Instagram != "" {
		props["Instagram"] = richTextProp(lead.Instagram)
	}

	// Select: "Canal de contato"
	if lead.ContactChannel != "" {
		props["Canal de contato"] = selectProp(lead.ContactChannel)
	}

	// Checkbox: "Trial ativo"
	props["Trial ativo"] = checkboxProp(lead.Status == model.LeadStatusTrialAtivo)

	// Checkbox: "Converteu"
	props["Converteu"] = checkboxProp(lead.Status == model.LeadStatusConvertido)

	// Checkbox: "Cancelou"
	props["Cancelou"] = checkboxProp(lead.CancelledAt != nil)

	return props
}

func (n *notionAdapter) doRequest(ctx context.Context, method, url string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+n.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", notionVersion)

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Printf("[Notion CRM] API error %d: %s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("Notion API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// --- Page content (body blocks) ---

func buildPageContent(message string) []map[string]interface{} {
	var blocks []map[string]interface{}

	// Heading
	blocks = append(blocks, map[string]interface{}{
		"object": "block",
		"type":   "heading_2",
		"heading_2": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"type": "text", "text": map[string]interface{}{"content": "Mensagem Inicial"}},
			},
		},
	})

	// Split message into chunks of 2000 chars (Notion block limit)
	for len(message) > 0 {
		chunk := message
		if len(chunk) > 2000 {
			chunk = chunk[:2000]
		}
		message = message[len(chunk):]

		blocks = append(blocks, map[string]interface{}{
			"object": "block",
			"type":   "paragraph",
			"paragraph": map[string]interface{}{
				"rich_text": []map[string]interface{}{
					{"type": "text", "text": map[string]interface{}{"content": chunk}},
				},
			},
		})
	}

	return blocks
}

// --- Property helpers ---

func titleProp(text string) map[string]interface{} {
	return map[string]interface{}{
		"title": []map[string]interface{}{
			{"text": map[string]interface{}{"content": text}},
		},
	}
}

func richTextProp(text string) map[string]interface{} {
	// Notion rich_text has a 2000 char limit per block
	if len(text) > 2000 {
		text = text[:2000]
	}
	return map[string]interface{}{
		"rich_text": []map[string]interface{}{
			{"text": map[string]interface{}{"content": text}},
		},
	}
}

func selectProp(name string) map[string]interface{} {
	return map[string]interface{}{
		"select": map[string]interface{}{"name": name},
	}
}

func dateProp(t time.Time) map[string]interface{} {
	return map[string]interface{}{
		"date": map[string]interface{}{
			"start": t.Format("2006-01-02"),
		},
	}
}

func checkboxProp(value bool) map[string]interface{} {
	return map[string]interface{}{
		"checkbox": value,
	}
}

// --- Value mappers (lead enum -> Notion select option name) ---

func mapLeadType(t model.LeadType) string {
	switch t {
	case model.LeadTypeEstetica:
		return "Estética"
	case model.LeadTypeFisioterapia:
		return "Fisioterapia"
	case model.LeadTypeMisto:
		return "Misto"
	case model.LeadTypePsicologia:
		return "Psicologia"
	default:
		return string(t)
	}
}

func mapChannel(c model.LeadChannel) string {
	switch c {
	case model.LeadChannelOutbound:
		return "Outbound"
	case model.LeadChannelInbound:
		return "Inbound"
	default:
		return string(c)
	}
}

func mapSource(s model.LeadSource) string {
	switch s {
	case model.LeadSourceGoogleMaps:
		return "Google Maps"
	case model.LeadSourceInstagram:
		return "Instagram"
	case model.LeadSourceIndicacao:
		return "Indicação"
	case model.LeadSourceAnuncio:
		return "Anúncio"
	default:
		return string(s)
	}
}

func mapStatus(s model.LeadStatus) string {
	switch s {
	case model.LeadStatusIdentificado:
		return "Identificado"
	case model.LeadStatusContactado:
		return "Contactado"
	case model.LeadStatusRespondeu:
		return "Respondeu"
	case model.LeadStatusDemoAgendada:
		return "Demo agendada"
	case model.LeadStatusTrialAtivo:
		return "Trial ativo"
	case model.LeadStatusNegociando:
		return "Negociando"
	case model.LeadStatusConvertido:
		return "Convertido"
	case model.LeadStatusSemResposta:
		return "Sem resposta"
	case model.LeadStatusDescartado:
		return "Descartado"
	case model.LeadStatusIsca:
		return "Isca"
	default:
		return string(s)
	}
}

func mapPainScore(p model.PainScore) string {
	switch p {
	case model.PainScoreAlto:
		return "Alto"
	case model.PainScoreMedio:
		return "Médio"
	case model.PainScoreBaixo:
		return "Baixo"
	default:
		return string(p)
	}
}
