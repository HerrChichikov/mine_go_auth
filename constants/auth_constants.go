package constants

import (
	"time"
)

const ENV_JWT_SECRET string = "JWT_SECRET"

const DEFAULT_JWT_SECRET string = "default_secret"

const SESSION_EXPIRES_DELAY time.Duration = 7 * 24 * time.Hour
const RESERVATION_TTL time.Duration = 24 * time.Hour
