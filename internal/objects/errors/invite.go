package errors

import "errors"

var (
	ErrInvitationNotFound = errors.New("приглашение не найдено")
	ErrInvitationUsed     = errors.New("приглашение уже использовано")
	ErrInvitationExpired  = errors.New("срок действия приглашения истек")
	ErrInvitationReserved = errors.New("приглашение уже зарезервировано")
	ErrInvitationLimit    = errors.New("лимит приглашений исчерпан")
)
