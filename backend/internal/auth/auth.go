// Package auth provides password login and revocable Bearer sessions using SCS.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/alexedwards/scs/v2"
	"golang.org/x/crypto/bcrypt"
)

const (
	PasswordCost     = 12
	MinPasswordBytes = 12
	MaxPasswordBytes = 72
)

var (
	ErrUnauthorized = errors.New("invalid credentials or session")
	ErrUserNotFound = errors.New("auth user not found")
)

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	AuthProvider string `json:"authProvider"`
}

type Account struct {
	User
	PasswordHash []byte
	Version      int64
}

type Repository interface {
	ByUsername(context.Context, string) (Account, error)
	ByID(context.Context, string) (Account, error)
}

type LoginResponse struct {
	AccessToken string    `json:"accessToken"`
	TokenType   string    `json:"tokenType"`
	ExpiresAt   time.Time `json:"expiresAt"`
	User        User      `json:"user"`
}

type Service struct {
	users     Repository
	sessions  *scs.SessionManager
	dummyHash []byte
}

func New(users Repository, store scs.Store, lifetime time.Duration) (*Service, error) {
	if lifetime <= 0 {
		return nil, errors.New("session lifetime must be positive")
	}
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("unknown-account-timing-placeholder"), PasswordCost)
	if err != nil {
		return nil, fmt.Errorf("prepare password verifier: %w", err)
	}
	sessions := scs.New()
	sessions.Store = store
	sessions.Lifetime = lifetime
	sessions.IdleTimeout = 0 // expiresAt is an absolute deadline, never silently extended.
	sessions.HashTokenInStore = true
	return &Service{users: users, sessions: sessions, dummyHash: dummyHash}, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (LoginResponse, error) {
	account, err := s.users.ByUsername(ctx, username)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return LoginResponse{}, fmt.Errorf("find auth user: %w", err)
	}
	hash := account.PasswordHash
	if errors.Is(err, ErrUserNotFound) {
		hash = s.dummyHash
	}
	passwordErr := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil || passwordErr != nil || len(password) > MaxPasswordBytes {
		return LoginResponse{}, ErrUnauthorized
	}
	return s.startSession(ctx, account)
}

func (s *Service) startSession(ctx context.Context, account Account) (LoginResponse, error) {
	// Ignore any incoming session on login. SCS creates a fresh unpredictable token.
	ctx, err := s.sessions.Load(ctx, "")
	if err != nil {
		return LoginResponse{}, fmt.Errorf("create session: %w", err)
	}
	s.sessions.Put(ctx, "userID", account.ID)
	s.sessions.Put(ctx, "version", account.Version)
	s.sessions.Put(ctx, "provider", account.AuthProvider)
	token, expiresAt, err := s.sessions.Commit(ctx)
	if err != nil {
		return LoginResponse{}, fmt.Errorf("save session: %w", err)
	}
	return LoginResponse{token, "Bearer", expiresAt.UTC().Truncate(time.Millisecond), account.User}, nil
}

type userKey struct{}

// CurrentUser is available to every handler behind RequireAuth.
func CurrentUser(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userKey{}).(User)
	return user, ok
}

func (s *Service) Authenticate(ctx context.Context, token string) (context.Context, error) {
	if token == "" || len(token) > 256 {
		return ctx, ErrUnauthorized
	}
	ctx, err := s.sessions.Load(ctx, token)
	if err != nil {
		return ctx, fmt.Errorf("load session: %w", err)
	}
	id := s.sessions.GetString(ctx, "userID")
	if id == "" {
		return ctx, ErrUnauthorized
	}
	account, err := s.users.ByID(ctx, id)
	if errors.Is(err, ErrUserNotFound) {
		return ctx, ErrUnauthorized
	}
	if err != nil {
		return ctx, fmt.Errorf("find session user: %w", err)
	}
	// Changing a password invalidates all of that user's previous sessions.
	if account.Version != s.sessions.GetInt64(ctx, "version") {
		return ctx, ErrUnauthorized
	}
	return context.WithValue(ctx, userKey{}, account.User), nil
}

func (s *Service) Logout(ctx context.Context) error {
	return s.sessions.Destroy(ctx)
}

func ValidateCredentials(username, password string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	return ValidatePassword(password)
}

func ValidateUsername(username string) error {
	if username == "" || len(username) > 128 || strings.TrimSpace(username) != username || strings.ContainsFunc(username, unicode.IsControl) {
		return errors.New("логин должен содержать от 1 до 128 байт без пробелов по краям и управляющих символов")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < MinPasswordBytes {
		return fmt.Errorf("пароль слишком короткий: введено %d символов (%d байт), нужно минимум %d байт; для латиницы это %d символов", utf8.RuneCountInString(password), len(password), MinPasswordBytes, MinPasswordBytes)
	}
	if len(password) > MaxPasswordBytes {
		return fmt.Errorf("пароль слишком длинный: введено %d символов (%d байт), bcrypt допускает максимум %d байта", utf8.RuneCountInString(password), len(password), MaxPasswordBytes)
	}
	return nil
}
