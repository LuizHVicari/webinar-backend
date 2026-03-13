package invite

import (
	"time"

	"github.com/google/uuid"
	"github.com/LuizHVicari/webinar-backend/internal/organization"
)

type Invite struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Email          string
	Role           organization.Role
	InvitedBy      uuid.UUID
	Accepted       bool
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
