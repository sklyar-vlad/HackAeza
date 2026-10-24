package source

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/handler"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

const MaxSnapshotBytes = 100 << 20

type Handler struct {
	service      *Service
	authenticate func(context.Context, string) (string, error)
}

func NewHandler(service *Service, authenticate func(context.Context, string) (string, error)) *Handler {
	return &Handler{service: service, authenticate: authenticate}
}

func (h *Handler) Register(router gin.IRouter) {
	router.PUT("/source/snapshot", h.importSnapshot)
	router.GET("/source/status", h.status)
}

func (h *Handler) importSnapshot(c *gin.Context) {
	actorID, ok := h.actor(c)
	if !ok {
		return
	}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) > 1 || (len(keys) == 1 && keys[0] == "") {
		handler.BadRequest(c, []entity.FieldError{{Path: "Idempotency-Key", Message: "Ожидается один непустой ключ."}})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxSnapshotBytes)
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			handler.Error(c, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Размер снимка превышает 100 МиБ.")
		} else {
			handler.BadRequest(c, nil)
		}
		return
	}
	accepted, err := h.service.Import(c.Request.Context(), payload, jobs.Submission{
		ActorID: actorID, RequestID: c.GetString("requestId"), IdempotencyKey: c.GetHeader("Idempotency-Key"),
	})
	if respondError(c, err) {
		return
	}
	c.Header("Location", accepted.StatusURL)
	c.Header("Retry-After", "2")
	c.JSON(http.StatusAccepted, accepted)
}

func (h *Handler) status(c *gin.Context) {
	if _, ok := h.actor(c); !ok {
		return
	}
	status, err := h.service.Status(c.Request.Context())
	if !respondError(c, err) {
		c.JSON(http.StatusOK, status)
	}
}

func (h *Handler) actor(c *gin.Context) (string, bool) {
	c.Header("Cache-Control", "no-store")
	authorization := strings.Fields(c.GetHeader("Authorization"))
	if len(authorization) != 2 || !strings.EqualFold(authorization[0], "Bearer") {
		respondError(c, jobs.ErrUnauthorized)
		return "", false
	}
	actorID, err := h.authenticate(c.Request.Context(), authorization[1])
	if respondError(c, err) {
		return "", false
	}
	return actorID, true
}

func respondError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	var validation *ValidationError
	var connection *pgconn.ConnectError
	switch {
	case errors.Is(err, jobs.ErrUnauthorized):
		c.Header("WWW-Authenticate", "Bearer")
		handler.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Необходима действующая сессия.")
	case errors.Is(err, ErrInvalidJSON), errors.Is(err, jobs.ErrInvalidInput):
		handler.BadRequest(c, nil)
	case errors.As(err, &validation):
		handler.Error(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Снимок содержит некорректные данные.", validation.Details...)
	case errors.Is(err, entity.ErrIdempotencyConflict):
		handler.Error(c, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Ключ уже использован для другого запроса.")
	case errors.Is(err, entity.ErrJobInProgress):
		handler.Error(c, http.StatusConflict, "JOB_IN_PROGRESS", "Уже выполняется изменение данных или модели.")
	case errors.As(err, &connection), errors.Is(err, context.DeadlineExceeded):
		_ = c.Error(err)
		handler.Error(c, http.StatusServiceUnavailable, "UNAVAILABLE", "Сервис временно недоступен.")
	default:
		handler.Internal(c, err)
	}
	return true
}
