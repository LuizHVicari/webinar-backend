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
