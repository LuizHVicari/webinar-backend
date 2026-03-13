package user_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LuizHVicari/webinar-backend/internal/organization"
	"github.com/LuizHVicari/webinar-backend/internal/user"
	"github.com/LuizHVicari/webinar-backend/pkg/testhelper"
	db "github.com/LuizHVicari/webinar-backend/sqlc/generated"
)

func TestUserRepository_Create(t *testing.T) {
	pool := testhelper.NewPostgres(t)
	repo := user.NewUserRepository(db.New(pool))
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)

	u, err := repo.Create(ctx, id, identityID)
	require.NoError(t, err)

	assert.Equal(t, id, u.ID)
	assert.Equal(t, identityID, u.IdentityID)
	assert.Nil(t, u.OrganizationID)
	assert.False(t, u.CreatedAt.IsZero())
}

func TestUserRepository_GetByID(t *testing.T) {
	pool := testhelper.NewPostgres(t)
	repo := user.NewUserRepository(db.New(pool))
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = repo.Create(ctx, id, identityID)
	require.NoError(t, err)

	u, err := repo.GetByID(ctx, id)
	require.NoError(t, err)

	assert.Equal(t, id, u.ID)
	assert.Equal(t, identityID, u.IdentityID)
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	pool := testhelper.NewPostgres(t)
	repo := user.NewUserRepository(db.New(pool))
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, id)
	assert.ErrorIs(t, err, user.ErrNotFound)
}

func TestUserRepository_GetByIdentityID(t *testing.T) {
	pool := testhelper.NewPostgres(t)
	repo := user.NewUserRepository(db.New(pool))
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = repo.Create(ctx, id, identityID)
	require.NoError(t, err)

	u, err := repo.GetByIdentityID(ctx, identityID)
	require.NoError(t, err)

	assert.Equal(t, id, u.ID)
	assert.Equal(t, identityID, u.IdentityID)
}

func TestUserRepository_GetByIdentityID_NotFound(t *testing.T) {
	pool := testhelper.NewPostgres(t)
	repo := user.NewUserRepository(db.New(pool))
	ctx := context.Background()

	identityID, err := uuid.NewV7()
	require.NoError(t, err)

	_, err = repo.GetByIdentityID(ctx, identityID)
	assert.ErrorIs(t, err, user.ErrNotFound)
}

func TestUserRepository_UpdateOrganization(t *testing.T) {
	pool := testhelper.NewPostgres(t)
	queries := db.New(pool)
	repo := user.NewUserRepository(queries)
	orgRepo := organization.NewOrganizationRepository(queries)
	ctx := context.Background()

	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = orgRepo.Create(ctx, orgID, "Acme Corp")
	require.NoError(t, err)

	id, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = repo.Create(ctx, id, identityID)
	require.NoError(t, err)

	updated, err := repo.UpdateOrganization(ctx, id, orgID)
	require.NoError(t, err)

	require.NotNil(t, updated.OrganizationID)
	assert.Equal(t, orgID, *updated.OrganizationID)
}
