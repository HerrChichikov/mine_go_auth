package modules

import (
	"alpha/constants"
	"alpha/internal/modules/logger"
	"alpha/internal/objects/config"
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func NewPostgresDB(cfg *config.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		constants.PG_CONNECT,
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.SSLMode,
	)

	db, err := sql.Open("pgx", dsn)
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

func ApplyMigrations(db *sql.DB) error {
	logger.Log.Info("Приминяю миграции...")

	goose.SetBaseFS(migrationsFS)
	defer goose.SetBaseFS(nil)

	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}

	logger.Log.Info("Все миграции применены успешно")
	return nil
}
