package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const AezaIssuer = "https://my.aeza.net"

var (
	ErrOIDCDisabled  = errors.New("aeza oauth is not configured")
	ErrOIDCForbidden = errors.New("aeza account is not allowed")
)

type AezaConfig struct {
	ClientID      string
	ClientSecret  string
	AuthorizeURL  string
	TokenURL      string
	APIURL        string
	RedirectURI   string
	ResponseType  string
	Scope         string
	AllowedEmails map[string]bool
}

func AezaConfigFromEnv() AezaConfig {
	env := func(name, fallback string) string {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
		return fallback
	}
	allowed := map[string]bool{}
	for _, email := range strings.Split(os.Getenv("AEZA_OAUTH_ALLOWED_EMAILS"), ",") {
		if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
			allowed[email] = true
		}
	}
	return AezaConfig{
		ClientID:      env("AEZA_OAUTH_CLIENT_ID", ""),
		ClientSecret:  env("AEZA_OAUTH_CLIENT_SECRET", ""),
		AuthorizeURL:  env("AEZA_OAUTH_AUTHORIZE_URL", AezaIssuer+"/oauth/authorize"),
		TokenURL:      env("AEZA_OAUTH_TOKEN_URL", ""),
		APIURL:        strings.TrimRight(env("AEZA_API_URL", AezaIssuer+"/api/v2"), "/"),
		RedirectURI:   env("AEZA_OAUTH_REDIRECT_URI", ""),
		ResponseType:  env("AEZA_OAUTH_RESPONSE_TYPE", "token"),
		Scope:         env("AEZA_OAUTH_SCOPE", "user:read"),
		AllowedEmails: allowed,
	}
}

func (c AezaConfig) Enabled() bool {
	return c.ClientID != "" && c.RedirectURI != ""
}

type Aeza struct {
	config  AezaConfig
	client  *http.Client
	users   *Postgres
	service *Service
}

func NewAeza(config AezaConfig, users *Postgres, service *Service) *Aeza {
	return &Aeza{config: config, client: &http.Client{Timeout: 10 * time.Second}, users: users, service: service}
}

func (a *Aeza) Login(ctx context.Context, code, accessToken string) (LoginResponse, error) {
	if !a.config.Enabled() {
		return LoginResponse{}, ErrOIDCDisabled
	}
	if accessToken == "" && code != "" {
		token, err := a.exchange(ctx, code)
		if err != nil {
			return LoginResponse{}, err
		}
		accessToken = token
	}
	if accessToken == "" || len(accessToken) > 4096 {
		return LoginResponse{}, ErrUnauthorized
	}
	profile, err := a.profile(ctx, accessToken)
	if err != nil {
		return LoginResponse{}, err
	}
	email := strings.ToLower(strings.TrimSpace(profile.Email))
	if profile.Status != "active" || email == "" || !a.config.AllowedEmails[email] {
		return LoginResponse{}, ErrOIDCForbidden
	}
	account, err := a.users.LinkOIDC(ctx, AezaIssuer, strconv.FormatInt(profile.ID, 10), email)
	if errors.Is(err, ErrUserNotFound) {
		return LoginResponse{}, ErrOIDCForbidden
	}
	if err != nil {
		return LoginResponse{}, fmt.Errorf("link aeza account: %w", err)
	}
	return a.service.startSession(ctx, account)
}

type aezaProfile struct {
	ID     int64  `json:"id"`
	Email  string `json:"email"`
	Status string `json:"status"`
}

func (a *Aeza) profile(ctx context.Context, token string) (aezaProfile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.config.APIURL+"/accounts/me", nil)
	if err != nil {
		return aezaProfile{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return aezaProfile{}, fmt.Errorf("request aeza profile: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return aezaProfile{}, ErrUnauthorized
	}
	if response.StatusCode != http.StatusOK {
		return aezaProfile{}, fmt.Errorf("aeza profile: HTTP %d", response.StatusCode)
	}
	var body struct {
		aezaProfile
		Data *aezaProfile `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body); err != nil {
		return aezaProfile{}, fmt.Errorf("decode aeza profile: %w", err)
	}
	if body.Data != nil {
		return *body.Data, nil
	}
	return body.aezaProfile, nil
}

func (a *Aeza) exchange(ctx context.Context, code string) (string, error) {
	if a.config.TokenURL == "" {
		return "", ErrUnauthorized
	}
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {a.config.RedirectURI},
		"client_id":    {a.config.ClientID},
	}
	if a.config.ClientSecret != "" {
		form.Set("client_secret", a.config.ClientSecret)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("exchange aeza code: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 && response.StatusCode < 500 {
		return "", ErrUnauthorized
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("exchange aeza code: HTTP %d", response.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("decode aeza token: %w", err)
	}
	return body.AccessToken, nil
}

type OIDCConfig struct {
	Enabled               bool     `json:"enabled"`
	Issuer                string   `json:"issuer,omitempty"`
	ClientID              string   `json:"clientId,omitempty"`
	RedirectURI           string   `json:"redirectUri,omitempty"`
	Scopes                []string `json:"scopes,omitempty"`
	AuthorizationEndpoint string   `json:"authorizationEndpoint,omitempty"`
	ResponseType          string   `json:"responseType,omitempty"`
	Provider              string   `json:"provider,omitempty"`
}

func (a *Aeza) PublicConfig() OIDCConfig {
	if !a.config.Enabled() {
		return OIDCConfig{}
	}
	return OIDCConfig{
		Enabled:               true,
		Issuer:                AezaIssuer,
		ClientID:              a.config.ClientID,
		RedirectURI:           a.config.RedirectURI,
		Scopes:                strings.Fields(a.config.Scope),
		AuthorizationEndpoint: a.config.AuthorizeURL,
		ResponseType:          a.config.ResponseType,
		Provider:              "aeza",
	}
}
