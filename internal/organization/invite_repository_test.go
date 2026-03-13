package organization_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LuizHVicari/webinar-backend/internal/organization"
	"github.com/LuizHVicari/webinar-backend/pkg/testhelper"
	db "github.com/LuizHVicari/webinar-backend/sqlc/generated"
)

type inviteFixtures struct {
	repo      *organization.InviteRepository
	orgID     uuid.UUID
	inviterID uuid.UUID
}

func newInviteFixtures(t *testing.T) inviteFixtures {
	t.Helper()
	pool := testhelper.NewPostgres(t)
	queries := db.New(pool)
	ctx := context.Background()

	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = organization.NewOrganizationRepository(queries).Create(ctx, orgID, "Test Org")
	require.NoError(t, err)

	inviterID, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = queries.CreateUser(ctx, db.CreateUserParams{ID: inviterID, IdentityID: identityID})
	require.NoError(t, err)

	return inviteFixtures{
		repo:      organization.NewInviteRepository(queries),
		orgID:     orgID,
		inviterID: inviterID,
	}
}

func TestInviteRepository_Create(t *testing.T) {
	f := newInviteFixtures(t)
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	invite, err := f.repo.Create(ctx, id, f.orgID, f.inviterID, "user@example.com", organization.RoleDeveloper, expiresAt)
	require.NoError(t, err)

	assert.Equal(t, id, invite.ID)
	assert.Equal(t, f.orgID, invite.OrganizationID)
	assert.Equal(t, f.inviterID, invite.InvitedBy)
	assert.Equal(t, "user@example.com", invite.Email)
	assert.Equal(t, organization.RoleDeveloper, invite.Role)
	assert.False(t, invite.Accepted)
}

func TestInviteRepository_GetByID(t *testing.T) {
	f := newInviteFixtures(t)
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = f.repo.Create(ctx, id, f.orgID, f.inviterID, "user@example.com", organization.RoleDeveloper, time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)

	invite, err := f.repo.GetByID(ctx, id)
	require.NoError(t, err)

	assert.Equal(t, id, invite.ID)
	assert.Equal(t, "user@example.com", invite.Email)
}

func TestInviteRepository_GetByID_NotFound(t *testing.T) {
	f := newInviteFixtures(t)
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)

	_, err = f.repo.GetByID(ctx, id)
	assert.ErrorIs(t, err, organization.ErrInviteNotFound)
}

func TestInviteRepository_GetPendingByEmail(t *testing.T) {
	f := newInviteFixtures(t)
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = f.repo.Create(ctx, id, f.orgID, f.inviterID, "pending@example.com", organization.RoleDeveloper, time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)

	// Expired invite — must not appear in results
	expiredID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = f.repo.Create(ctx, expiredID, f.orgID, f.inviterID, "pending@example.com", organization.RoleDeveloper, time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	invites, err := f.repo.GetPendingByEmail(ctx, "pending@example.com")
	require.NoError(t, err)

	require.Len(t, invites, 1)
	assert.Equal(t, id, invites[0].ID)
}

func TestInviteRepository_Accept(t *testing.T) {
	f := newInviteFixtures(t)
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = f.repo.Create(ctx, id, f.orgID, f.inviterID, "user@example.com", organization.RoleDeveloper, time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)

	accepted, err := f.repo.Accept(ctx, id)
	require.NoError(t, err)

	assert.True(t, accepted.Accepted)
}
