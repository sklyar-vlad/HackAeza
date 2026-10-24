package analytics

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
}

func (h *Handler) summary(c *gin.Context) {
	summary, err := h.service.Summary(c.Request.Context())
	if err != nil {
		handler.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}
