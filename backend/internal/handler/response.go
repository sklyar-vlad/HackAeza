package handler

import (
	"crypto/rand"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

const requestIDKey = "requestId"

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func RequestID(c *gin.Context) {
	id := c.GetHeader("X-Request-Id")
	if !requestIDPattern.MatchString(id) {
		id = rand.Text()
	}
	c.Set(requestIDKey, id)
	c.Header("X-Request-Id", id)
	c.Next()
}

func Error(c *gin.Context, status int, code, message string, details ...entity.FieldError) {
	c.AbortWithStatusJSON(status, gin.H{"error": entity.JobError{
		Code:      code,
		Message:   message,
		RequestID: c.GetString(requestIDKey),
		Details:   details,
	}})
}

func BadRequest(c *gin.Context, details []entity.FieldError) {
	Error(c, http.StatusBadRequest, "BAD_REQUEST", "Некорректные параметры запроса.", details...)
}

func NotFound(c *gin.Context) {
	Error(c, http.StatusNotFound, "NOT_FOUND", "Ресурс не найден.")
}

func Internal(c *gin.Context, err error) {
	_ = c.Error(err)
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Внутренняя ошибка сервера.")
}
