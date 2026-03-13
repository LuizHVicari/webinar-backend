package organization

import (
	"time"

	"github.com/google/uuid"
)

type Invite struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Email          string    `json:"email"`
	Role           Role      `json:"role"`
	InvitedBy      uuid.UUID `json:"invited_by"`
	Accepted       bool      `json:"accepted"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
