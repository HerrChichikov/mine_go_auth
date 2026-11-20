package dto

import (
	"alpha/internal/objects/models"
	"time"

	"github.com/google/uuid"
)

type UserPublic struct {
	ID              uuid.UUID       `json:"id"`
	Nickname        string          `json:"nickname"`
	Role            models.UserRole `json:"role"`
	EmailVerifiedAt *time.Time      `json:"email_verified_at"`
	CreatedAt       time.Time       `json:"created_at"`
}

type RegisterRequest struct {
	Email          string `json:"email" validate:"required,email"`
	Nickname       string `json:"nickname" validate:"required,min=3,max=50"`
	Password       string `json:"password" validate:"required,min=8"`
	InvitationCode string `json:"invitation_code" validate:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expires_at"`
	User      UserPublic `json:"user"`
}
