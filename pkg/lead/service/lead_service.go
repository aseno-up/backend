package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/lead/crm"
	"github.com/aseno/backend/pkg/lead/model"
	"github.com/aseno/backend/pkg/lead/repository"
	"github.com/aseno/backend/pkg/pagination"
)

// LeadService defines the interface for lead business logic
type LeadService interface {
	Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateLeadRequest) (*model.LeadResponse, error)
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.LeadResponse, error)
	List(ctx context.Context, clinicID uuid.UUID, filter model.LeadFilter) (pagination.Response[*model.LeadResponse], error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateLeadRequest) (*model.LeadResponse, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
}

type leadService struct {
	repo        repository.LeadRepository
	crmProvider crm.CRMProvider
}

func NewLeadService(repo repository.LeadRepository, crmProvider crm.CRMProvider) LeadService {
	return &leadService{
		repo:        repo,
		crmProvider: crmProvider,
	}
}

func (s *leadService) Create(ctx context.Context, clinicID uuid.UUID, req *model.CreateLeadRequest) (*model.LeadResponse, error) {
	status := req.Status
	if status == "" {
		status = model.LeadStatusIdentificado
	}

	lead := &model.Lead{
		ClinicID:           clinicID,
		ClinicName:         req.ClinicName,
		ContactName:        req.ContactName,
		Phone:              req.Phone,
		City:               req.City,
		Type:               req.Type,
		Channel:            req.Channel,
		Source:             req.Source,
		Status:             status,
		PainScore:          req.PainScore,
		FirstContactAt:     req.FirstContactAt,
		NextFollowUpAt:     req.NextFollowUpAt,
		TrialStartAt:       req.TrialStartAt,
		TrialEndAt:         req.TrialEndAt,
		InterestPlan:       req.InterestPlan,
		ContractedPlan:     req.ContractedPlan,
		CancellationReason: req.CancellationReason,
		MRR:                req.MRR,
		Site:               req.Site,
		Instagram:          req.Instagram,
		ContactChannel:     req.ContactChannel,
		InitialMessage:     req.InitialMessage,
		Notes:              req.Notes,
	}

	if err := s.repo.Create(ctx, lead); err != nil {
		return nil, err
	}

	s.syncCRM(ctx, lead)

	return lead.ToResponse(), nil
}

func (s *leadService) GetByID(ctx context.Context, id, clinicID uuid.UUID) (*model.LeadResponse, error) {
	lead, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}
	return lead.ToResponse(), nil
}

func (s *leadService) List(ctx context.Context, clinicID uuid.UUID, filter model.LeadFilter) (pagination.Response[*model.LeadResponse], error) {
	leads, total, err := s.repo.List(ctx, clinicID, filter)
	if err != nil {
		return pagination.Response[*model.LeadResponse]{}, err
	}

	data := make([]*model.LeadResponse, len(leads))
	for i, l := range leads {
		data[i] = l.ToResponse()
	}

	return pagination.Response[*model.LeadResponse]{
		Data: data,
		Meta: pagination.NewMeta(filter.GetPage(), filter.GetLimit(), total),
	}, nil
}

func (s *leadService) Update(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdateLeadRequest) (*model.LeadResponse, error) {
	lead, err := s.repo.GetByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	if req.ClinicName != nil {
		lead.ClinicName = *req.ClinicName
	}
	if req.ContactName != nil {
		lead.ContactName = *req.ContactName
	}
	if req.Phone != nil {
		lead.Phone = *req.Phone
	}
	if req.City != nil {
		lead.City = *req.City
	}
	if req.Type != nil {
		lead.Type = *req.Type
	}
	if req.Channel != nil {
		lead.Channel = *req.Channel
	}
	if req.Source != nil {
		lead.Source = *req.Source
	}
	if req.Status != nil {
		lead.Status = *req.Status
	}
	if req.PainScore != nil {
		lead.PainScore = *req.PainScore
	}
	if req.FirstContactAt != nil {
		lead.FirstContactAt = req.FirstContactAt
	}
	if req.NextFollowUpAt != nil {
		lead.NextFollowUpAt = req.NextFollowUpAt
	}
	if req.TrialStartAt != nil {
		lead.TrialStartAt = req.TrialStartAt
	}
	if req.TrialEndAt != nil {
		lead.TrialEndAt = req.TrialEndAt
	}
	if req.ConvertedAt != nil {
		lead.ConvertedAt = req.ConvertedAt
	}
	if req.CancelledAt != nil {
		lead.CancelledAt = req.CancelledAt
	}
	if req.InterestPlan != nil {
		lead.InterestPlan = *req.InterestPlan
	}
	if req.ContractedPlan != nil {
		lead.ContractedPlan = *req.ContractedPlan
	}
	if req.CancellationReason != nil {
		lead.CancellationReason = *req.CancellationReason
	}
	if req.MRR != nil {
		lead.MRR = *req.MRR
	}
	if req.Site != nil {
		lead.Site = *req.Site
	}
	if req.Instagram != nil {
		lead.Instagram = *req.Instagram
	}
	if req.ContactChannel != nil {
		lead.ContactChannel = *req.ContactChannel
	}
	if req.InitialMessage != nil {
		lead.InitialMessage = *req.InitialMessage
	}
	if req.Notes != nil {
		lead.Notes = *req.Notes
	}

	// Link to existing account — auto-set status to convertido
	if req.AccountID != nil {
		wasLinked := lead.AccountID != nil
		lead.AccountID = req.AccountID

		if !wasLinked && req.AccountID != nil {
			now := time.Now()
			lead.Status = model.LeadStatusConvertido
			lead.ConvertedAt = &now
		}
	}

	if err := s.repo.Update(ctx, lead); err != nil {
		return nil, err
	}

	s.syncCRM(ctx, lead)

	return lead.ToResponse(), nil
}

func (s *leadService) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.repo.Delete(ctx, id, clinicID)
}

func (s *leadService) syncCRM(ctx context.Context, lead *model.Lead) {
	if s.crmProvider == nil {
		return
	}

	if lead.ExternalID == "" {
		externalID, err := s.crmProvider.CreatePage(ctx, lead)
		if err == nil && externalID != "" {
			lead.ExternalID = externalID
			_ = s.repo.Update(ctx, lead)
		}
	} else {
		_ = s.crmProvider.UpdatePage(ctx, lead.ExternalID, lead)
	}
}
