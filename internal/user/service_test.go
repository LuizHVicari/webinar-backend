package user_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LuizHVicari/webinar-backend/internal/organization"
	"github.com/LuizHVicari/webinar-backend/internal/user"
)

// fakeUserRepo is an in-memory implementation of the userRepo interface.
type fakeUserRepo struct {
	users map[uuid.UUID]*user.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[uuid.UUID]*user.User)}
}

func (f *fakeUserRepo) Create(_ context.Context, id, identityID uuid.UUID) (*user.User, error) {
	u := &user.User{ID: id, IdentityID: identityID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.users[id] = u
	return u, nil
}

func (f *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) GetByIdentityID(_ context.Context, identityID uuid.UUID) (*user.User, error) {
	for _, u := range f.users {
		if u.IdentityID == identityID {
			return u, nil
		}
	}
	return nil, user.ErrNotFound
}

func (f *fakeUserRepo) UpdateOrganization(_ context.Context, id, orgID uuid.UUID) (*user.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	u.OrganizationID = &orgID
	return u, nil
}

// fakeOrgCreator is an in-memory implementation of the orgCreator interface.
type fakeOrgCreator struct {
	orgs map[uuid.UUID]*organization.Organization
}

func newFakeOrgCreator() *fakeOrgCreator {
	return &fakeOrgCreator{orgs: make(map[uuid.UUID]*organization.Organization)}
}

func (f *fakeOrgCreator) Create(_ context.Context, id uuid.UUID, name string) (*organization.Organization, error) {
	org := &organization.Organization{ID: id, Name: name}
	f.orgs[id] = org
	return org, nil
}

// fakeInviteAcceptor is an in-memory implementation of the inviteAcceptor interface.
type fakeInviteAcceptor struct {
	orgID uuid.UUID
	role  organization.Role
	err   error
}

func (f *fakeInviteAcceptor) Accept(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, organization.Role, error) {
	return f.orgID, f.role, f.err
}

// fakeKetoClient is an in-memory implementation of the ketoClient interface.
type fakeKetoClient struct {
	relations map[string]bool
}

func newFakeKetoClient() *fakeKetoClient {
	return &fakeKetoClient{relations: make(map[string]bool)}
}

func (f *fakeKetoClient) key(object, relation, subjectID string) string {
	return object + "#" + relation + "@" + subjectID
}

func (f *fakeKetoClient) HasRelation(_ context.Context, _, object, relation, subjectID string) (bool, error) {
	return f.relations[f.key(object, relation, subjectID)], nil
}

func (f *fakeKetoClient) AddRelation(_ context.Context, _, object, relation, subjectID string) error {
	f.relations[f.key(object, relation, subjectID)] = true
	return nil
}

func (f *fakeKetoClient) DeleteRelation(_ context.Context, _, object, relation, subjectID string) error {
	delete(f.relations, f.key(object, relation, subjectID))
	return nil
}

func (f *fakeKetoClient) grantRole(orgID uuid.UUID, role organization.Role, userID uuid.UUID) {
	f.relations[f.key(orgID.String(), string(role), userID.String())] = true
}

func TestUserService_GetOrCreate_CreatesNewUser(t *testing.T) {
	repo := newFakeUserRepo()
	svc := user.NewUserService(repo, newFakeOrgCreator(), &fakeInviteAcceptor{}, newFakeKetoClient())
	ctx := context.Background()

	identityID, err := uuid.NewV7()
	require.NoError(t, err)

	userID, orgID, err := svc.GetOrCreate(ctx, identityID)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.UUID{}, userID)
	assert.Nil(t, orgID)
}

func TestUserService_GetOrCreate_ReturnsExistingUser(t *testing.T) {
	repo := newFakeUserRepo()
	svc := user.NewUserService(repo, newFakeOrgCreator(), &fakeInviteAcceptor{}, newFakeKetoClient())
	ctx := context.Background()

	identityID, err := uuid.NewV7()
	require.NoError(t, err)

	// First call creates
	userID1, _, err := svc.GetOrCreate(ctx, identityID)
	require.NoError(t, err)

	// Second call retrieves
	userID2, _, err := svc.GetOrCreate(ctx, identityID)
	require.NoError(t, err)

	assert.Equal(t, userID1, userID2)
}

func TestUserService_JoinViaInvite_Success(t *testing.T) {
	repo := newFakeUserRepo()
	keto := newFakeKetoClient()
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	invites := &fakeInviteAcceptor{orgID: orgID, role: organization.RoleDeveloper}
	svc := user.NewUserService(repo, newFakeOrgCreator(), invites, keto)
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = repo.Create(ctx, id, identityID)
	require.NoError(t, err)

	inviteID, err := uuid.NewV7()
	require.NoError(t, err)

	updated, err := svc.JoinViaInvite(ctx, id, "user@example.com", inviteID)
	require.NoError(t, err)

	require.NotNil(t, updated.OrganizationID)
	assert.Equal(t, orgID, *updated.OrganizationID)
	assert.True(t, keto.relations[keto.key(orgID.String(), string(organization.RoleDeveloper), id.String())])
}

func TestUserService_JoinViaInvite_AlreadyInOrg(t *testing.T) {
	repo := newFakeUserRepo()
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	svc := user.NewUserService(repo, newFakeOrgCreator(), &fakeInviteAcceptor{orgID: orgID}, newFakeKetoClient())
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	u, err := repo.Create(ctx, id, identityID)
	require.NoError(t, err)
	u.OrganizationID = &orgID // already in org

	inviteID, err := uuid.NewV7()
	require.NoError(t, err)

	_, err = svc.JoinViaInvite(ctx, id, "user@example.com", inviteID)
	assert.ErrorIs(t, err, user.ErrAlreadyInOrg)
}

func TestUserService_CreateWithOrg_Success(t *testing.T) {
	repo := newFakeUserRepo()
	keto := newFakeKetoClient()
	svc := user.NewUserService(repo, newFakeOrgCreator(), &fakeInviteAcceptor{}, keto)
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = repo.Create(ctx, id, identityID)
	require.NoError(t, err)

	updated, err := svc.CreateWithOrg(ctx, id, "My Startup")
	require.NoError(t, err)

	require.NotNil(t, updated.OrganizationID)
	assert.True(t, keto.relations[keto.key(updated.OrganizationID.String(), string(organization.RoleAdmin), id.String())])
}

func TestUserService_CreateWithOrg_AlreadyInOrg(t *testing.T) {
	repo := newFakeUserRepo()
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	svc := user.NewUserService(repo, newFakeOrgCreator(), &fakeInviteAcceptor{}, newFakeKetoClient())
	ctx := context.Background()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	u, err := repo.Create(ctx, id, identityID)
	require.NoError(t, err)
	u.OrganizationID = &orgID

	_, err = svc.CreateWithOrg(ctx, id, "Another Org")
	assert.ErrorIs(t, err, user.ErrAlreadyInOrg)
}

func TestUserService_ChangeRole_AdminCanChangeAnyRole(t *testing.T) {
	repo := newFakeUserRepo()
	keto := newFakeKetoClient()
	svc := user.NewUserService(repo, newFakeOrgCreator(), &fakeInviteAcceptor{}, keto)
	ctx := context.Background()

	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	targetID, err := uuid.NewV7()
	require.NoError(t, err)

	keto.grantRole(orgID, organization.RoleAdmin, callerID)
	keto.grantRole(orgID, organization.RoleDeveloper, targetID)

	err = svc.ChangeRole(ctx, callerID, orgID, targetID, organization.RoleManager)
	require.NoError(t, err)

	assert.False(t, keto.relations[keto.key(orgID.String(), string(organization.RoleDeveloper), targetID.String())])
	assert.True(t, keto.relations[keto.key(orgID.String(), string(organization.RoleManager), targetID.String())])
}

func TestUserService_ChangeRole_HRCannotChangeSelf(t *testing.T) {
	repo := newFakeUserRepo()
	keto := newFakeKetoClient()
	svc := user.NewUserService(repo, newFakeOrgCreator(), &fakeInviteAcceptor{}, keto)
	ctx := context.Background()

	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	callerID, err := uuid.NewV7()
	require.NoError(t, err)

	keto.grantRole(orgID, organization.RoleHumanResource, callerID)

	err = svc.ChangeRole(ctx, callerID, orgID, callerID, organization.RoleDeveloper)
	assert.ErrorIs(t, err, organization.ErrUnauthorized)
}

func TestUserService_ChangeRole_HRCannotChangeAdmin(t *testing.T) {
	repo := newFakeUserRepo()
	keto := newFakeKetoClient()
	svc := user.NewUserService(repo, newFakeOrgCreator(), &fakeInviteAcceptor{}, keto)
	ctx := context.Background()

	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	targetID, err := uuid.NewV7()
	require.NoError(t, err)

	keto.grantRole(orgID, organization.RoleHumanResource, callerID)
	keto.grantRole(orgID, organization.RoleAdmin, targetID)

	err = svc.ChangeRole(ctx, callerID, orgID, targetID, organization.RoleDeveloper)
	assert.ErrorIs(t, err, organization.ErrUnauthorized)
}

func TestUserService_ChangeRole_InvalidRole(t *testing.T) {
	svc := user.NewUserService(newFakeUserRepo(), newFakeOrgCreator(), &fakeInviteAcceptor{}, newFakeKetoClient())
	ctx := context.Background()

	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	targetID, err := uuid.NewV7()
	require.NoError(t, err)

	err = svc.ChangeRole(ctx, callerID, orgID, targetID, "not-a-role")
	assert.ErrorIs(t, err, organization.ErrInvalidRole)
}
