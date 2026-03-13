package user

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID             uuid.UUID  `json:"id"`
	IdentityID     uuid.UUID  `json:"identity_id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
