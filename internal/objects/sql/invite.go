package sql

const baseInvitationSelect = `
	SELECT 
		id, code, created_by, used_by, used_at, 
		expires_at, created_at, reserved_at
	FROM invitations
`

const CreateInvitation = `
	INSERT INTO invitations (id, code, created_by, expires_at, created_at)
	VALUES ($1, $2, $3, $4, $5)
`

const GetInvitationByID = baseInvitationSelect + ` WHERE id = $1`
const GetInvitationByCode = baseInvitationSelect + ` WHERE code = $1`

const GetInvitationsByCreator = baseInvitationSelect + `
	WHERE created_by = $1
	ORDER BY created_at DESC
`

const ReserveInvitation = `
	UPDATE invitations
	SET reserved_at = NOW()
	WHERE code = $1
		AND used_by IS NULL
		AND (expires_at IS NULL OR expires_at > NOW())
		AND reserved_at IS NULL
`

const UseInvitation = `
	UPDATE invitations
	SET used_by = $1,
		used_at = NOW(),
		reserved_at = NULL
	WHERE id = $2 AND used_by IS NULL
`

const ClearExpiredReservations = `
	UPDATE invitations
	SET reserved_at = NULL
	WHERE reserved_at IS NOT NULL
		AND reserved_at < NOW() - INTERVAL '1 day'
		AND used_by IS NULL
`

const DeleteInvitation = `
	DELETE FROM invitations
	WHERE id = $1 AND used_by IS NULL
`

const UpdateInvitation = `
	UPDATE invitations
	SET used_by = $1,
		used_at = $2,
		expires_at = $3,
		reserved_at = $4
	WHERE id = $5
`

const MarkInvitationAsUsed = `
	UPDATE invitations
	SET used_by = $1,
		used_at = NOW()
	WHERE id = $2
`
