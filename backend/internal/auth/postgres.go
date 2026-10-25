package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) ByUsername(ctx context.Context, username string) (Account, error) {
	return scanAccount(p.pool.QueryRow(ctx, "SELECT id::text, username, password_hash, session_version FROM app_users WHERE username = $1 AND disabled_at IS NULL AND password_hash IS NOT NULL", username))
}

func (p *Postgres) ByID(ctx context.Context, id string) (Account, error) {
	return scanAccount(p.pool.QueryRow(ctx, "SELECT id::text, username, password_hash, session_version FROM app_users WHERE id = $1 AND disabled_at IS NULL", id))
}

func (p *Postgres) LinkOIDC(ctx context.Context, issuer, subject, username string) (Account, error) {
	return scanAccount(p.pool.QueryRow(ctx, `
		INSERT INTO app_users (username, oidc_issuer, oidc_subject) VALUES ($1, $2, $3)
		ON CONFLICT (oidc_issuer, oidc_subject) DO UPDATE SET username = EXCLUDED.username
		WHERE app_users.disabled_at IS NULL
		RETURNING id::text, username, password_hash, session_version`, username, issuer, subject))
}

func scanAccount(row pgx.Row) (Account, error) {
	var account Account
	var hash *string
	err := row.Scan(&account.ID, &account.Username, &hash, &account.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrUserNotFound
	}
	account.AuthProvider = "oidc"
	if hash != nil {
		account.PasswordHash = []byte(*hash)
		account.AuthProvider = "password"
	}
	return account, err
}

// SetPassword creates a user or changes their password, revoking older sessions by version.
func (p *Postgres) SetPassword(ctx context.Context, username, password string) error {
	if err := ValidateCredentials(username, password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), PasswordCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO app_users (username, password_hash) VALUES ($1, $2)
		ON CONFLICT (username) DO UPDATE
		SET password_hash = EXCLUDED.password_hash, session_version = app_users.session_version + 1`, username, string(hash))
	return err
}
