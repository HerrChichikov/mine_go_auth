package repository

import (
	"context"
	"database/sql"
	"errors"
	"net/netip"
	"strings"
	"time"

	"alpha/constants"
	uerr "alpha/internal/objects/errors"
	"alpha/yggdrasil"

	"github.com/google/uuid"
)

type GameProfile struct {
	ID       uuid.UUID `json:"id"`
	Realm    string    `json:"realm"`
	Nickname string    `json:"nickname"`
	SkinURL  string    `json:"skinUrl,omitempty"`
	SkinSlim bool      `json:"skinSlim"`
}

func (p GameProfile) Wire() yggdrasil.ProfileDetails {
	var skin *yggdrasil.Skin
	if p.SkinURL != "" {
		skin = &yggdrasil.Skin{URL: p.SkinURL, Slim: p.SkinSlim}
	}
	return yggdrasil.ProfileDetails{
		Profile: yggdrasil.Profile{ID: hexUUID(p.ID), Name: p.Nickname}, Skin: skin,
	}
}

func hexUUID(id uuid.UUID) string {
	return strings.ReplaceAll(id.String(), "-", "")
}

type GameRepository struct{ db *sql.DB }

func NewGameRepository(db *sql.DB) *GameRepository { return &GameRepository{db} }

type rowScanner interface{ Scan(...any) error }

func scanGameProfile(row rowScanner) (GameProfile, error) {
	var p GameProfile
	err := row.Scan(&p.ID, &p.Realm, &p.Nickname, &p.SkinURL, &p.SkinSlim)
	if errors.Is(err, sql.ErrNoRows) {
		return p, yggdrasil.ErrNotFound
	}
	return p, err
}

const profileFields = "p.id, p.realm, p.nickname, COALESCE(p.skin_url,''), p.skin_slim"
const activeProfile = " FROM game_profiles p JOIN users u ON p.user_id = u.id AND u.is_active "

func (r *GameRepository) Profiles(ctx context.Context, userID uuid.UUID) ([]GameProfile, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+profileFields+activeProfile+"WHERE p.user_id = $1 ORDER BY p.realm", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := make([]GameProfile, 0)
	for rows.Next() {
		profile, err := scanGameProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (r *GameRepository) CreateProfile(ctx context.Context, userID uuid.UUID, realm, nickname string) (GameProfile, error) {
	return scanGameProfile(r.db.QueryRowContext(ctx, `
		INSERT INTO game_profiles AS p(user_id, realm, nickname)
		SELECT id, $2, $3 FROM users WHERE id = $1 AND is_active
		ON CONFLICT (user_id, realm) DO UPDATE SET nickname=p.nickname
		RETURNING `+profileFields, userID, realm, nickname))
}

func (r *GameRepository) SetSkin(ctx context.Context, userID, id uuid.UUID, url string, slim bool) (GameProfile, error) {
	return scanGameProfile(r.db.QueryRowContext(ctx, `
		UPDATE game_profiles AS p SET skin_url = NULLIF($3,''), skin_slim = $4
		WHERE p.user_id = $1 AND p.id = $2 AND EXISTS(SELECT 1 FROM users u WHERE u.id = $1 AND u.is_active)
		RETURNING `+profileFields, userID, id, url, slim))
}

func (r *GameRepository) Profile(ctx context.Context, realm string, id uuid.UUID) (GameProfile, error) {
	return scanGameProfile(r.db.QueryRowContext(ctx, "SELECT "+profileFields+activeProfile+"WHERE p.realm = $1 AND p.id = $2", realm, id))
}

func (r *GameRepository) ProfilesByName(ctx context.Context, realm string, names []string) ([]yggdrasil.Profile, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+profileFields+activeProfile+`
		WHERE p.realm=$1 AND lower(p.nickname)=ANY($2::text[]) ORDER BY p.nickname`, realm, names)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := make([]yggdrasil.Profile, 0)
	for rows.Next() {
		profile, err := scanGameProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile.Wire().Profile)
	}
	return profiles, rows.Err()
}

func (r *GameRepository) ProfileOwner(ctx context.Context, realm, nickname string) (uuid.UUID, error) {
	var owner uuid.UUID
	err := r.db.QueryRowContext(ctx, `SELECT p.user_id`+activeProfile+`
        WHERE p.realm=$1 AND lower(p.nickname)=lower($2)`, realm, nickname).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, yggdrasil.ErrNotFound
	}
	return owner, err
}

func (r *GameRepository) Issue(ctx context.Context, owner uuid.UUID, sessionID *uuid.UUID, profileID *uuid.UUID, realm, hash, clientToken string, expiry time.Time) (GameProfile, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return GameProfile{}, err
	}
	defer tx.Rollback()
	var active bool
	err = tx.QueryRowContext(ctx, "SELECT is_active FROM users WHERE id=$1 FOR UPDATE", owner).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !active) {
		return GameProfile{}, yggdrasil.ErrNotFound
	}
	if err != nil {
		return GameProfile{}, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM game_tokens t
        JOIN game_profiles p ON p.id=t.profile_id WHERE p.user_id=$1 AND t.expires_at > now()`, owner).Scan(&count); err != nil {
		return GameProfile{}, err
	}
	if count >= constants.GameTokenLimit {
		return GameProfile{}, uerr.ErrRateLimited
	}
	profile, err := scanGameProfile(tx.QueryRowContext(ctx, `
		WITH issued AS (
			INSERT INTO game_tokens(token_hash, profile_id, session_id, client_token, expires_at)
			SELECT $5, p.id, $2, $6, $7`+activeProfile+`
			WHERE p.user_id=$1 AND ($3::uuid IS NULL OR p.id=$3) AND ($4='' OR p.realm=$4)
			  AND ($2::uuid IS NULL OR EXISTS (
			      SELECT 1 FROM sessions s WHERE s.id=$2 AND s.user_id=$1 AND s.expires_at > now()))
			RETURNING profile_id
		)
		SELECT `+profileFields+` FROM game_profiles p JOIN issued t ON p.id=t.profile_id
	`, owner, sessionID, profileID, realm, hash, clientToken, expiry))
	if err != nil {
		return GameProfile{}, err
	}
	return profile, tx.Commit()
}

const validGame = " t.expires_at > now() AND u.is_active AND (t.session_id IS NULL OR EXISTS (SELECT 1 FROM sessions s WHERE s.id=t.session_id AND s.expires_at > now())) "
const gameTable = " game_tokens t JOIN game_profiles p ON t.profile_id=p.id JOIN users u ON p.user_id=u.id "

func (r *GameRepository) Validate(ctx context.Context, realm, hash string, clientToken *string) error {
	var ok bool
	err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM"+gameTable+`
		WHERE p.realm=$1 AND t.token_hash=$2 AND ($3::text IS NULL OR t.client_token=$3) AND `+validGame+")", realm, hash, clientToken).Scan(&ok)
	if err == nil && !ok {
		return yggdrasil.ErrInvalidToken
	}
	return err
}

func (r *GameRepository) Rotate(ctx context.Context, realm, hash, nextHash string, clientToken *string, selecting bool, expiry time.Time) (GameProfile, uuid.UUID, string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return GameProfile{}, uuid.Nil, "", err
	}
	defer tx.Rollback()
	var profileID, owner uuid.UUID
	var binding string
	err = tx.QueryRowContext(ctx, "SELECT p.id,p.user_id,t.client_token FROM"+gameTable+`
		WHERE p.realm=$1 AND t.token_hash=$2 AND ($3::text IS NULL OR t.client_token=$3)
		  AND `+validGame+" FOR UPDATE OF t", realm, hash, clientToken).Scan(&profileID, &owner, &binding)
	if errors.Is(err, sql.ErrNoRows) {
		return GameProfile{}, uuid.Nil, "", yggdrasil.ErrInvalidToken
	}
	if err != nil {
		return GameProfile{}, uuid.Nil, "", err
	}
	if selecting {
		return GameProfile{}, uuid.Nil, "", yggdrasil.ErrProfileBound
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM game_joins WHERE token_hash=$1", hash); err != nil {
		return GameProfile{}, uuid.Nil, "", err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE game_tokens SET token_hash=$2,expires_at=$3 WHERE token_hash=$1", hash, nextHash, expiry); err != nil {
		return GameProfile{}, uuid.Nil, "", err
	}
	profile, err := scanGameProfile(tx.QueryRowContext(ctx, "SELECT "+profileFields+" FROM game_profiles p WHERE p.id=$1", profileID))
	if err != nil {
		return GameProfile{}, uuid.Nil, "", err
	}
	return profile, owner, binding, tx.Commit()
}

func (r *GameRepository) Invalidate(ctx context.Context, realm, hash string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM game_tokens t USING game_profiles p WHERE p.id=t.profile_id AND p.realm=$1 AND t.token_hash=$2", realm, hash)
	return err
}

func (r *GameRepository) Signout(ctx context.Context, realm string, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM game_tokens t USING game_profiles p WHERE p.id=t.profile_id AND p.realm=$1 AND p.user_id=$2", realm, userID)
	return err
}

func (r *GameRepository) Join(ctx context.Context, realm string, id uuid.UUID, hash, challenge string, ip netip.Addr, expiry time.Time) error {
	var ipString any
	if ip.IsValid() {
		ipString = ip.String()
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO game_joins(profile_id, challenge, token_hash, client_ip, expires_at)
		SELECT p.id,$4,t.token_hash,$5::inet,$6 FROM`+gameTable+`
		WHERE p.realm=$1 AND p.id=$2 AND t.token_hash=$3 AND `+validGame+`
		ON CONFLICT (profile_id,challenge) DO UPDATE SET
		token_hash=EXCLUDED.token_hash,client_ip=EXCLUDED.client_ip,expires_at=EXCLUDED.expires_at
	`, realm, id, hash, challenge, ipString, expiry)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err == nil && count == 0 {
		return yggdrasil.ErrInvalidToken
	}
	return err
}

func (r *GameRepository) HasJoined(ctx context.Context, realm, nickname, challenge string, ip netip.Addr) (GameProfile, error) {
	var ipString any
	if ip.IsValid() {
		ipString = ip.String()
	}
	return scanGameProfile(r.db.QueryRowContext(ctx, "SELECT "+profileFields+" FROM"+gameTable+`
		JOIN game_joins j ON j.profile_id=p.id AND j.token_hash=t.token_hash
		WHERE p.realm=$1 AND lower(p.nickname)=lower($2) AND j.challenge=$3
		 AND ($4::inet IS NULL OR j.client_ip=$4) AND j.expires_at > now() AND `+validGame, realm, nickname, challenge, ipString))
}

func (r *GameRepository) Prune(ctx context.Context) error {
	for _, query := range []string{
		"DELETE FROM game_joins WHERE expires_at <= now()",
		"DELETE FROM game_tokens WHERE expires_at <= now()",
		"DELETE FROM sessions WHERE expires_at <= now()",
		"DELETE FROM auth_attempts WHERE expires_at <= now()",
	} {
		if _, err := r.db.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return nil
}
