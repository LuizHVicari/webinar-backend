package user

import (
	"errors"
	"net/http"

	"github.com/LuizHVicari/webinar-backend/internal/organization"
)

var (
	ErrNotFound     = errors.New("user not found")
	ErrAlreadyInOrg = errors.New("user already belongs to an organization")
	ErrRoleNotFound = errors.New("user has no role in this organization")
)

func HTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrAlreadyInOrg):
		return http.StatusConflict
	case errors.Is(err, ErrRoleNotFound):
		return http.StatusUnprocessableEntity
	}
	return organization.HTTPStatus(err)
}
