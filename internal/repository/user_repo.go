package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"alpha/internal/interfaces"
	uerr "alpha/internal/objects/errors"
	"alpha/internal/objects/models"
	rsql "alpha/internal/objects/sql"

	"github.com/google/uuid"
)

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) interfaces.UserInterface {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := r.db.ExecContext(
		ctx, rsql.CreateUser,
		user.ID,
		user.Email,
		user.Nickname,
		user.PasswordHash,
		user.Role,
		user.IsActive,
		user.MaxReferrals,
		user.ReferralsCount,
		user.ReferredBy,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return r.getUser(ctx, rsql.GetUserByID, id)
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.getUser(ctx, rsql.GetByEmail, email)
}

func (r *userRepository) GetByNickname(ctx context.Context, nickname string) (*models.User, error) {
	return r.getUser(ctx, rsql.GetByNickname, nickname)
}

func (r *userRepository) getUser(ctx context.Context, query string, args ...any) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.Email,
		&user.Nickname,
		&user.PasswordHash,
		&user.Role,
		&user.IsActive,
		&user.MaxReferrals,
		&user.ReferralsCount,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.EmailVerifiedAt,
		&user.LastLoginAt,
		&user.LastSeenAt,
		&user.ReferredBy,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, uerr.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, rsql.ExistsByEmail, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("exists by email: %w", err)
	}
	return exists, nil
}

func (r *userRepository) ExistsByNickname(ctx context.Context, nickname string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, rsql.ExistsByNickname, nickname).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("exists by nickname: %w", err)
	}
	return exists, nil
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, rsql.UpdateLastLogin, id)
	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

func (r *userRepository) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, rsql.UpdateLastSeen, id)
	if err != nil {
		return fmt.Errorf("update last seen: %w", err)
	}
	return nil
}

func (r *userRepository) VerifyEmail(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, rsql.VerifyEmail, id)
	if err != nil {
		return fmt.Errorf("verify email: %w", err)
	}
	return nil
}

func (r *userRepository) IncrementReferralsCount(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, rsql.IncrementReferrals, id)
	if err != nil {
		return fmt.Errorf("increment referrals: %w", err)
	}
	return nil
}

func (r *userRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, rsql.SoftDelete, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	return nil
}
