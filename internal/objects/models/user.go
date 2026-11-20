package models

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	UserRoleUser      UserRole = "user"
	UserRoleVIP       UserRole = "vip"
	UserRoleModerator UserRole = "moderator"
	UserRoleAdmin     UserRole = "admin"
)

type User struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	Email           string     `db:"email" json:"email"`
	Nickname        string     `db:"nickname" json:"nickname"`
	PasswordHash    string     `db:"password_hash" json:"-"` // json:"-" чтобы не отдавать в API
	Role            UserRole   `db:"role" json:"role"`
	IsActive        bool       `db:"is_active" json:"is_active"`
	MaxReferrals    int        `db:"max_referrals" json:"max_referrals"`
	ReferralsCount  int        `db:"referrals_count" json:"referrals_count"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
	EmailVerifiedAt *time.Time `db:"email_verified_at" json:"email_verified_at"`
	LastLoginAt     *time.Time `db:"last_login_at" json:"last_login_at"`
	LastSeenAt      *time.Time `db:"last_seen_at" json:"last_seen_at"`
	ReferredBy      *uuid.UUID `db:"referred_by" json:"referred_by"`
}

func (u *User) IsEmailVerified() bool {
	return u.EmailVerifiedAt != nil
}

func (u *User) IsOnline() bool {
	if u.LastSeenAt == nil {
		return false
	}
	threshold := time.Now().Add(-5 * time.Minute)
	return u.LastSeenAt.After(threshold)
}

func (u *User) CanCreateInvitation() bool {
	return u.ReferralsCount < u.MaxReferrals
}

func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

func (u *User) IsModerator() bool {
	return u.Role == UserRoleModerator || u.Role == UserRoleAdmin
}
