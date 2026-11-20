package errors

import "errors"

var (
	ErrInvalidToken = errors.New("недействительный токен")
	ErrTokenExpired = errors.New("срок действия токена истек")
)
