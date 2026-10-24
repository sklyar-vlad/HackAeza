package auth_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/sklyar-vlad/HackAeza/backend/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

const password = "correct-test-password"

type users struct {
	account auth.Account
	err     error
}

func (u *users) ByUsername(_ context.Context, username string) (auth.Account, error) {
	if u.err != nil {
		return auth.Account{}, u.err
	}
	if username != u.account.Username {
		return auth.Account{}, auth.ErrUserNotFound
	}
	return u.account, nil
}

func (u *users) ByID(_ context.Context, id string) (auth.Account, error) {
	if u.err != nil {
		return auth.Account{}, u.err
	}
	if id != u.account.ID {
		return auth.Account{}, auth.ErrUserNotFound
	}
	return u.account, nil
}

func fixture(t *testing.T) (*users, *memstore.MemStore, http.Handler) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	u := &users{account: auth.Account{User: auth.User{ID: "user-1", Username: "analyst", AuthProvider: "password"}, PasswordHash: hash, Version: 1}}
	store := memstore.NewWithCleanupInterval(0)
	return u, store, makeHandler(t, u, store)
}

func makeHandler(t *testing.T, u *users, store scs.Store) http.Handler {
	t.Helper()
	service, err := auth.New(u, store, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h := auth.NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	root, api := http.NewServeMux(), http.NewServeMux()
	h.Register(root, api)
	api.HandleFunc("GET /api/v1/private", func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.CurrentUser(r.Context())
		if !ok {
			t.Error("protected handler has no identity")
		}
		auth.JSON(w, r, 200, user)
	})
	root.Handle("/api/v1/", h.RequireAuth(api))
	return auth.RequestID(root)
}

func request(handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func login(t *testing.T, handler http.Handler) auth.LoginResponse {
	t.Helper()
	w := request(handler, "POST", "/api/v1/auth/login", "ignored-old-token", `{"username":"analyst","password":"`+password+`"}`)
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	var response auth.LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.TokenType != "Bearer" || response.User.Username != "analyst" || response.User.AuthProvider != "password" {
		t.Fatalf("invalid login contract: %+v", response.User)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(response.AccessToken)
	if err != nil || len(decoded) != 32 {
		t.Fatal("session token must have 256 bits from SCS")
	}
	if !response.ExpiresAt.After(time.Now()) || response.ExpiresAt.After(time.Now().Add(time.Hour)) {
		t.Fatal("incorrect session expiry")
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
		t.Fatal("Bearer response must not be cached or set a session cookie")
	}
	return response
}

func TestLoginMeProtectedAndLogout(t *testing.T) {
	_, store, handler := fixture(t)
	first, second := login(t, handler), login(t, handler)
	if first.AccessToken == second.AccessToken {
		t.Fatal("separate logins reused a token")
	}
	if _, found, _ := store.Find(first.AccessToken); found {
		t.Fatal("raw Bearer token was persisted")
	}
	for _, path := range []string{"/api/v1/auth/me", "/api/v1/private"} {
		w := request(handler, "GET", path, first.AccessToken, "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"id":"user-1"`) {
			t.Fatalf("protected identity: %d %s", w.Code, w.Body.String())
		}
	}
	w := request(handler, "POST", "/api/v1/auth/logout", first.AccessToken, "")
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal("logout must return empty 204")
	}
	for _, method := range []string{"GET", "POST"} {
		path := "/api/v1/auth/me"
		if method == "POST" {
			path = "/api/v1/auth/logout"
		}
		if w := request(handler, method, path, first.AccessToken, ""); w.Code != 401 {
			t.Fatal("revoked token remained valid")
		}
	}
	if w := request(handler, "GET", "/api/v1/auth/me", second.AccessToken, ""); w.Code != 200 {
		t.Fatal("logout revoked a separate session")
	}
}

func TestInvalidLoginRequests(t *testing.T) {
	_, _, handler := fixture(t)
	for _, body := range []string{`null`, `[]`, `{}`, `{"username":"analyst"}`, `{"username":"analyst","password":""}`, `{"username":"analyst","password":"x","extra":true}`, `{"username":"analyst","password":"x"} {}`, strings.Repeat("x", 17000)} {
		w := request(handler, "POST", "/api/v1/auth/login", "", body)
		if w.Code != 400 {
			t.Fatalf("invalid body accepted (%d)", w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("non-JSON input accepted")
	}
	if w := request(handler, "GET", "/api/v1/auth/login", "", ""); w.Code != 405 {
		t.Fatal("login accepted GET")
	}
}

func TestBadCredentialsAndHeaderValidation(t *testing.T) {
	_, _, handler := fixture(t)
	for _, username := range []string{"analyst", "nonexistent"} {
		w := request(handler, "POST", "/api/v1/auth/login", "", `{"username":"`+username+`","password":"wrong"}`)
		assertUnauthorized(t, w)
	}
	for _, header := range []string{"", "Basic abc", "Bearer", "Bearer invalid", "Bearer one two", "Bearer " + strings.Repeat("a", 257)} {
		r := httptest.NewRequest("GET", "/api/v1/auth/me?access_token=ignored", nil)
		r.Header.Set("Authorization", header)
		r.AddCookie(&http.Cookie{Name: "session", Value: "ignored"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		assertUnauthorized(t, w)
	}
	r := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	r.Header.Add("Authorization", "Bearer one")
	r.Header.Add("Authorization", "Bearer two")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	assertUnauthorized(t, w)
	if w := request(handler, "GET", "/api/v1/future-route", "", ""); w.Code != 401 {
		t.Fatal("future API routes did not inherit authentication")
	}
	if w := request(handler, "GET", "/api/v1/auth/config", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"enabled":false`) {
		t.Fatal("public auth configuration is incorrect")
	}
}

func assertUnauthorized(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != 401 || w.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("expected Bearer challenge, got %d", w.Code)
	}
	var body struct {
		Error struct {
			Code, RequestID string
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "UNAUTHORIZED" || body.Error.RequestID == "" || body.Error.RequestID != w.Header().Get("X-Request-ID") {
		t.Fatal("error does not match the contract or request identifier")
	}
}

func TestExpiredAndChangedPasswordSessions(t *testing.T) {
	u, store, handler := fixture(t)
	session := login(t, handler)
	u.account.Version++
	assertUnauthorized(t, request(handler, "GET", "/api/v1/auth/me", session.AccessToken, ""))
	session = login(t, handler)
	hash := sha256.Sum256([]byte(session.AccessToken))
	key := base64.RawURLEncoding.EncodeToString(hash[:])
	data, found, err := store.Find(key)
	if err != nil || !found {
		t.Fatal("hashed session was not persisted")
	}
	if bytes.Contains(data, []byte(session.AccessToken)) {
		t.Fatal("session payload contains the raw token")
	}
	if err := store.Commit(key, data, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertUnauthorized(t, request(handler, "GET", "/api/v1/auth/me", session.AccessToken, ""))
	// Recreating the service preserves valid sessions in its shared store.
	session = login(t, handler)
	restarted := makeHandler(t, u, store)
	if w := request(restarted, "GET", "/api/v1/auth/me", session.AccessToken, ""); w.Code != 200 {
		t.Fatal("service restart invalidated a persisted session")
	}
	u.err = auth.ErrUserNotFound
	assertUnauthorized(t, request(handler, "GET", "/api/v1/auth/me", session.AccessToken, ""))
}

func TestLoginRateLimit(t *testing.T) {
	_, _, handler := fixture(t)
	for i := 0; i < 10; i++ {
		if w := request(handler, "POST", "/api/v1/auth/login", "", `{"username":"analyst","password":"wrong"}`); w.Code != 401 {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	w := request(handler, "POST", "/api/v1/auth/login", "", `{"username":"analyst","password":"wrong"}`)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), "RATE_LIMITED") {
		t.Fatal("login attempts were not limited with contract response")
	}
}

type failingStore struct{ scs.Store }

func (f failingStore) Commit(string, []byte, time.Time) error { return errors.New("storage-secret") }
func (f failingStore) Find(string) ([]byte, bool, error) {
	return nil, false, errors.New("storage-secret")
}
func (f failingStore) Delete(string) error { return errors.New("storage-secret") }

func TestStorageFailureFailsClosed(t *testing.T) {
	u, store, _ := fixture(t)
	handler := makeHandler(t, u, failingStore{store})
	for _, test := range []struct {
		response *httptest.ResponseRecorder
		status   int
	}{
		{request(handler, "POST", "/api/v1/auth/login", "", `{"username":"analyst","password":"`+password+`"}`), 503},
		{request(handler, "GET", "/api/v1/auth/me", "unknown", ""), 500},
	} {
		if test.response.Code != test.status || strings.Contains(test.response.Body.String(), "storage-secret") {
			t.Fatal("storage failure was accepted or leaked internals")
		}
	}
}

type deleteFailStore struct{ scs.Store }

func (f deleteFailStore) Delete(string) error { return errors.New("storage-secret") }

func TestFailedRevocationDoesNotClaimSuccessfulLogout(t *testing.T) {
	u, store, _ := fixture(t)
	handler := makeHandler(t, u, deleteFailStore{store})
	session := login(t, handler)
	w := request(handler, "POST", "/api/v1/auth/logout", session.AccessToken, "")
	if w.Code != 503 || strings.Contains(w.Body.String(), "storage-secret") {
		t.Fatal("failed session revocation was reported as success or leaked internals")
	}
}
