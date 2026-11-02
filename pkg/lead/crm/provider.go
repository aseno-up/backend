package crm

import (
	"context"

	"github.com/aseno/backend/pkg/lead/model"
)

// CRMProvider defines the interface for external CRM sync (System -> External only)
type CRMProvider interface {
	CreatePage(ctx context.Context, lead *model.Lead) (externalID string, err error)
	UpdatePage(ctx context.Context, externalID string, lead *model.Lead) error
}
