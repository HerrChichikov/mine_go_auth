package errors

import "errors"

var (
	ErrRateLimited  = errors.New("authentication limit reached")
	ErrInvalidToken = errors.New("недействительный токен")
	ErrTokenExpired = errors.New("срок действия токена истек")
)
