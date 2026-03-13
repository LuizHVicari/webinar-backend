package user

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/LuizHVicari/webinar-backend/sqlc/generated"
)

type UserRepository struct {
	q *db.Queries
}

func NewUserRepository(q *db.Queries) *UserRepository {
	return &UserRepository{q: q}
}

func (r *UserRepository) Create(ctx context.Context, id, identityID uuid.UUID) (*User, error) {
	row, err := r.q.CreateUser(ctx, db.CreateUserParams{ID: id, IdentityID: identityID})
	if err != nil {
		return nil, err
	}
	return toUser(row), nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toUser(row), nil
}

func (r *UserRepository) GetByIdentityID(ctx context.Context, identityID uuid.UUID) (*User, error) {
	row, err := r.q.GetUserByIdentityID(ctx, identityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toUser(row), nil
}

func (r *UserRepository) UpdateOrganization(ctx context.Context, id, orgID uuid.UUID) (*User, error) {
	row, err := r.q.UpdateUserOrganization(ctx, db.UpdateUserOrganizationParams{
		ID:             id,
		OrganizationID: pgtype.UUID{Bytes: orgID, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	return toUser(row), nil
}

func toUser(row db.User) *User {
	return &User{
		ID:             row.ID,
		IdentityID:     row.IdentityID,
		OrganizationID: pgUUIDToPtr(row.OrganizationID),
		CreatedAt:      pgTimestampToTime(row.CreatedAt),
		UpdatedAt:      pgTimestampToTime(row.UpdatedAt),
	}
}

func pgUUIDToPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	id := uuid.UUID(p.Bytes)
	return &id
}

func pgTimestampToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}
