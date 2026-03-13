package organization_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LuizHVicari/webinar-backend/internal/organization"
)

// fakeOrgRepo implements the organizationRepo interface consumed by OrganizationService.
type fakeOrgRepo struct {
	orgs map[uuid.UUID]*organization.Organization
}

func newFakeOrgRepo() *fakeOrgRepo {
	return &fakeOrgRepo{orgs: make(map[uuid.UUID]*organization.Organization)}
}

func (f *fakeOrgRepo) Create(_ context.Context, id uuid.UUID, name string) (*organization.Organization, error) {
	org := &organization.Organization{ID: id, Name: name, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.orgs[id] = org
	return org, nil
}

func (f *fakeOrgRepo) GetByID(_ context.Context, id uuid.UUID) (*organization.Organization, error) {
	org, ok := f.orgs[id]
	if !ok {
		return nil, organization.ErrNotFound
	}
	return org, nil
}

func TestOrganizationService_Create(t *testing.T) {
	svc := organization.NewOrganizationService(newFakeOrgRepo())
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)

	org, err := svc.Create(ctx, id, "Acme Corp")
	require.NoError(t, err)

	assert.Equal(t, id, org.ID)
	assert.Equal(t, "Acme Corp", org.Name)
}

func TestOrganizationService_GetByID(t *testing.T) {
	svc := organization.NewOrganizationService(newFakeOrgRepo())
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = svc.Create(ctx, id, "Acme Corp")
	require.NoError(t, err)

	org, err := svc.GetByID(ctx, id)
	require.NoError(t, err)

	assert.Equal(t, id, org.ID)
}

func TestOrganizationService_GetByID_NotFound(t *testing.T) {
	svc := organization.NewOrganizationService(newFakeOrgRepo())
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)

	_, err = svc.GetByID(ctx, id)
	assert.ErrorIs(t, err, organization.ErrNotFound)
}
