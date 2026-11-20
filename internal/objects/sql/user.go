package sql

const baseUserSelect = `
	SELECT 
		id, COALESCE(email, ''), nickname, password_hash, role, is_active,
		max_referrals, referrals_count,
		created_at, updated_at, email_verified_at, 
		last_login_at, last_seen_at, referred_by
	FROM users 
	WHERE is_active = true
`

const CreateUser = `
	INSERT INTO users (
		id, email, nickname, password_hash, role, is_active,
		max_referrals, referrals_count, referred_by,
		created_at, updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
	)
`

// Каждый запрос имеет свой WHERE
const GetUserByID = baseUserSelect + ` AND id = $1`
const GetByEmail = baseUserSelect + ` AND email = $1`
const GetByNickname = baseUserSelect + ` AND nickname = $1`

const ExistsByEmail = `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND is_active = true)`
const ExistsByNickname = `SELECT EXISTS(SELECT 1 FROM users WHERE nickname = $1 AND is_active = true)`

const UpdateLastLogin = `UPDATE users SET last_login_at = NOW() WHERE id = $1`
const UpdateLastSeen = `UPDATE users SET last_seen_at = NOW() WHERE id = $1`
const VerifyEmail = `UPDATE users SET email_verified_at = NOW() WHERE id = $1`

const IncrementReferrals = `
	UPDATE users 
	SET referrals_count = referrals_count + 1 
	WHERE id = $1
`

const SoftDelete = `UPDATE users SET is_active = false WHERE id = $1`
