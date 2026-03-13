package organization

import (
	"context"

	"github.com/google/uuid"
)

type organizationRepo interface {
	Create(ctx context.Context, id uuid.UUID, name string) (*Organization, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Organization, error)
}

type OrganizationService struct {
	repo organizationRepo
}

func NewOrganizationService(repo organizationRepo) *OrganizationService {
	return &OrganizationService{repo: repo}
}

func (s *OrganizationService) Create(ctx context.Context, id uuid.UUID, name string) (*Organization, error) {
	return s.repo.Create(ctx, id, name)
}

func (s *OrganizationService) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	return s.repo.GetByID(ctx, id)
}
