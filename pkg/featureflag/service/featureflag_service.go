package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/featureflag/model"
	"github.com/aseno/backend/pkg/featureflag/repository"
)

// FeatureFlagService defines the interface for feature flag operations
type FeatureFlagService interface {
	Create(ctx context.Context, req *model.CreateFeatureFlagRequest) (*model.FeatureFlag, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateFeatureFlagRequest) (*model.FeatureFlag, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) (*model.FeatureFlagResponse, error)
	IsEnabled(ctx context.Context, key string, clinicID uuid.UUID) bool
}

type featureFlagService struct {
	repo repository.FeatureFlagRepository
}

// NewFeatureFlagService creates a new feature flag service
func NewFeatureFlagService(repo repository.FeatureFlagRepository) FeatureFlagService {
	return &featureFlagService{repo: repo}
}

func (s *featureFlagService) Create(ctx context.Context, req *model.CreateFeatureFlagRequest) (*model.FeatureFlag, error) {
	flag := &model.FeatureFlag{
		Key:         req.Key,
		Description: req.Description,
		Enabled:     req.Enabled,
	}

	if req.ClinicID != "" {
		clinicID, err := uuid.Parse(req.ClinicID)
		if err == nil {
			flag.ClinicID = &clinicID
		}
	}

	if err := s.repo.Create(ctx, flag); err != nil {
		return nil, err
	}

	return flag, nil
}

func (s *featureFlagService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateFeatureFlagRequest) (*model.FeatureFlag, error) {
	flag, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Description != nil {
		flag.Description = *req.Description
	}
	if req.Enabled != nil {
		flag.Enabled = *req.Enabled
	}

	if err := s.repo.Update(ctx, flag); err != nil {
		return nil, err
	}

	return flag, nil
}

func (s *featureFlagService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *featureFlagService) List(ctx context.Context) (*model.FeatureFlagResponse, error) {
	flags, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	return &model.FeatureFlagResponse{Data: flags}, nil
}

// IsEnabled checks if a feature is enabled for a specific clinic
func (s *featureFlagService) IsEnabled(ctx context.Context, key string, clinicID uuid.UUID) bool {
	enabled, _ := s.repo.IsEnabled(ctx, key, clinicID)
	return enabled
}
