package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"alpha/constants"
)

type AttemptRepository struct{ db *sql.DB }

func NewAttemptRepository(db *sql.DB) *AttemptRepository { return &AttemptRepository{db} }

func (r *AttemptRepository) AllowIP(ctx context.Context, operation, ip string) (bool, error) {
	return r.take(ctx, operation+":ip:"+ip, constants.IPAttemptLimit)
}

func (r *AttemptRepository) AllowLogin(ctx context.Context, identity string) (bool, error) {
	hash := sha256.Sum256([]byte(strings.ToLower(identity)))
	return r.take(ctx, "login:identity:"+hex.EncodeToString(hash[:]), constants.IdentityAttemptLimit)
}

func (r *AttemptRepository) take(ctx context.Context, key string, limit int) (bool, error) {
	expiry := time.Now().UTC().Add(constants.AttemptWindow)
	var count int
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO auth_attempts(bucket, expires_at, attempts) VALUES ($1,$2,1)
		ON CONFLICT(bucket) DO UPDATE SET
			expires_at=CASE WHEN auth_attempts.expires_at <= now() THEN EXCLUDED.expires_at ELSE auth_attempts.expires_at END,
			attempts=CASE WHEN auth_attempts.expires_at <= now() THEN 1 ELSE LEAST(auth_attempts.attempts + 1, $3+1) END
		RETURNING attempts
	`, key, expiry, limit).Scan(&count)
	return count <= limit, err
}
