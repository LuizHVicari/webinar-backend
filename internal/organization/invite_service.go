package organization

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ketoChecker interface {
	HasRelation(ctx context.Context, namespace, object, relation, subjectID string) (bool, error)
}

type inviteRepo interface {
	Create(ctx context.Context, id, orgID, invitedBy uuid.UUID, email string, role Role, expiresAt time.Time) (*Invite, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Invite, error)
	GetPendingByEmail(ctx context.Context, email string) ([]*Invite, error)
	Accept(ctx context.Context, id uuid.UUID) (*Invite, error)
}

type InviteService struct {
	repo inviteRepo
	keto ketoChecker
}

func NewInviteService(repo inviteRepo, keto ketoChecker) *InviteService {
	return &InviteService{repo: repo, keto: keto}
}

func (s *InviteService) Create(ctx context.Context, callerID, orgID uuid.UUID, email string, role Role) (*Invite, error) {
	if !role.IsValid() {
		return nil, ErrInvalidRole
	}

	callerRole, err := s.resolveRole(ctx, orgID, callerID)
	if err != nil {
		return nil, err
	}
	if !callerRole.IsAdminOrHR() {
		return nil, ErrUnauthorized
	}
	if callerRole == RoleHumanResource && role == RoleAdmin {
		return nil, ErrUnauthorized
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, id, orgID, callerID, email, role, time.Now().Add(7*24*time.Hour))
}

func (s *InviteService) GetPendingByEmail(ctx context.Context, email string) ([]*Invite, error) {
	return s.repo.GetPendingByEmail(ctx, email)
}

func (s *InviteService) Accept(ctx context.Context, inviteID uuid.UUID, callerEmail string) (uuid.UUID, Role, error) {
	invite, err := s.repo.GetByID(ctx, inviteID)
	if err != nil {
		return uuid.UUID{}, "", err
	}
	if invite.Accepted {
		return uuid.UUID{}, "", ErrInviteAlreadyAccepted
	}
	if time.Now().After(invite.ExpiresAt) {
		return uuid.UUID{}, "", ErrInviteExpired
	}
	if invite.Email != callerEmail {
		return uuid.UUID{}, "", ErrInviteNotForUser
	}

	if _, err := s.repo.Accept(ctx, inviteID); err != nil {
		return uuid.UUID{}, "", err
	}
	return invite.OrganizationID, invite.Role, nil
}

func (s *InviteService) resolveRole(ctx context.Context, orgID, userID uuid.UUID) (Role, error) {
	for _, r := range Roles() {
		ok, err := s.keto.HasRelation(ctx, "Organization", orgID.String(), string(r), userID.String())
		if err != nil {
			return "", err
		}
		if ok {
			return r, nil
		}
	}
	return "", ErrUnauthorized
}
