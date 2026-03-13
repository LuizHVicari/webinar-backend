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

type OrganizationRepository struct {
	q *db.Queries
}

func NewOrganizationRepository(q *db.Queries) *OrganizationRepository {
	return &OrganizationRepository{q: q}
}

func (r *OrganizationRepository) Create(ctx context.Context, id uuid.UUID, name string) (*Organization, error) {
	row, err := r.q.CreateOrganization(ctx, db.CreateOrganizationParams{ID: id, Name: name})
	if err != nil {
		return nil, err
	}
	return toOrganization(row), nil
}

func (r *OrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	row, err := r.q.GetOrganizationByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toOrganization(row), nil
}

func toOrganization(row db.Organization) *Organization {
	return &Organization{
		ID:        row.ID,
		Name:      row.Name,
		CreatedAt: pgTimestampToTime(row.CreatedAt),
		UpdatedAt: pgTimestampToTime(row.UpdatedAt),
	}
}

func pgTimestampToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}
