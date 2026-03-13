package organization

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/LuizHVicari/webinar-backend/pkg/middleware"
)

type inviteService interface {
	Create(ctx context.Context, callerID, orgID uuid.UUID, email string, role Role) (*Invite, error)
	GetPendingByEmail(ctx context.Context, email string) ([]*Invite, error)
}

type Handler struct {
	inviteSvc inviteService
}

func NewHandler(inviteSvc inviteService) *Handler {
	return &Handler{inviteSvc: inviteSvc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/invites", h.createInvite)
	rg.GET("/invites/pending", h.listPendingInvites)
}

// @Summary      Create an invite
// @Tags         invites
// @Accept       json
// @Produce      json
// @Param        body  body      CreateInviteRequest  true  "Invite details"
// @Success      201   {object}  Invite
// @Failure      400   {object}  common.ErrorResponse
// @Failure      401   {object}  common.ErrorResponse
// @Failure      403   {object}  common.ErrorResponse
// @Failure      500   {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /invites [post]
func (h *Handler) createInvite(c *gin.Context) {
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
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var req CreateInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	invite, err := h.inviteSvc.Create(c.Request.Context(), callerID, orgID, req.Email, Role(req.Role))
	if err != nil {
		c.JSON(errStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, invite)
}

// @Summary      List pending invites
// @Tags         invites
// @Produce      json
// @Success      200  {array}   Invite
// @Failure      401  {object}  common.ErrorResponse
// @Failure      500  {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /invites/pending [get]
func (h *Handler) listPendingInvites(c *gin.Context) {
	email := c.GetString(middleware.ContextKeyIdentityEmail)
	if email == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	invites, err := h.inviteSvc.GetPendingByEmail(c.Request.Context(), email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, invites)
}

func errStatus(err error) int {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidRole):
		return http.StatusBadRequest
	case errors.Is(err, ErrInviteNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInviteExpired), errors.Is(err, ErrInviteAlreadyAccepted), errors.Is(err, ErrInviteNotForUser):
		return http.StatusUnprocessableEntity
	}
	return http.StatusInternalServerError
}
