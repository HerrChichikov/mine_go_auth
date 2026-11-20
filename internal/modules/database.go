package modules

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/url"

	"alpha/constants"
	"alpha/internal/modules/logger"
	"alpha/internal/objects/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func NewPostgresDB(cfg *config.DatabaseConfig) (*sql.DB, error) {
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, cfg.Port),
		Path:     "/" + cfg.DBName,
		RawQuery: url.Values{"sslmode": {cfg.SSLMode}}.Encode(),
	}

	db, err := sql.Open("pgx", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("failed to open db connection: %w", err)
	}

	db.SetMaxOpenConns(constants.DB_MAX_OPEN_CONN)
	db.SetMaxIdleConns(constants.DB_MAX_IDLE_CONN)
	db.SetConnMaxLifetime(constants.DB_MAX_CONN_LIFETIME)
	db.SetConnMaxIdleTime(constants.DB_MAX_CONN_IDLETIME)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		constants.DB_CTX_TIMEOUT,
	)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close() // Обязательно закрываем, если пинг не прошел
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	logger.Log.Info("Подключение к БД: успешно")
	return db, nil
}

func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations, goose.WithSessionLocker(locker))
	if err != nil {
		return err
	}
	if _, err := provider.Up(ctx); err != nil {
		return err
	}
	logger.Log.Info("Все миграции применены успешно")
	return nil
}
