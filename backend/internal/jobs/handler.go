package jobs

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"go.uber.org/zap"
)

type Handler struct {
	reader       Reader
	authenticate func(context.Context, string) (string, error)
	logger       *zap.Logger
}

func NewHandler(reader Reader, authenticate func(context.Context, string) (string, error), logger *zap.Logger) *Handler {
	return &Handler{reader: reader, authenticate: authenticate, logger: logger}
}

func (h *Handler) Get(c *gin.Context) {
	requestID := rand.Text()
	c.Header("X-Request-ID", requestID)
	c.Header("Cache-Control", "no-store")
	authorization := strings.Fields(c.GetHeader("Authorization"))
	if len(authorization) != 2 || !strings.EqualFold(authorization[0], "Bearer") {
		h.respond(c, requestID, http.StatusUnauthorized, "UNAUTHORIZED", "Необходима авторизация.")
		return
	}
	actorID, err := h.authenticate(c.Request.Context(), authorization[1])
	if errors.Is(err, ErrUnauthorized) {
		h.respond(c, requestID, http.StatusUnauthorized, "UNAUTHORIZED", "Сессия недействительна или истекла.")
		return
	}
	if err != nil {
		h.internalError(c, requestID, err)
		return
	}
	id := entity.JobID(c.Param("jobId"))
	if !validUUID(string(id)) {
		h.respond(c, requestID, http.StatusBadRequest, "BAD_REQUEST", "jobId должен быть UUID.")
		return
	}
	job, err := h.reader.Get(c.Request.Context(), actorID, id)
	if errors.Is(err, entity.ErrNotFound) {
		h.respond(c, requestID, http.StatusNotFound, "NOT_FOUND", "Задание не найдено или недоступно.")
		return
	}
	if err != nil {
		h.internalError(c, requestID, err)
		return
	}
	if job.Status == entity.JobQueued || job.Status == entity.JobRunning {
		c.Header("Retry-After", "2")
	}
	c.JSON(http.StatusOK, job)
}

func (h *Handler) internalError(c *gin.Context, requestID string, err error) {
	h.logger.Error("read job status", zap.String("requestId", requestID), zap.Error(err))
	var connection *pgconn.ConnectError
	if errors.As(err, &connection) || errors.Is(err, context.DeadlineExceeded) {
		h.respond(c, requestID, http.StatusServiceUnavailable, "UNAVAILABLE", "Сервис временно недоступен.")
		return
	}
	h.respond(c, requestID, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось получить состояние задания.")
}

func (h *Handler) respond(c *gin.Context, requestID string, status int, code, message string) {
	if status == http.StatusUnauthorized {
		c.Header("WWW-Authenticate", "Bearer")
	}
	c.AbortWithStatusJSON(status, gin.H{"error": entity.JobError{Code: code, Message: message, RequestID: requestID}})
}
