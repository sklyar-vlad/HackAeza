package auth

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/httprate"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
	limiter *httprate.RateLimiter
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service, logger, httprate.NewRateLimiter(10, time.Minute, httprate.WithLimitHandler(rateLimited))}
}

// Register leaves login/config public and installs identity handlers on the protected API mux.
func (h *Handler) Register(public, protected *http.ServeMux) {
	login := http.NewServeMux()
	login.Handle("POST /api/v1/auth/login", httprate.LimitBy(60, time.Minute,
		func(r *http.Request) (string, error) {
			// Use the TCP peer; never trust arbitrary forwarded headers from direct clients.
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			return host, nil
		}, httprate.WithLimitHandler(rateLimited))(http.HandlerFunc(h.login)))
	public.Handle("/api/v1/auth/login", login)
	config := http.NewServeMux()
	config.HandleFunc("GET /api/v1/auth/config", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, r, http.StatusOK, struct {
			PasswordEnabled bool `json:"passwordEnabled"`
			OIDC            struct {
				Enabled bool `json:"enabled"`
			} `json:"oidc"`
		}{PasswordEnabled: true})
	})
	public.Handle("/api/v1/auth/config", config)
	protected.HandleFunc("GET /api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		user, _ := CurrentUser(r.Context())
		JSON(w, r, http.StatusOK, user)
	})
	protected.HandleFunc("POST /api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if err := h.service.Logout(r.Context()); err != nil {
			h.unavailable(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Ожидается Content-Type: application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.Username == "" || input.Password == "" || len(input.Username) > 128 {
		Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Укажите логин и пароль в корректном JSON")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Тело должно содержать один JSON-объект")
		return
	}
	if h.limiter.RespondOnLimit(w, r, input.Username) {
		return
	}
	response, err := h.service.Login(r.Context(), input.Username, input.Password)
	if errors.Is(err, ErrUnauthorized) {
		unauthorized(w, r)
		return
	}
	if err != nil {
		h.unavailable(w, r, err)
		return
	}
	JSON(w, r, http.StatusOK, response)
}

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		values := r.Header.Values("Authorization")
		if len(values) != 1 {
			unauthorized(w, r)
			return
		}
		parts := strings.Fields(values[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			unauthorized(w, r)
			return
		}
		ctx, err := h.service.Authenticate(r.Context(), parts[1])
		if errors.Is(err, ErrUnauthorized) {
			unauthorized(w, r)
			return
		}
		if err != nil {
			h.logger.Error("session verification failed", "requestId", w.Header().Get("X-Request-ID"), "error", err)
			Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось проверить сессию")
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Неверный логин или пароль, либо сессия недействительна")
}

func rateLimited(w http.ResponseWriter, r *http.Request) {
	Error(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Слишком много попыток входа, попробуйте позже")
}

func (h *Handler) unavailable(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.Error("authentication storage failed", "requestId", w.Header().Get("X-Request-ID"), "error", err)
	Error(w, r, http.StatusServiceUnavailable, "UNAVAILABLE", "Сервис авторизации временно недоступен")
}

// RequestID provides a server-generated identifier, never accepting one from the caller.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", rand.Text())
		next.ServeHTTP(w, r)
	})
}

func JSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("write JSON response", "requestId", w.Header().Get("X-Request-ID"), "error", err)
	}
}

func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		requestID = rand.Text()
		w.Header().Set("X-Request-ID", requestID)
	}
	type problem struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	}
	JSON(w, r, status, struct {
		Error problem `json:"error"`
	}{Error: problem{code, message, requestID}})
}
