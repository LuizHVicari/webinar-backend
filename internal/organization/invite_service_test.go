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

// fakeInviteRepo is an in-memory implementation of the inviteRepo interface.
type fakeInviteRepo struct {
	invites map[uuid.UUID]*organization.Invite
}

func newFakeInviteRepo() *fakeInviteRepo {
	return &fakeInviteRepo{invites: make(map[uuid.UUID]*organization.Invite)}
}

func (f *fakeInviteRepo) Create(_ context.Context, id, orgID, invitedBy uuid.UUID, email string, role organization.Role, expiresAt time.Time) (*organization.Invite, error) {
	inv := &organization.Invite{
		ID:             id,
		OrganizationID: orgID,
		InvitedBy:      invitedBy,
		Email:          email,
		Role:           role,
		ExpiresAt:      expiresAt,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	f.invites[id] = inv
	return inv, nil
}

func (f *fakeInviteRepo) GetByID(_ context.Context, id uuid.UUID) (*organization.Invite, error) {
	inv, ok := f.invites[id]
	if !ok {
		return nil, organization.ErrInviteNotFound
	}
	return inv, nil
}

func (f *fakeInviteRepo) GetPendingByEmail(_ context.Context, email string) ([]*organization.Invite, error) {
	var result []*organization.Invite
	for _, inv := range f.invites {
		if inv.Email == email && !inv.Accepted && time.Now().Before(inv.ExpiresAt) {
			result = append(result, inv)
		}
	}
	return result, nil
}

func (f *fakeInviteRepo) Accept(_ context.Context, id uuid.UUID) (*organization.Invite, error) {
	inv, ok := f.invites[id]
	if !ok {
		return nil, organization.ErrInviteNotFound
	}
	inv.Accepted = true
	return inv, nil
}

// fakeKetoChecker is an in-memory implementation of the ketoChecker interface.
type fakeKetoChecker struct {
	relations map[string]bool
}

func newFakeKetoChecker() *fakeKetoChecker {
	return &fakeKetoChecker{relations: make(map[string]bool)}
}

func (f *fakeKetoChecker) HasRelation(_ context.Context, _, object, relation, subjectID string) (bool, error) {
	key := object + "#" + relation + "@" + subjectID
	return f.relations[key], nil
}

func (f *fakeKetoChecker) grantRole(orgID uuid.UUID, role organization.Role, userID uuid.UUID) {
	key := orgID.String() + "#" + string(role) + "@" + userID.String()
	f.relations[key] = true
}

func newInviteService(repo *fakeInviteRepo, keto *fakeKetoChecker) *organization.InviteService {
	return organization.NewInviteService(repo, keto)
}

func TestInviteService_Create_AdminCanInviteAnyRole(t *testing.T) {
	repo := newFakeInviteRepo()
	keto := newFakeKetoChecker()

	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	keto.grantRole(orgID, organization.RoleAdmin, callerID)

	svc := newInviteService(repo, keto)
	ctx := context.Background()

	for _, role := range organization.Roles() {
		inv, err := svc.Create(ctx, callerID, orgID, "user@example.com", role)
		require.NoError(t, err)
		assert.Equal(t, role, inv.Role)
	}
}

func TestInviteService_Create_HRCannotInviteAdmin(t *testing.T) {
	repo := newFakeInviteRepo()
	keto := newFakeKetoChecker()

	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	keto.grantRole(orgID, organization.RoleHumanResource, callerID)

	svc := newInviteService(repo, keto)
	ctx := context.Background()

	_, err = svc.Create(ctx, callerID, orgID, "user@example.com", organization.RoleAdmin)
	assert.ErrorIs(t, err, organization.ErrUnauthorized)
}

func TestInviteService_Create_DeveloperCannotInvite(t *testing.T) {
	repo := newFakeInviteRepo()
	keto := newFakeKetoChecker()

	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	keto.grantRole(orgID, organization.RoleDeveloper, callerID)

	svc := newInviteService(repo, keto)
	ctx := context.Background()

	_, err = svc.Create(ctx, callerID, orgID, "user@example.com", organization.RoleDeveloper)
	assert.ErrorIs(t, err, organization.ErrUnauthorized)
}

func TestInviteService_Create_InvalidRole(t *testing.T) {
	svc := newInviteService(newFakeInviteRepo(), newFakeKetoChecker())
	ctx := context.Background()

	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	orgID, err := uuid.NewV7()
	require.NoError(t, err)

	_, err = svc.Create(ctx, callerID, orgID, "user@example.com", "not-a-role")
	assert.ErrorIs(t, err, organization.ErrInvalidRole)
}

func TestInviteService_Accept_Success(t *testing.T) {
	repo := newFakeInviteRepo()
	keto := newFakeKetoChecker()
	svc := newInviteService(repo, keto)
	ctx := context.Background()

	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	keto.grantRole(orgID, organization.RoleAdmin, callerID)

	inv, err := svc.Create(ctx, callerID, orgID, "invitee@example.com", organization.RoleDeveloper)
	require.NoError(t, err)

	gotOrgID, gotRole, err := svc.Accept(ctx, inv.ID, "invitee@example.com")
	require.NoError(t, err)
	assert.Equal(t, orgID, gotOrgID)
	assert.Equal(t, organization.RoleDeveloper, gotRole)
}

func TestInviteService_Accept_WrongEmail(t *testing.T) {
	repo := newFakeInviteRepo()
	keto := newFakeKetoChecker()
	svc := newInviteService(repo, keto)
	ctx := context.Background()

	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	keto.grantRole(orgID, organization.RoleAdmin, callerID)

	inv, err := svc.Create(ctx, callerID, orgID, "invitee@example.com", organization.RoleDeveloper)
	require.NoError(t, err)

	_, _, err = svc.Accept(ctx, inv.ID, "other@example.com")
	assert.ErrorIs(t, err, organization.ErrInviteNotForUser)
}

func TestInviteService_Accept_AlreadyAccepted(t *testing.T) {
	repo := newFakeInviteRepo()
	keto := newFakeKetoChecker()
	svc := newInviteService(repo, keto)
	ctx := context.Background()

	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	keto.grantRole(orgID, organization.RoleAdmin, callerID)

	inv, err := svc.Create(ctx, callerID, orgID, "invitee@example.com", organization.RoleDeveloper)
	require.NoError(t, err)

	_, _, err = svc.Accept(ctx, inv.ID, "invitee@example.com")
	require.NoError(t, err)

	_, _, err = svc.Accept(ctx, inv.ID, "invitee@example.com")
	assert.ErrorIs(t, err, organization.ErrInviteAlreadyAccepted)
}

func TestInviteService_Accept_Expired(t *testing.T) {
	repo := newFakeInviteRepo()
	svc := newInviteService(repo, newFakeKetoChecker())
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	inviterID, err := uuid.NewV7()
	require.NoError(t, err)

	// Inject expired invite directly into fake repo
	_, err = repo.Create(ctx, id, orgID, inviterID, "invitee@example.com", organization.RoleDeveloper, time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	_, _, err = svc.Accept(ctx, id, "invitee@example.com")
	assert.ErrorIs(t, err, organization.ErrInviteExpired)
}
