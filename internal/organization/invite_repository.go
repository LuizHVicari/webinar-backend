package organization

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/LuizHVicari/webinar-backend/sqlc/generated"
)

type InviteRepository struct {
	q *db.Queries
}

func NewInviteRepository(q *db.Queries) *InviteRepository {
	return &InviteRepository{q: q}
}

func (r *InviteRepository) Create(ctx context.Context, id, orgID, invitedBy uuid.UUID, email string, role Role, expiresAt time.Time) (*Invite, error) {
	row, err := r.q.CreateInvite(ctx, db.CreateInviteParams{
		ID:             id,
		OrganizationID: orgID,
		Email:          email,
		Role:           string(role),
		InvitedBy:      invitedBy,
		ExpiresAt:      pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	return toInvite(row), nil
}

func (r *InviteRepository) GetByID(ctx context.Context, id uuid.UUID) (*Invite, error) {
	row, err := r.q.GetInviteByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInviteNotFound
	}
	if err != nil {
		return nil, err
	}
	return toInvite(row), nil
}

func (r *InviteRepository) GetPendingByEmail(ctx context.Context, email string) ([]*Invite, error) {
	rows, err := r.q.GetPendingInvitesByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	invites := make([]*Invite, len(rows))
	for i, row := range rows {
		invites[i] = toInvite(row)
	}
	return invites, nil
}

func (r *InviteRepository) Accept(ctx context.Context, id uuid.UUID) (*Invite, error) {
	row, err := r.q.AcceptInvite(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInviteNotFound
	}
	if err != nil {
		return nil, err
	}
	return toInvite(row), nil
}

func toInvite(row db.Invite) *Invite {
	return &Invite{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Email:          row.Email,
		Role:           Role(row.Role),
		InvitedBy:      row.InvitedBy,
		Accepted:       row.Accepted,
		ExpiresAt:      pgTimestampToTime(row.ExpiresAt),
		CreatedAt:      pgTimestampToTime(row.CreatedAt),
		UpdatedAt:      pgTimestampToTime(row.UpdatedAt),
	}
}
