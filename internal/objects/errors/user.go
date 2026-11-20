package errors

import "errors"

var (
	ErrUserNotFound        = errors.New("пользователь не найден")
	ErrUserExists          = errors.New("пользователь уже существует")
	ErrEmailExists         = errors.New("email уже занят")
	ErrNicknameExists      = errors.New("никнейм уже занят")
	ErrInvalidCredentials  = errors.New("неверный email или пароль")
	ErrUserInactive        = errors.New("аккаунт деактивирован")
)