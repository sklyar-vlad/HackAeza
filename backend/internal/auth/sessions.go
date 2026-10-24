package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5"
)

// These methods adapt SCS's store contract to the project's auth_sessions schema.
// SCS hashes the token before invoking the store; its wire token is never persisted.
func (p *Postgres) FindCtx(ctx context.Context, token string) ([]byte, bool, error) {
	hash, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, false, err
	}
	var data []byte
	err = p.pool.QueryRow(ctx, `SELECT session_data FROM auth_sessions
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`, hash).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	return data, err == nil, err
}

func (p *Postgres) CommitCtx(ctx context.Context, token string, data []byte, expiry time.Time) error {
	hash, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return err
	}
	_, values, err := (scs.GobCodec{}).Decode(data)
	if err != nil {
		return err
	}
	userID, ok := values["userID"].(string)
	if !ok || userID == "" {
		return errors.New("session has no user")
	}
	result, err := p.pool.Exec(ctx, `INSERT INTO auth_sessions
		(token_hash, user_id, auth_provider, expires_at, session_data)
		VALUES ($1, $2, 'password', $3, $4)
		ON CONFLICT (token_hash) DO UPDATE SET session_data = EXCLUDED.session_data, expires_at = EXCLUDED.expires_at
		WHERE auth_sessions.revoked_at IS NULL`, hash, userID, expiry, data)
	if err == nil && result.RowsAffected() == 0 {
		return errors.New("cannot save revoked session")
	}
	return err
}

func (p *Postgres) DeleteCtx(ctx context.Context, token string) error {
	hash, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, "UPDATE auth_sessions SET revoked_at = COALESCE(revoked_at, now()) WHERE token_hash = $1", hash)
	return err
}

func (p *Postgres) Find(token string) ([]byte, bool, error) {
	return p.FindCtx(context.Background(), token)
}

func (p *Postgres) Commit(token string, data []byte, expiry time.Time) error {
	return p.CommitCtx(context.Background(), token, data, expiry)
}

func (p *Postgres) Delete(token string) error {
	return p.DeleteCtx(context.Background(), token)
}

// Prune retains expired/revoked sessions for one day, then removes their payloads.
func (p *Postgres) Prune(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM auth_sessions
		WHERE expires_at < now() - interval '1 day' OR revoked_at < now() - interval '1 day'`)
	return err
}
