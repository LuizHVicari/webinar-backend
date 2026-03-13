package organization

import (
	"context"

	"github.com/google/uuid"
)

type OrganizationService struct {
	repo *OrganizationRepository
}

func NewOrganizationService(repo *OrganizationRepository) *OrganizationService {
	return &OrganizationService{repo: repo}
}

func (s *OrganizationService) Create(ctx context.Context, id uuid.UUID, name string) (*Organization, error) {
	return s.repo.Create(ctx, id, name)
}

func (s *OrganizationService) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	return s.repo.GetByID(ctx, id)
}
