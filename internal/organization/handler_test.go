package organization_test

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
	"github.com/LuizHVicari/webinar-backend/pkg/middleware"
	"github.com/LuizHVicari/webinar-backend/pkg/testhelper"
	db "github.com/LuizHVicari/webinar-backend/sqlc/generated"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type handlerStack struct {
	router   *gin.Engine
	invSvc   *organization.InviteService
	invRepo  *organization.InviteRepository
	callerID uuid.UUID
	orgID    uuid.UUID
}

// newHandlerStack wires up the full real stack (shared Postgres + Keto) and seeds an admin caller.
func newHandlerStack(t *testing.T) handlerStack {
	t.Helper()
	testhelper.TruncateTables(t, sharedPool)
	testhelper.DeleteAllRelations(t, sharedKeto)
	queries := db.New(sharedPool)
	ctx := context.Background()

	invRepo := organization.NewInviteRepository(queries)
	invSvc := organization.NewInviteService(invRepo, sharedKeto)
	handler := organization.NewHandler(invSvc)

	orgID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = organization.NewOrganizationRepository(queries).Create(ctx, orgID, "Test Org")
	require.NoError(t, err)

	callerID, err := uuid.NewV7()
	require.NoError(t, err)
	identityID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = queries.CreateUser(ctx, db.CreateUserParams{ID: callerID, IdentityID: identityID})
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

	return handlerStack{router: r, invSvc: invSvc, invRepo: invRepo, callerID: callerID, orgID: orgID}
}

func TestOrgHandler_CreateInvite_Success(t *testing.T) {
	s := newHandlerStack(t)

	body, _ := json.Marshal(map[string]string{"email": "invitee@example.com", "role": "developer"})
	req := httptest.NewRequest(http.MethodPost, "/invites", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "invitee@example.com", result["Email"])
	assert.Equal(t, "developer", result["Role"])
}

func TestOrgHandler_CreateInvite_InvalidRole(t *testing.T) {
	s := newHandlerStack(t)

	body, _ := json.Marshal(map[string]string{"email": "invitee@example.com", "role": "superuser"})
	req := httptest.NewRequest(http.MethodPost, "/invites", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOrgHandler_CreateInvite_Forbidden_NoOrgID(t *testing.T) {
	testhelper.TruncateTables(t, sharedPool)
	queries := db.New(sharedPool)

	invSvc := organization.NewInviteService(organization.NewInviteRepository(queries), newFakeKetoChecker())
	handler := organization.NewHandler(invSvc)

	callerID, err := uuid.NewV7()
	require.NoError(t, err)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, callerID.String())
		// org_id intentionally not set
		c.Next()
	})
	handler.RegisterRoutes(r.Group("/"))

	body, _ := json.Marshal(map[string]string{"email": "invitee@example.com", "role": "developer"})
	req := httptest.NewRequest(http.MethodPost, "/invites", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestOrgHandler_ListPendingInvites_Success(t *testing.T) {
	s := newHandlerStack(t)
	ctx := context.Background()

	// Seed an invite via the service (goes through real business logic + DB + Keto)
	_, err := s.invSvc.Create(ctx, s.callerID, s.orgID, "invitee@example.com", organization.RoleDeveloper)
	require.NoError(t, err)

	// Build a router scoped to the invitee's perspective (same service, different identity email)
	inviteeRouter := gin.New()
	inviteeRouter.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyIdentityEmail, "invitee@example.com")
		c.Next()
	})
	organization.NewHandler(s.invSvc).RegisterRoutes(inviteeRouter.Group("/"))

	req := httptest.NewRequest(http.MethodGet, "/invites/pending", nil)
	w := httptest.NewRecorder()
	inviteeRouter.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 1)
	assert.Equal(t, "invitee@example.com", result[0]["Email"])
}

func TestOrgHandler_ListPendingInvites_Unauthorized(t *testing.T) {
	testhelper.TruncateTables(t, sharedPool)
	queries := db.New(sharedPool)

	invSvc := organization.NewInviteService(organization.NewInviteRepository(queries), newFakeKetoChecker())
	handler := organization.NewHandler(invSvc)

	r := gin.New()
	// identity_email not set → 401
	handler.RegisterRoutes(r.Group("/"))

	req := httptest.NewRequest(http.MethodGet, "/invites/pending", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
