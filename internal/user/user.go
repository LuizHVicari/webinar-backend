package user

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID             uuid.UUID
	IdentityID     uuid.UUID
	OrganizationID *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
