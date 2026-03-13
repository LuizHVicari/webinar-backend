package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ory "github.com/ory/client-go"
)

const (
	ContextKeyIdentityID    = "identity_id"
	ContextKeyIdentityEmail = "identity_email"
	ContextKeyUserID        = "user_id"
	ContextKeyOrgID         = "org_id"
)

// UserResolver fetches or creates a local user record from the Kratos identity ID.
// Implemented by the user service and injected at wire-up time.
type UserResolver interface {
	GetOrCreate(ctx context.Context, identityID uuid.UUID) (userID uuid.UUID, orgID *uuid.UUID, err error)
}

func Auth(kratosPublicURL string, users UserResolver) gin.HandlerFunc {
	cfg := ory.NewConfiguration()
	cfg.Servers = ory.ServerConfigurations{{URL: kratosPublicURL}}
	client := ory.NewAPIClient(cfg)

	return func(c *gin.Context) {
		cookie, err := c.Cookie("ory_kratos_session")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		session, _, err := client.FrontendAPI.ToSession(c.Request.Context()).
			Cookie("ory_kratos_session=" + cookie).
			Execute()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		identity := session.GetIdentity()
		email, _ := identity.GetTraits().(map[string]any)["email"].(string)
		identityID, err := uuid.Parse(identity.GetId())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		userID, orgID, err := users.GetOrCreate(c.Request.Context(), identityID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.Set(ContextKeyIdentityID, identity.GetId())
		c.Set(ContextKeyIdentityEmail, email)
		c.Set(ContextKeyUserID, userID.String())
		if orgID != nil {
			c.Set(ContextKeyOrgID, orgID.String())
		}
		c.Next()
	}
}
