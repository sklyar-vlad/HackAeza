package auth_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/auth"
	"github.com/sklyar-vlad/HackAeza/backend/internal/platform/migrations"
	"golang.org/x/crypto/bcrypt"
)

// Uses a unique schema, so this test never clears application users or sessions.
func TestPostgresAuthLifecycle(t *testing.T) {
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to run PostgreSQL integration checks")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("configure test database")
	}
	defer admin.Close()
	schema := "auth_test_" + rand.Text()
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("configure isolated schema")
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = identifier
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repository := auth.NewPostgres(pool)
	var status bytes.Buffer
	if err := migrations.Run(ctx, pool, "status", &status); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(status.Bytes(), []byte("pending")) {
		t.Fatal("fresh database should have a pending migration")
	}
	if err := migrations.Check(ctx, pool); err == nil {
		t.Fatal("application accepted a database before migration")
	}
	for i := 0; i < 2; i++ {
		if err := migrations.Run(ctx, pool, "up", io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.SetPassword(ctx, "analyst", password); err != nil {
		t.Fatal(err)
	}
	account, err := repository.ByUsername(ctx, "analyst")
	if err != nil {
		t.Fatal(err)
	}
	if cost, err := bcrypt.Cost(account.PasswordHash); err != nil || cost != auth.PasswordCost {
		t.Fatal("password was not hashed with configured bcrypt cost")
	}
	newService := func() *auth.Service {
		service, err := auth.New(repository, repository, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return service
	}
	service := newService()
	session, err := service.Login(ctx, "analyst", password)
	if err != nil {
		t.Fatal(err)
	}
	var hash, data []byte
	var userID, provider string
	if err := pool.QueryRow(ctx, "SELECT token_hash, session_data, user_id::text, auth_provider FROM auth_sessions").Scan(&hash, &data, &userID, &provider); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte(session.AccessToken))
	if !bytes.Equal(hash, want[:]) || bytes.Contains(data, []byte(session.AccessToken)) || userID != account.ID || provider != "password" {
		t.Fatal("stored session violates project schema or contains raw token")
	}
	// Model an existing database created before Goose: schema and credentials must survive adoption.
	if _, err := pool.Exec(ctx, `DROP TABLE goose_db_version;
		CREATE TABLE auth_schema_migrations (version TEXT PRIMARY KEY, checksum BYTEA NOT NULL);
		INSERT INTO auth_schema_migrations VALUES ('000001_auth.sql', 'old-checksum')`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(ctx, pool, "up", io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, session.AccessToken); err != nil {
		t.Fatal("adopting existing tables invalidated the stored user or session")
	}
	// New service instance must load the existing session from PostgreSQL.
	restarted := newService()
	authCtx, err := restarted.Authenticate(ctx, session.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Logout(authCtx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, session.AccessToken); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal("PostgreSQL logout did not revoke session across instances")
	}
	var revoked bool
	if err := pool.QueryRow(ctx, "SELECT revoked_at IS NOT NULL FROM auth_sessions WHERE token_hash = $1", hash).Scan(&revoked); err != nil || !revoked {
		t.Fatal("revocation was not persisted")
	}
	session, err = service.Login(ctx, "analyst", password)
	if err != nil {
		t.Fatal(err)
	}
	expiredHash := sha256.Sum256([]byte(session.AccessToken))
	if _, err := pool.Exec(ctx, "UPDATE auth_sessions SET expires_at = now() - interval '1 second' WHERE token_hash = $1", expiredHash[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, session.AccessToken); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal("expired PostgreSQL session retained access")
	}
	session, err = service.Login(ctx, "analyst", password)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SetPassword(ctx, "analyst", "replacement-test-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, session.AccessToken); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal("password change did not revoke old sessions")
	}
	if _, err := service.Login(ctx, "analyst", password); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal("old password still works")
	}
	session, err = service.Login(ctx, "analyst", "replacement-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE app_users SET disabled_at = now() WHERE id = $1", account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, session.AccessToken); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal("disabled employee retained access")
	}
	if _, err := service.Login(ctx, "analyst", "replacement-test-password"); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal("disabled employee can sign in")
	}
	if err := repository.Prune(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Check(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(ctx, pool, "down", io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Check(ctx, pool); err == nil {
		t.Fatal("application accepted the schema after a rollback")
	}
	if err := migrations.Run(ctx, pool, "up", io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Check(ctx, pool); err != nil {
		t.Fatal(err)
	}
}
