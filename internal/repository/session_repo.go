package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"alpha/constants"
	"alpha/internal/interfaces"
	serr "alpha/internal/objects/errors"
	"alpha/internal/objects/models"

	"github.com/google/uuid"
)

type sessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) interfaces.SessionInterface {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(ctx context.Context, session models.AccountSession) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active bool
	if err := tx.QueryRowContext(ctx, "SELECT is_active FROM users WHERE id=$1 FOR UPDATE", session.UserID).Scan(&active); err != nil {
		return err
	}
	if !active {
		return serr.ErrInvalidCredentials
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE user_id=$1 AND
        access_hash IS NOT NULL AND expires_at > now()`, session.UserID).Scan(&count); err != nil {
		return err
	}
	if count >= constants.SessionLimit {
		return serr.ErrRateLimited
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, access_hash, access_expires_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, session.ID, session.UserID, session.RefreshHash, session.AccessHash, session.AccessUntil, session.RefreshUntil)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *sessionRepository) ResolveAccess(ctx context.Context, hash string) (models.AccountSession, error) {
	var session models.AccountSession
	err := r.db.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, s.access_expires_at, s.expires_at FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.access_hash = $1 AND s.access_expires_at > now() AND s.expires_at > now() AND u.is_active
	`, hash).Scan(&session.ID, &session.UserID, &session.AccessUntil, &session.RefreshUntil)
	return session, sessionError(err)
}

func (r *sessionRepository) Rotate(ctx context.Context, refreshHash, nextRefreshHash, nextAccessHash string, accessUntil time.Time) (models.AccountSession, error) {
	var session models.AccountSession
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return session, err
	}
	defer tx.Rollback()

	var currentHash string
	var rotations int
	// Lock by stable family ID. A second query observes a concurrent rotation even
	// if an earlier snapshot saw the original token as the current one.
	var id uuid.UUID
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM sessions WHERE token_hash = $1 AND access_hash IS NOT NULL
		UNION ALL SELECT session_id FROM spent_refresh_tokens WHERE token_hash = $1
	`, refreshHash).Scan(&id)
	if err != nil {
		return session, sessionError(err)
	}
	err = tx.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, s.token_hash, s.expires_at, s.refresh_count FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = $1 AND s.expires_at > now() AND u.is_active FOR UPDATE OF s
	`, id).Scan(&session.ID, &session.UserID, &currentHash, &session.RefreshUntil, &rotations)
	if err != nil {
		return session, sessionError(err)
	}
	if currentHash != refreshHash {
		if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE id = $1", id); err != nil {
			return session, err
		}
		if err = tx.Commit(); err != nil {
			return session, err
		}
		return models.AccountSession{}, serr.ErrInvalidToken
	}

	if rotations >= constants.MaxSessionRotations {
		return session, serr.ErrRateLimited
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO spent_refresh_tokens(token_hash, session_id) VALUES ($1,$2)", refreshHash, id); err != nil {
		return session, err
	}
	if session.RefreshUntil.Before(accessUntil) {
		accessUntil = session.RefreshUntil
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE sessions SET token_hash = $2, access_hash = $3, access_expires_at = $4, last_active_at = now(), refresh_count = refresh_count + 1
		WHERE id = $1
	`, id, nextRefreshHash, nextAccessHash, accessUntil)
	if err != nil {
		return session, err
	}
	session.AccessUntil = accessUntil
	return session, tx.Commit()
}

func (r *sessionRepository) Revoke(ctx context.Context, accessHash, refreshHash string) error {
	_, err := r.db.ExecContext(ctx, `WITH family AS (
            SELECT id FROM sessions
            WHERE access_hash IS NOT NULL AND ((access_hash = $1 AND access_expires_at > now()) OR token_hash = $2)
            UNION SELECT session_id FROM spent_refresh_tokens WHERE token_hash = $2
        )
        DELETE FROM sessions WHERE id IN (SELECT id FROM family)
	`, accessHash, refreshHash)
	return err
}

func (r *sessionRepository) RevokeUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = $1", userID)
	return err
}

func sessionError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return serr.ErrInvalidToken
	}
	return err
}
