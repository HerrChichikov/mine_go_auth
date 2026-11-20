package handlers

import (
	cerr "alpha/internal/objects/errors"
	"errors"
	"fmt"
	"net/http"
)

func MapErrorToHTTP(err error) (int, string) {
	switch {
	case errors.Is(err, cerr.ErrUserNotFound),
		errors.Is(err, cerr.ErrInvitationNotFound),
		errors.Is(err, cerr.ErrSessionNotFound):
		return http.StatusNotFound, err.Error()

	case errors.Is(err, cerr.ErrInvalidInput),
		errors.Is(err, cerr.ErrInvalidCredentials),
		errors.Is(err, cerr.ErrInvalidToken):
		return http.StatusBadRequest, err.Error()

	case errors.Is(err, cerr.ErrEmailExists),
		errors.Is(err, cerr.ErrNicknameExists),
		errors.Is(err, cerr.ErrUserExists):
		return http.StatusConflict, err.Error()

	case errors.Is(err, cerr.ErrInvitationUsed),
		errors.Is(err, cerr.ErrInvitationExpired),
		errors.Is(err, cerr.ErrInvitationReserved),
		errors.Is(err, cerr.ErrInvitationLimit),
		errors.Is(err, cerr.ErrSessionExpired),
		errors.Is(err, cerr.ErrTokenExpired):
		return http.StatusForbidden, err.Error()

	case errors.Is(err, cerr.ErrUserInactive):
		return http.StatusForbidden, err.Error()

	default:
		return http.StatusInternalServerError, fmt.Sprintf("Внутреняя ошибка сервера: %s", err)
	}
}
