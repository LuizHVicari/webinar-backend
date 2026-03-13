package user

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/LuizHVicari/webinar-backend/internal/organization"
)

type userRepo interface {
	Create(ctx context.Context, id, identityID uuid.UUID) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByIdentityID(ctx context.Context, identityID uuid.UUID) (*User, error)
	UpdateOrganization(ctx context.Context, id, orgID uuid.UUID) (*User, error)
}

// orgCreator is satisfied by *organization.OrganizationService.
type orgCreator interface {
	Create(ctx context.Context, id uuid.UUID, name string) (*organization.Organization, error)
}

// inviteAcceptor is satisfied by *organization.InviteService.
type inviteAcceptor interface {
	Accept(ctx context.Context, inviteID uuid.UUID, callerEmail string) (uuid.UUID, organization.Role, error)
}

type ketoClient interface {
	HasRelation(ctx context.Context, namespace, object, relation, subjectID string) (bool, error)
	AddRelation(ctx context.Context, namespace, object, relation, subjectID string) error
	DeleteRelation(ctx context.Context, namespace, object, relation, subjectID string) error
}

type UserService struct {
	repo    userRepo
	orgSvc  orgCreator
	invites inviteAcceptor
	keto    ketoClient
}

func NewUserService(repo userRepo, orgSvc orgCreator, invites inviteAcceptor, keto ketoClient) *UserService {
	return &UserService{repo: repo, orgSvc: orgSvc, invites: invites, keto: keto}
}

// GetOrCreate satisfies middleware.UserResolver.
func (s *UserService) GetOrCreate(ctx context.Context, identityID uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
	u, err := s.repo.GetByIdentityID(ctx, identityID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return uuid.UUID{}, nil, err
	}
	if err == nil {
		return u.ID, u.OrganizationID, nil
	}

	id, err := uuid.NewV7()
	if err != nil {
		return uuid.UUID{}, nil, err
	}
	u, err = s.repo.Create(ctx, id, identityID)
	if err != nil {
		return uuid.UUID{}, nil, err
	}
	return u.ID, u.OrganizationID, nil
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) JoinViaInvite(ctx context.Context, userID uuid.UUID, email string, inviteID uuid.UUID) (*User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.OrganizationID != nil {
		return nil, ErrAlreadyInOrg
	}

	orgID, role, err := s.invites.Accept(ctx, inviteID, email)
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.UpdateOrganization(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}

	if err := s.keto.AddRelation(ctx, "Organization", orgID.String(), string(role), userID.String()); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *UserService) CreateWithOrg(ctx context.Context, userID uuid.UUID, orgName string) (*User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.OrganizationID != nil {
		return nil, ErrAlreadyInOrg
	}

	orgID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	if _, err := s.orgSvc.Create(ctx, orgID, orgName); err != nil {
		return nil, err
	}

	updated, err := s.repo.UpdateOrganization(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}

	if err := s.keto.AddRelation(ctx, "Organization", orgID.String(), string(organization.RoleAdmin), userID.String()); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *UserService) ChangeRole(ctx context.Context, callerID, callerOrgID, targetID uuid.UUID, newRole organization.Role) error {
	if !newRole.IsValid() {
		return organization.ErrInvalidRole
	}

	callerRole, err := s.resolveRole(ctx, callerOrgID, callerID)
	if err != nil {
		return err
	}

	targetRole, err := s.resolveRole(ctx, callerOrgID, targetID)
	if err != nil {
		return err
	}

	if err := s.checkChangeRolePermission(callerRole, targetRole, newRole, callerID, targetID); err != nil {
		return err
	}

	if err := s.keto.DeleteRelation(ctx, "Organization", callerOrgID.String(), string(targetRole), targetID.String()); err != nil {
		return err
	}
	return s.keto.AddRelation(ctx, "Organization", callerOrgID.String(), string(newRole), targetID.String())
}

func (s *UserService) checkChangeRolePermission(callerRole, targetRole, newRole organization.Role, callerID, targetID uuid.UUID) error {
	if callerRole == organization.RoleAdmin {
		return nil
	}
	if callerRole != organization.RoleHumanResource {
		return organization.ErrUnauthorized
	}
	if callerID == targetID {
		return organization.ErrUnauthorized
	}
	if targetRole.IsAdminOrHR() {
		return organization.ErrUnauthorized
	}
	if newRole.IsAdminOrHR() {
		return organization.ErrUnauthorized
	}
	return nil
}

func (s *UserService) resolveRole(ctx context.Context, orgID, userID uuid.UUID) (organization.Role, error) {
	for _, r := range organization.Roles() {
		ok, err := s.keto.HasRelation(ctx, "Organization", orgID.String(), string(r), userID.String())
		if err != nil {
			return "", err
		}
		if ok {
			return r, nil
		}
	}
	return "", ErrRoleNotFound
}
