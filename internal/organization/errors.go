package organization

import (
	"errors"
	"net/http"
)

var (
	ErrNotFound              = errors.New("organization not found")
	ErrInviteNotFound        = errors.New("invite not found")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrInvalidRole           = errors.New("invalid role")
	ErrInviteExpired         = errors.New("invite expired")
	ErrInviteAlreadyAccepted = errors.New("invite already accepted")
	ErrInviteNotForUser      = errors.New("invite not for this user")
)

func HTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrInviteNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrUnauthorized):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidRole):
		return http.StatusBadRequest
	case errors.Is(err, ErrInviteExpired), errors.Is(err, ErrInviteAlreadyAccepted), errors.Is(err, ErrInviteNotForUser):
		return http.StatusUnprocessableEntity
	}
	return http.StatusInternalServerError
}
