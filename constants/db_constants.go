package constants

import (
	"time"
)

const PG_CONNECT string = "postgres://%s:%s@%s:%s/%s?sslmode=%s"
const DB_MAX_OPEN_CONN int = 25                            // Максимальное кол-во открытых соединений
const DB_MAX_IDLE_CONN int = 10                            // Максимальное кол-во неактивных (idle) соединений
const DB_MAX_CONN_LIFETIME time.Duration = 5 * time.Minute // Максимальное время жизни соединения (защита от утечек и переподключения при рестарте БД)
const DB_MAX_CONN_IDLETIME time.Duration = 3 * time.Minute // Максимальное время простоя соединения
const DB_CTX_TIMEOUT time.Duration = 5 * time.Second

const ENV_DB_HOST string = "DB_HOST"
const ENV_DB_PORT string = "DB_PORT"
const ENV_DB_USER string = "DB_USER"
const ENV_DB_PASSWORD string = "DB_PASSWORD"
const ENV_DB_NAME string = "DB_NAME"
const ENV_DB_SSL_MODE string = "DB_SSLMODE"

const DB_DEFAULT_HOST string = "localhost"
const DB_DEFAULT_PORT string = "5432"
const DB_DEFAULT_USER string = "postgres"
const DB_DEFAULT_PASSWORD string = "secret"
const DB_DEFAULT_DB_NAME string = "mydb"
const DB_DEFAULT_SSL_MODE string = "disable"
