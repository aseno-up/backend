package model

import (
	"time"

	"github.com/google/uuid"
)

// LeadType represents the type of clinic
type LeadType string

const (
	LeadTypeEstetica     LeadType = "estetica"
	LeadTypeFisioterapia LeadType = "fisioterapia"
	LeadTypeMisto        LeadType = "misto"
	LeadTypePsicologia   LeadType = "psicologia"
)

// LeadChannel represents the acquisition channel
type LeadChannel string

const (
	LeadChannelOutbound LeadChannel = "outbound"
	LeadChannelInbound  LeadChannel = "inbound"
)

// LeadSource represents the lead source
type LeadSource string

const (
	LeadSourceGoogleMaps LeadSource = "google_maps"
	LeadSourceInstagram  LeadSource = "instagram"
	LeadSourceIndicacao  LeadSource = "indicacao"
	LeadSourceAnuncio    LeadSource = "anuncio"
)

// LeadStatus represents the current status of the lead
type LeadStatus string

const (
	LeadStatusIdentificado  LeadStatus = "identificado"
	LeadStatusContactado    LeadStatus = "contactado"
	LeadStatusRespondeu     LeadStatus = "respondeu"
	LeadStatusDemoAgendada  LeadStatus = "demo_agendada"
	LeadStatusTrialAtivo    LeadStatus = "trial_ativo"
	LeadStatusNegociando    LeadStatus = "negociando"
	LeadStatusConvertido    LeadStatus = "convertido"
	LeadStatusSemResposta   LeadStatus = "sem_resposta"
	LeadStatusDescartado    LeadStatus = "descartado"
)

// PainScore represents the pain level of the lead
type PainScore string

const (
	PainScoreAlto  PainScore = "alto"
	PainScoreMedio PainScore = "medio"
	PainScoreBaixo PainScore = "baixo"
)

// Lead represents a CRM lead
type Lead struct {
	ID                 uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClinicID           uuid.UUID   `json:"clinic_id" gorm:"type:uuid;not null;index"`
	AccountID          *uuid.UUID  `json:"account_id,omitempty" gorm:"type:uuid;index"`
	ClinicName         string      `json:"clinic_name" gorm:"not null"`
	ContactName        string      `json:"contact_name" gorm:"not null"`
	Phone              string      `json:"phone"`
	City               string      `json:"city"`
	Type               LeadType    `json:"type"`
	Channel            LeadChannel `json:"channel"`
	Source             LeadSource  `json:"source"`
	Status             LeadStatus  `json:"status" gorm:"default:'identificado';index"`
	PainScore          PainScore   `json:"pain_score"`
	FirstContactAt     *time.Time  `json:"first_contact_at,omitempty"`
	NextFollowUpAt     *time.Time  `json:"next_follow_up_at,omitempty"`
	TrialStartAt       *time.Time  `json:"trial_start_at,omitempty"`
	TrialEndAt         *time.Time  `json:"trial_end_at,omitempty"`
	ConvertedAt        *time.Time  `json:"converted_at,omitempty"`
	CancelledAt        *time.Time  `json:"cancelled_at,omitempty"`
	InterestPlan       string      `json:"interest_plan"`
	ContractedPlan     string      `json:"contracted_plan"`
	CancellationReason string      `json:"cancellation_reason"`
	MRR                float64     `json:"mrr" gorm:"column:mrr;default:0"`
	Site               string      `json:"site"`
	Instagram          string      `json:"instagram"`
	ContactChannel     string      `json:"contact_channel"`
	InitialMessage     string      `json:"initial_message" gorm:"type:text"`
	Notes              string      `json:"notes" gorm:"type:text"`
	ExternalID         string      `json:"external_id" gorm:"index"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	DeletedAt          *time.Time  `json:"deleted_at,omitempty" gorm:"index"`
}

func (Lead) TableName() string {
	return "leads"
}

// LeadResponse represents the API response for a lead
type LeadResponse struct {
	ID                 uuid.UUID   `json:"id"`
	ClinicID           uuid.UUID   `json:"clinic_id"`
	AccountID          *uuid.UUID  `json:"account_id,omitempty"`
	ClinicName         string      `json:"clinic_name"`
	ContactName        string      `json:"contact_name"`
	Phone              string      `json:"phone"`
	City               string      `json:"city"`
	Type               LeadType    `json:"type"`
	Channel            LeadChannel `json:"channel"`
	Source             LeadSource  `json:"source"`
	Status             LeadStatus  `json:"status"`
	PainScore          PainScore   `json:"pain_score"`
	FirstContactAt     *time.Time  `json:"first_contact_at,omitempty"`
	NextFollowUpAt     *time.Time  `json:"next_follow_up_at,omitempty"`
	TrialStartAt       *time.Time  `json:"trial_start_at,omitempty"`
	TrialEndAt         *time.Time  `json:"trial_end_at,omitempty"`
	ConvertedAt        *time.Time  `json:"converted_at,omitempty"`
	CancelledAt        *time.Time  `json:"cancelled_at,omitempty"`
	InterestPlan       string      `json:"interest_plan"`
	ContractedPlan     string      `json:"contracted_plan"`
	CancellationReason string      `json:"cancellation_reason"`
	MRR                float64     `json:"mrr"`
	Site               string      `json:"site"`
	Instagram          string      `json:"instagram"`
	ContactChannel     string      `json:"contact_channel"`
	InitialMessage     string      `json:"initial_message"`
	Notes              string      `json:"notes"`
	ExternalID         string      `json:"external_id"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

func (l *Lead) ToResponse() *LeadResponse {
	return &LeadResponse{
		ID:                 l.ID,
		ClinicID:           l.ClinicID,
		AccountID:          l.AccountID,
		ClinicName:         l.ClinicName,
		ContactName:        l.ContactName,
		Phone:              l.Phone,
		City:               l.City,
		Type:               l.Type,
		Channel:            l.Channel,
		Source:             l.Source,
		Status:             l.Status,
		PainScore:          l.PainScore,
		FirstContactAt:     l.FirstContactAt,
		NextFollowUpAt:     l.NextFollowUpAt,
		TrialStartAt:       l.TrialStartAt,
		TrialEndAt:         l.TrialEndAt,
		ConvertedAt:        l.ConvertedAt,
		CancelledAt:        l.CancelledAt,
		InterestPlan:       l.InterestPlan,
		ContractedPlan:     l.ContractedPlan,
		CancellationReason: l.CancellationReason,
		MRR:                l.MRR,
		Site:               l.Site,
		Instagram:          l.Instagram,
		ContactChannel:     l.ContactChannel,
		InitialMessage:     l.InitialMessage,
		Notes:              l.Notes,
		ExternalID:         l.ExternalID,
		CreatedAt:          l.CreatedAt,
		UpdatedAt:          l.UpdatedAt,
	}
}

// CreateLeadRequest represents the request to create a lead
type CreateLeadRequest struct {
	ClinicName         string      `json:"clinic_name" binding:"required"`
	ContactName        string      `json:"contact_name" binding:"required"`
	Phone              string      `json:"phone"`
	City               string      `json:"city"`
	Type               LeadType    `json:"type" binding:"omitempty,oneof=estetica fisioterapia misto psicologia"`
	Channel            LeadChannel `json:"channel" binding:"omitempty,oneof=outbound inbound"`
	Source             LeadSource  `json:"source" binding:"omitempty,oneof=google_maps instagram indicacao anuncio"`
	Status             LeadStatus  `json:"status" binding:"omitempty,oneof=identificado contactado respondeu demo_agendada trial_ativo negociando convertido sem_resposta descartado"`
	PainScore          PainScore   `json:"pain_score" binding:"omitempty,oneof=alto medio baixo"`
	FirstContactAt     *time.Time  `json:"first_contact_at"`
	NextFollowUpAt     *time.Time  `json:"next_follow_up_at"`
	TrialStartAt       *time.Time  `json:"trial_start_at"`
	TrialEndAt         *time.Time  `json:"trial_end_at"`
	InterestPlan       string      `json:"interest_plan"`
	ContractedPlan     string      `json:"contracted_plan"`
	CancellationReason string      `json:"cancellation_reason"`
	MRR                float64     `json:"mrr"`
	Site               string      `json:"site"`
	Instagram          string      `json:"instagram"`
	ContactChannel     string      `json:"contact_channel"`
	InitialMessage     string      `json:"initial_message"`
	Notes              string      `json:"notes"`
}

// UpdateLeadRequest represents the request to update a lead
type UpdateLeadRequest struct {
	AccountID          *uuid.UUID   `json:"account_id"`
	ClinicName         *string      `json:"clinic_name"`
	ContactName        *string      `json:"contact_name"`
	Phone              *string      `json:"phone"`
	City               *string      `json:"city"`
	Type               *LeadType    `json:"type" binding:"omitempty,oneof=estetica fisioterapia misto psicologia"`
	Channel            *LeadChannel `json:"channel" binding:"omitempty,oneof=outbound inbound"`
	Source             *LeadSource  `json:"source" binding:"omitempty,oneof=google_maps instagram indicacao anuncio"`
	Status             *LeadStatus  `json:"status" binding:"omitempty,oneof=identificado contactado respondeu demo_agendada trial_ativo negociando convertido sem_resposta descartado"`
	PainScore          *PainScore   `json:"pain_score" binding:"omitempty,oneof=alto medio baixo"`
	FirstContactAt     *time.Time   `json:"first_contact_at"`
	NextFollowUpAt     *time.Time   `json:"next_follow_up_at"`
	TrialStartAt       *time.Time   `json:"trial_start_at"`
	TrialEndAt         *time.Time   `json:"trial_end_at"`
	ConvertedAt        *time.Time   `json:"converted_at"`
	CancelledAt        *time.Time   `json:"cancelled_at"`
	InterestPlan       *string      `json:"interest_plan"`
	ContractedPlan     *string      `json:"contracted_plan"`
	CancellationReason *string      `json:"cancellation_reason"`
	MRR                *float64     `json:"mrr"`
	Site               *string      `json:"site"`
	Instagram          *string      `json:"instagram"`
	ContactChannel     *string      `json:"contact_channel"`
	InitialMessage     *string      `json:"initial_message"`
	Notes              *string      `json:"notes"`
}

// LeadFilter represents filter parameters for listing leads
type LeadFilter struct {
	Page     int        `form:"page"`
	Limit    int        `form:"limit"`
	Search   string     `form:"search"`
	Status   LeadStatus `form:"status"`
	Channel  LeadChannel `form:"channel"`
	Source   LeadSource  `form:"source"`
	PainScore PainScore  `form:"pain_score"`
	DateFrom string     `form:"date_from"`
	DateTo   string     `form:"date_to"`
	SortBy   string     `form:"sort_by"`
}

func (f *LeadFilter) GetLimit() int {
	if f.Limit <= 0 {
		return 20
	}
	if f.Limit > 100 {
		return 100
	}
	return f.Limit
}

func (f *LeadFilter) GetPage() int {
	if f.Page <= 0 {
		return 1
	}
	return f.Page
}

func (f *LeadFilter) GetOffset() int {
	return (f.GetPage() - 1) * f.GetLimit()
}
