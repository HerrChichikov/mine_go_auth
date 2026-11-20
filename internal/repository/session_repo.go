package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"alpha/internal/interfaces"
	serr "alpha/internal/objects/errors"
	"alpha/internal/objects/models"
	rsql "alpha/internal/objects/sql"

	"github.com/google/uuid"
)

type sessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) interfaces.SessionInterface {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(ctx context.Context, session *models.Session) error {
	_, err := r.db.ExecContext(
		ctx, rsql.CreateSession,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.IPAddress,
		session.UserAgent,
		session.CreatedAt,
		session.ExpiresAt,
		session.LastActiveAt,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *sessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Session, error) {
	return r.getOne(ctx, rsql.GetSessionByID, id)
}

func (r *sessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.Session, error) {
	return r.getOne(ctx, rsql.GetSessionByTokenHash, tokenHash)
}

func (r *sessionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Session, error) {
	rows, err := r.db.QueryContext(ctx, rsql.GetSessionsByUserID, userID)
	if err != nil {
		return nil, fmt.Errorf("get sessions by user: %w", err)
	}
	defer rows.Close()

	var result []*models.Session
	for rows.Next() {
		session, err := r.scanSession(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, session)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}
	return result, nil
}

func (r *sessionRepository) UpdateLastActive(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, rsql.UpdateSessionLastActive, id)
	if err != nil {
		return fmt.Errorf("update last active: %w", err)
	}
	return nil
}

func (r *sessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, rsql.DeleteSession, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return serr.ErrSessionNotFound
	}
	return nil
}

func (r *sessionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, rsql.DeleteSessionsByUserID, userID)
	if err != nil {
		return fmt.Errorf("delete sessions by user: %w", err)
	}
	return nil
}

func (r *sessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, rsql.DeleteExpiredSessions)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return res.RowsAffected()
}

func (r *sessionRepository) getOne(ctx context.Context, query string, args ...any) (*models.Session, error) {
	row := r.db.QueryRowContext(ctx, query, args...)

	session := &models.Session{}
	err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.IPAddress,
		&session.UserAgent,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastActiveAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, serr.ErrSessionNotFound
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}

	return session, nil
}

func (r *sessionRepository) scanSession(rows *sql.Rows) (*models.Session, error) {
	session := &models.Session{}
	err := rows.Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.IPAddress,
		&session.UserAgent,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastActiveAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, serr.ErrSessionNotFound
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}
	return session, nil
}
