package modules

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"alpha/internal/modules/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestUpgradeArchiveSchemaRetainsExistingData(t *testing.T) {
	dsn := os.Getenv("MINECRAFT_AUTH_TEST_DATABASE")
	if dsn == "" {
		t.Skip("MINECRAFT_AUTH_TEST_DATABASE points to a disposable PostgreSQL database")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*config)
	defer admin.Close()
	schema := "auth_upgrade_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.ExecContext(t.Context(), "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	defer db.Close()
	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(t.Context(), 20260620145706); err != nil {
		t.Fatal(err)
	}
	var userID, sessionID uuid.UUID
	if err := db.QueryRow(`INSERT INTO users(email, nickname, password_hash) VALUES
		('legacy@example.test', 'Legacy', 'legacy-hash') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO invitations(code, created_by) VALUES ('LEGACY', $1)`, userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO sessions(user_id, token_hash, expires_at) VALUES
		($1, 'legacy-token-hash', now() + interval '1 day') RETURNING id`, userID).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}
	logger.LoggerInit("test")
	for range 2 {
		if err := ApplyMigrations(t.Context(), db); err != nil {
			t.Fatal(err)
		}
	}
	var user, invitation, legacySession bool
	err = db.QueryRow(`SELECT
		EXISTS (SELECT 1 FROM users WHERE id=$1 AND email='legacy@example.test' AND password_hash='legacy-hash'),
		EXISTS (SELECT 1 FROM invitations WHERE created_by=$1 AND code='LEGACY' AND used_by IS NULL),
		EXISTS (SELECT 1 FROM sessions WHERE id=$2 AND user_id=$1 AND access_hash IS NULL AND token_hash='legacy-token-hash')`,
		userID, sessionID).Scan(&user, &invitation, &legacySession)
	if err != nil || !user || !invitation || !legacySession {
		t.Fatalf("upgrade changed legacy data: account=%t invite=%t session=%t err=%v", user, invitation, legacySession, err)
	}
}
