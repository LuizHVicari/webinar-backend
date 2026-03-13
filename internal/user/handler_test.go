package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LuizHVicari/webinar-backend/internal/organization"
	"github.com/LuizHVicari/webinar-backend/internal/user"
	"github.com/LuizHVicari/webinar-backend/pkg/middleware"
	"github.com/LuizHVicari/webinar-backend/pkg/testhelper"
	db "github.com/LuizHVicari/webinar-backend/sqlc/generated"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type userHandlerStack struct {
	router   *gin.Engine
	svc      *user.UserService
	invSvc   *organization.InviteService
	userRepo *user.UserRepository
	queries  *db.Queries
	callerID uuid.UUID
	orgID    uuid.UUID
}

// newUserHandlerStack wires up the full real stack (shared Postgres + Keto) with an admin caller already in an org.
func newUserHandlerStack(t *testing.T) userHandlerStack {
	t.Helper()
	testhelper.TruncateTables(t, sharedPool)
	testhelper.DeleteAllRelations(t, sharedKeto)
	queries := db.New(sharedPool)
	ctx := context.Background()

	orgRepo := organization.NewOrganizationRepository(queries)
	orgSvc := organization.NewOrganizationService(orgRepo)
	invRepo := organization.NewInviteRepository(queries)
	invSvc := organization.NewInviteService(invRepo, sharedKeto)
	userRepo := user.NewUserRepository(queries)
	userSvc := user.NewUserService(userRepo, orgSvc, invSvc, sharedKeto)
	handler := user.NewHandler(userSvc)

	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = orgRepo.Create(ctx, orgID, "Test Org")
	require.NoError(t, err)

	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = queries.CreateUser(ctx, db.CreateUserParams{ID: callerID, IdentityID: identityID})
	require.NoError(t, err)
	_, err = userRepo.UpdateOrganization(ctx, callerID, orgID)
	require.NoError(t, err)

	require.NoError(t, sharedKeto.AddRelation(ctx, "Organization", orgID.String(), string(organization.RoleAdmin), callerID.String()))

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, callerID.String())
		c.Set(middleware.ContextKeyOrgID, orgID.String())
		c.Set(middleware.ContextKeyIdentityEmail, "caller@example.com")
		c.Next()
	})
	handler.RegisterRoutes(r.Group("/"))

	return userHandlerStack{
		router:   r,
		svc:      userSvc,
		invSvc:   invSvc,
		userRepo: userRepo,
		queries:  queries,
		callerID: callerID,
		orgID:    orgID,
	}
}

func TestUserHandler_Me_Success(t *testing.T) {
	s := newUserHandlerStack(t)

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, s.callerID.String(), result["ID"])
}

func TestUserHandler_Me_Unauthorized(t *testing.T) {
	testhelper.TruncateTables(t, sharedPool)
	testhelper.DeleteAllRelations(t, sharedKeto)
	queries := db.New(sharedPool)

	orgSvc := organization.NewOrganizationService(organization.NewOrganizationRepository(queries))
	invSvc := organization.NewInviteService(organization.NewInviteRepository(queries), sharedKeto)
	userSvc := user.NewUserService(user.NewUserRepository(queries), orgSvc, invSvc, sharedKeto)
	handler := user.NewHandler(userSvc)

	r := gin.New()
	// no user_id set → 401
	handler.RegisterRoutes(r.Group("/"))

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUserHandler_CreateOrg_Success(t *testing.T) {
	testhelper.TruncateTables(t, sharedPool)
	testhelper.DeleteAllRelations(t, sharedKeto)
	queries := db.New(sharedPool)
	ctx := context.Background()

	orgSvc := organization.NewOrganizationService(organization.NewOrganizationRepository(queries))
	invSvc := organization.NewInviteService(organization.NewInviteRepository(queries), sharedKeto)
	userRepo := user.NewUserRepository(queries)
	userSvc := user.NewUserService(userRepo, orgSvc, invSvc, sharedKeto)
	handler := user.NewHandler(userSvc)

	userID, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = queries.CreateUser(ctx, db.CreateUserParams{ID: userID, IdentityID: identityID})
	require.NoError(t, err)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, userID.String())
		c.Next()
	})
	handler.RegisterRoutes(r.Group("/"))

	body, _ := json.Marshal(map[string]string{"org_name": "My New Org"})
	req := httptest.NewRequest(http.MethodPost, "/users/create-org", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.NotNil(t, result["OrganizationID"])
}

func TestUserHandler_CreateOrg_Unauthorized(t *testing.T) {
	testhelper.TruncateTables(t, sharedPool)
	testhelper.DeleteAllRelations(t, sharedKeto)
	queries := db.New(sharedPool)

	orgSvc := organization.NewOrganizationService(organization.NewOrganizationRepository(queries))
	invSvc := organization.NewInviteService(organization.NewInviteRepository(queries), sharedKeto)
	userSvc := user.NewUserService(user.NewUserRepository(queries), orgSvc, invSvc, sharedKeto)
	handler := user.NewHandler(userSvc)

	r := gin.New()
	// no user_id set → 401
	handler.RegisterRoutes(r.Group("/"))

	body, _ := json.Marshal(map[string]string{"org_name": "My Org"})
	req := httptest.NewRequest(http.MethodPost, "/users/create-org", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUserHandler_JoinViaInvite_Success(t *testing.T) {
	s := newUserHandlerStack(t)
	ctx := context.Background()

	// Create a separate user who will join via invite
	inviteeID, err := uuid.NewV7()
	require.NoError(t, err)
	inviteeIdentityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = s.queries.CreateUser(ctx, db.CreateUserParams{ID: inviteeID, IdentityID: inviteeIdentityID})
	require.NoError(t, err)

	// Seed the invite via the service (caller is admin in Keto)
	inv, err := s.invSvc.Create(ctx, s.callerID, s.orgID, "invitee@example.com", organization.RoleDeveloper)
	require.NoError(t, err)

	inviteeRouter := gin.New()
	inviteeRouter.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, inviteeID.String())
		c.Set(middleware.ContextKeyIdentityEmail, "invitee@example.com")
		c.Next()
	})
	user.NewHandler(s.svc).RegisterRoutes(inviteeRouter.Group("/"))

	body, _ := json.Marshal(map[string]string{"invite_id": inv.ID.String()})
	req := httptest.NewRequest(http.MethodPost, "/users/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	inviteeRouter.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, s.orgID.String(), result["OrganizationID"])
}

func TestUserHandler_ChangeRole_Success(t *testing.T) {
	s := newUserHandlerStack(t)
	ctx := context.Background()

	// Create a target user with developer role in the same org
	targetID, err := uuid.NewV7()
	require.NoError(t, err)
	targetIdentityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = s.queries.CreateUser(ctx, db.CreateUserParams{ID: targetID, IdentityID: targetIdentityID})
	require.NoError(t, err)
	_, err = s.userRepo.UpdateOrganization(ctx, targetID, s.orgID)
	require.NoError(t, err)
	require.NoError(t, sharedKeto.AddRelation(ctx, "Organization", s.orgID.String(), string(organization.RoleDeveloper), targetID.String()))

	body, _ := json.Marshal(map[string]string{"role": "manager"})
	req := httptest.NewRequest(http.MethodPut, "/users/"+targetID.String()+"/role", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
