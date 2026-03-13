package user

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/LuizHVicari/webinar-backend/internal/organization"
	"github.com/LuizHVicari/webinar-backend/pkg/middleware"
)

type userService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	JoinViaInvite(ctx context.Context, userID uuid.UUID, email string, inviteID uuid.UUID) (*User, error)
	CreateWithOrg(ctx context.Context, userID uuid.UUID, orgName string) (*User, error)
	ChangeRole(ctx context.Context, callerID, callerOrgID, targetID uuid.UUID, newRole organization.Role) error
}

type Handler struct {
	svc userService
}

func NewHandler(svc userService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/users/me", h.me)
	rg.POST("/users/join", h.joinViaInvite)
	rg.POST("/users/create-org", h.createOrg)
	rg.PUT("/users/:id/role", h.changeRole)
}

// @Summary      Get current user
// @Tags         users
// @Produce      json
// @Success      200  {object}  User
// @Failure      401  {object}  common.ErrorResponse
// @Failure      404  {object}  common.ErrorResponse
// @Failure      500  {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /users/me [get]
func (h *Handler) me(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.ContextKeyUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	u, err := h.svc.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(HTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}

// @Summary      Join an organization via invite
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      JoinViaInviteRequest  true  "Invite ID"
// @Success      200   {object}  User
// @Failure      400   {object}  common.ErrorResponse
// @Failure      401   {object}  common.ErrorResponse
// @Failure      409   {object}  common.ErrorResponse
// @Failure      422   {object}  common.ErrorResponse
// @Failure      500   {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /users/join [post]
func (h *Handler) joinViaInvite(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.ContextKeyUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	email := c.GetString(middleware.ContextKeyIdentityEmail)

	var req JoinViaInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	inviteID, _ := uuid.Parse(req.InviteID)

	u, err := h.svc.JoinViaInvite(c.Request.Context(), userID, email, inviteID)
	if err != nil {
		c.JSON(HTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}

// @Summary      Create an organization
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      CreateOrgRequest  true  "Organization name"
// @Success      201   {object}  User
// @Failure      400   {object}  common.ErrorResponse
// @Failure      401   {object}  common.ErrorResponse
// @Failure      409   {object}  common.ErrorResponse
// @Failure      500   {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /users/create-org [post]
func (h *Handler) createOrg(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.ContextKeyUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.svc.CreateWithOrg(c.Request.Context(), userID, req.OrgName)
	if err != nil {
		c.JSON(HTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}

// @Summary      Change a user's role
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      string             true  "Target user UUID"
// @Param        body  body      ChangeRoleRequest  true  "New role"
// @Success      204
// @Failure      400   {object}  common.ErrorResponse
// @Failure      401   {object}  common.ErrorResponse
// @Failure      403   {object}  common.ErrorResponse
// @Failure      422   {object}  common.ErrorResponse
// @Failure      500   {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /users/{id}/role [put]
func (h *Handler) changeRole(c *gin.Context) {
	callerID, err := uuid.Parse(c.GetString(middleware.ContextKeyUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	orgIDStr := c.GetString(middleware.ContextKeyOrgID)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "must belong to an organization"})
		return
	}
	orgID, _ := uuid.Parse(orgIDStr)

	targetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req ChangeRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.ChangeRole(c.Request.Context(), callerID, orgID, targetID, organization.Role(req.Role)); err != nil {
		c.JSON(HTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
