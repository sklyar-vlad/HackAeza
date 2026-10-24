package model

import (
	"crypto/sha256"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/sklyar-vlad/HackAeza/backend/internal/auth"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/handler"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(router gin.IRouter) {
	group := router.Group("/model", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if user, ok := auth.CurrentUser(c.Request.Context()); ok && user.ID != "" {
			c.Next()
			return
		}
		handler.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действующая сессия.")
	})
	group.GET("/settings", h.settings)
	group.GET("/features", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"items": Features()}) })
	group.PATCH("/settings", h.update)
}

func (h *Handler) settings(c *gin.Context) {
	settings, err := h.service.Settings(c.Request.Context())
	if err != nil {
		h.failure(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

var idempotencyKey = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func (h *Handler) update(c *gin.Context) {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		handler.BadRequest(c, nil)
		return
	}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) > 1 || len(keys) == 1 && !idempotencyKey.MatchString(keys[0]) {
		handler.BadRequest(c, []entity.FieldError{{Path: "Idempotency-Key", Message: "Недопустимый ключ идемпотентности."}})
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		handler.BadRequest(c, nil)
		return
	}
	patch, err := DecodePatch(data)
	if err != nil {
		h.failure(c, err)
		return
	}
	user, _ := auth.CurrentUser(c.Request.Context())
	accepted, err := h.service.Update(c.Request.Context(), patch, jobs.Submission{
		ActorID: user.ID, RequestID: c.GetString("requestId"), IdempotencyKey: c.GetHeader("Idempotency-Key"), PayloadHash: sha256.Sum256(data),
	})
	if err != nil {
		h.failure(c, err)
		return
	}
	url := "/api/v1/jobs/" + string(accepted.JobID)
	c.Header("Location", url)
	c.Header("Retry-After", "2")
	c.JSON(http.StatusAccepted, entity.JobAccepted{JobID: accepted.JobID, StatusURL: url, CreatedAt: accepted.CreatedAt, Calculation: accepted.Calculation})
}

func (h *Handler) failure(c *gin.Context, err error) {
	var validation *ValidationError
	switch {
	case errors.As(err, &validation):
		handler.Error(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Некорректные настройки модели.", validation.Details...)
	case errors.Is(err, ErrInvalidJSON):
		handler.BadRequest(c, nil)
	case errors.Is(err, entity.ErrVersionConflict):
		handler.Error(c, http.StatusConflict, "VERSION_CONFLICT", "Опубликованная версия настроек изменилась.")
	case errors.Is(err, entity.ErrJobInProgress):
		handler.Error(c, http.StatusConflict, "JOB_IN_PROGRESS", "Уже выполняется другое задание.")
	case errors.Is(err, entity.ErrIdempotencyConflict):
		handler.Error(c, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Ключ уже использован для другого запроса.")
	default:
		handler.Internal(c, err)
	}
}
