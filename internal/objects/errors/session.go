package errors

import "errors"

var (
	ErrSessionNotFound = errors.New("сессия не найдена")
	ErrSessionExpired  = errors.New("сессия истекла")
)
