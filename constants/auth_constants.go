package constants

import (
	"time"
)

const ENV_JWT_SECRET string = "JWT_SECRET"

const DEFAULT_JWT_SECRET string = "default_secret"

const RESERVATION_TTL time.Duration = 24 * time.Hour

const (
	AccessTTL            = 10 * time.Minute
	RefreshTTL           = 30 * 24 * time.Hour
	GameTTL              = 8 * time.Hour
	JoinTTL              = 30 * time.Second
	AttemptWindow        = 5 * time.Minute
	IdentityAttemptLimit = 10
	IPAttemptLimit       = 60
	SessionLimit         = 32
	MaxSessionRotations  = 8192
	GameTokenLimit       = 64
	RequestTimeout       = 10 * time.Second
	MigrationTimeout     = 2 * time.Minute
)
