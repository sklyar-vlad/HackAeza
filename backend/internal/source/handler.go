package source

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/handler"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

const (
	MaxSnapshotBytes = 1 << 30
	snapshotTimeout  = 15 * time.Minute
)

type Handler struct {
	service      *Service
	uploads      *Uploads
	authenticate func(context.Context, string) (string, error)
}

var uploadID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func NewHandler(service *Service, uploads *Uploads, authenticate func(context.Context, string) (string, error)) *Handler {
	return &Handler{service: service, uploads: uploads, authenticate: authenticate}
}

func (h *Handler) Register(router gin.IRouter) {
	router.PUT("/source/snapshot", h.importSnapshot)
	router.GET("/source/status", h.status)
	router.POST("/source/uploads", h.startUpload)
	router.GET("/source/uploads/:id", h.upload)
	router.PUT("/source/uploads/:id/chunks/:offset", h.putChunk)
	router.POST("/source/uploads/:id/complete", h.completeUpload)
}

func (h *Handler) startUpload(c *gin.Context) {
	actorID, ok := h.actor(c)
	if !ok {
		return
	}
	var body struct {
		TotalBytes int64 `json:"totalBytes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<10)).Decode(&body); err != nil {
		handler.BadRequest(c, []entity.FieldError{{Path: "totalBytes", Message: "Ожидается размер файла в байтах."}})
		return
	}
	if body.TotalBytes > MaxSnapshotBytes {
		tooLarge(c)
		return
	}
	upload, err := h.uploads.Start(c.Request.Context(), actorID, body.TotalBytes)
	if !respondError(c, err) {
		c.Header("Location", "/api/v1/source/uploads/"+upload.ID)
		c.JSON(http.StatusCreated, upload)
	}
}

func (h *Handler) upload(c *gin.Context) {
	actorID, id, ok := h.uploadTarget(c)
	if !ok {
		return
	}
	upload, err := h.uploads.Get(c.Request.Context(), actorID, id)
	if !respondError(c, err) {
		c.JSON(http.StatusOK, upload)
	}
}

func (h *Handler) putChunk(c *gin.Context) {
	actorID, id, ok := h.uploadTarget(c)
	if !ok {
		return
	}
	offset, err := strconv.ParseInt(c.Param("offset"), 10, 64)
	if err != nil || offset < 0 {
		handler.BadRequest(c, []entity.FieldError{{Path: "offset", Message: "Ожидается неотрицательное смещение в байтах."}})
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, ChunkBytes))
	if err != nil {
		handler.Error(c, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Часть файла больше 8 МиБ.")
		return
	}
	upload, err := h.uploads.Append(c.Request.Context(), actorID, id, offset, data)
	if !respondError(c, err) {
		c.JSON(http.StatusOK, upload)
	}
}

func (h *Handler) completeUpload(c *gin.Context) {
	actorID, id, ok := h.uploadTarget(c)
	if !ok {
		return
	}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) > 1 || (len(keys) == 1 && keys[0] == "") {
		handler.BadRequest(c, []entity.FieldError{{Path: "Idempotency-Key", Message: "Ожидается один непустой ключ."}})
		return
	}
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(snapshotTimeout))
	accepted, err := h.uploads.Complete(c.Request.Context(), actorID, id, jobs.Submission{
		ActorID: actorID, RequestID: c.GetString("requestId"), IdempotencyKey: c.GetHeader("Idempotency-Key"),
	})
	if respondError(c, err) {
		return
	}
	c.Header("Location", accepted.StatusURL)
	c.Header("Retry-After", "2")
	c.JSON(http.StatusAccepted, accepted)
}

func (h *Handler) uploadTarget(c *gin.Context) (string, string, bool) {
	actorID, ok := h.actor(c)
	if !ok {
		return "", "", false
	}
	id := c.Param("id")
	if !uploadID.MatchString(id) {
		respondError(c, ErrUploadNotFound)
		return "", "", false
	}
	return actorID, id, true
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
	if c.Request.ContentLength > MaxSnapshotBytes {
		tooLarge(c)
		return
	}
	deadline := time.Now().Add(snapshotTimeout)
	controller := http.NewResponseController(c.Writer)
	_ = controller.SetReadDeadline(deadline)
	_ = controller.SetWriteDeadline(deadline)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxSnapshotBytes)
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			tooLarge(c)
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
	case errors.Is(err, ErrUploadNotFound):
		handler.NotFound(c)
	case errors.Is(err, ErrUploadOffset):
		handler.Error(c, http.StatusConflict, "UPLOAD_OFFSET_MISMATCH", "Часть файла пришла не по порядку, продолжите с принятого смещения.")
	case errors.Is(err, ErrUploadIncomplete):
		handler.Error(c, http.StatusConflict, "UPLOAD_INCOMPLETE", "Файл загружен не полностью.")
	case errors.Is(err, ErrUploadState):
		handler.Error(c, http.StatusConflict, "UPLOAD_STATE", "Загрузка уже обрабатывается или завершена.")
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

func tooLarge(c *gin.Context) {
	handler.Error(c, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Размер снимка превышает 1 ГиБ.")
}
