package sql

const baseSessionSelect = `
	SELECT 
		id, user_id, token_hash, ip_address, user_agent,
		created_at, expires_at, last_active_at
	FROM sessions
`

const CreateSession = `
	INSERT INTO sessions (
		id, user_id, token_hash, ip_address, user_agent,
		created_at, expires_at, last_active_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`

const GetSessionByID = baseSessionSelect + ` WHERE id = $1`
const GetSessionByTokenHash = baseSessionSelect + ` WHERE token_hash = $1`

const GetSessionsByUserID = baseSessionSelect + `
	WHERE user_id = $1
	ORDER BY created_at DESC
`

const UpdateSessionLastActive = `
	UPDATE sessions
	SET last_active_at = NOW()
	WHERE id = $1
`

const DeleteSession = `
	DELETE FROM sessions
	WHERE id = $1
`

const DeleteSessionsByUserID = `
	DELETE FROM sessions
	WHERE user_id = $1
`

const DeleteExpiredSessions = `
	DELETE FROM sessions
	WHERE expires_at < NOW()
`
