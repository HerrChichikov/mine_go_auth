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
	"github.com/jackc/pgx/v5/pgconn"
)

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) interfaces.UserInterface {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	return createUser(ctx, r.db, user)
}

func createUser(ctx context.Context, q interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, user *models.User) error {
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	var email any
	if user.Email != "" {
		email = user.Email
	}
	_, err := q.ExecContext(
		ctx, rsql.CreateUser,
		user.ID,
		email,
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
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return uerr.ErrUserExists
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *userRepository) CreateWithInvitation(ctx context.Context, user *models.User, code string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var invitationID, creator uuid.UUID
	err = tx.QueryRowContext(ctx, `
		SELECT id, created_by FROM invitations WHERE code = $1 AND used_by IS NULL
			AND (expires_at IS NULL OR expires_at > now()) FOR UPDATE
	`, code).Scan(&invitationID, &creator)
	if errors.Is(err, sql.ErrNoRows) {
		return uerr.ErrInvitationNotFound
	}
	if err != nil {
		return err
	}
	user.ReferredBy = &invitationID
	if err := createUser(ctx, tx, user); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, rsql.UseInvitation, user.ID, invitationID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, rsql.IncrementReferrals, creator); err != nil {
		return err
	}
	return tx.Commit()
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
