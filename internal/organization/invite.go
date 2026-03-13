package organization

import (
	"time"

	"github.com/google/uuid"
)

type Invite struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Email          string
	Role           Role
	InvitedBy      uuid.UUID
	Accepted       bool
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
