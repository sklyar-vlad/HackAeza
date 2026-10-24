package analytics

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/handler"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(router gin.IRouter) {
	router.GET("/analytics/summary", h.summary)
	router.GET("/analytics/churn", h.churn)
}

func (h *Handler) churn(c *gin.Context) {
	from, fromErr := time.Parse(time.DateOnly, c.Query("from"))
	to, toErr := time.Parse(time.DateOnly, c.Query("to"))
	var details []entity.FieldError
	if fromErr != nil {
		details = append(details, entity.FieldError{Path: "from", Message: "Ожидается дата YYYY-MM-DD."})
	}
	if toErr != nil {
		details = append(details, entity.FieldError{Path: "to", Message: "Ожидается дата YYYY-MM-DD."})
	}
	if fromErr == nil && toErr == nil && from.After(to) {
		details = append(details, entity.FieldError{Path: "from", Message: "Должна быть не позже to."})
	}
	if len(details) > 0 {
		handler.BadRequest(c, details)
		return
	}
	series, err := h.service.Churn(c.Request.Context(), from, to)
	if err != nil {
		handler.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, series)
}

func (h *Handler) summary(c *gin.Context) {
	summary, err := h.service.Summary(c.Request.Context())
	if err != nil {
		handler.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}
