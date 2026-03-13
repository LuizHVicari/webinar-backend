package organization_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LuizHVicari/webinar-backend/internal/organization"
	"github.com/LuizHVicari/webinar-backend/pkg/testhelper"
	db "github.com/LuizHVicari/webinar-backend/sqlc/generated"
)

func TestOrganizationRepository_Create(t *testing.T) {
	testhelper.TruncateTables(t, sharedPool)
	repo := organization.NewOrganizationRepository(db.New(sharedPool))
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)

	org, err := repo.Create(ctx, id, "Acme Corp")
	require.NoError(t, err)

	assert.Equal(t, id, org.ID)
	assert.Equal(t, "Acme Corp", org.Name)
	assert.False(t, org.CreatedAt.IsZero())
	assert.False(t, org.UpdatedAt.IsZero())
}

func TestOrganizationRepository_GetByID(t *testing.T) {
	testhelper.TruncateTables(t, sharedPool)
	repo := organization.NewOrganizationRepository(db.New(sharedPool))
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = repo.Create(ctx, id, "Acme Corp")
	require.NoError(t, err)

	org, err := repo.GetByID(ctx, id)
	require.NoError(t, err)

	assert.Equal(t, id, org.ID)
	assert.Equal(t, "Acme Corp", org.Name)
}

func TestOrganizationRepository_GetByID_NotFound(t *testing.T) {
	testhelper.TruncateTables(t, sharedPool)
	repo := organization.NewOrganizationRepository(db.New(sharedPool))
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, id)
	assert.ErrorIs(t, err, organization.ErrNotFound)
}
