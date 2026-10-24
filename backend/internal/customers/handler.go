package customers

import (
	"errors"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/handler"
)

var riskLevels = []string{"low", "medium", "high", "unknown"}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(router gin.IRouter) {
	router.GET("/customers", h.list)
	router.GET("/customers/:id", h.get)
	router.GET("/customers/:id/transactions", h.transactions)
}

func (h *Handler) list(c *gin.Context) {
	var p params
	query := Query{
		Page:      p.integer(c, "page", 1, 1, math.MaxInt64/100),
		PageSize:  p.integer(c, "pageSize", 20, 1, 100),
		Search:    c.Query("search"),
		MinScore:  p.score(c, "minScore"),
		MaxScore:  p.score(c, "maxScore"),
		SortBy:    p.oneOf(c, "sortBy", "score", "id", "email", "balance", "totalReplenished", "score", "potentialLoss"),
		SortOrder: p.oneOf(c, "sortOrder", "desc", "asc", "desc"),
	}
	if utf8.RuneCountInString(query.Search) > 256 {
		p.fail("search", "Не длиннее 256 символов.")
	}
	if levels := c.Query("riskLevel"); levels != "" {
		for _, level := range strings.Split(levels, ",") {
			if !slices.Contains(riskLevels, level) {
				p.fail("riskLevel", "Допустимы low, medium, high, unknown.")
				break
			}
			query.RiskLevels = append(query.RiskLevels, level)
		}
	}
	if query.MinScore != nil && query.MaxScore != nil && *query.MinScore > *query.MaxScore {
		p.fail("minScore", "Должен быть не больше maxScore.")
	}
	if p.respond(c) {
		return
	}
	page, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		handler.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *Handler) get(c *gin.Context) {
	var p params
	id := p.id(c)
	if p.respond(c) {
		return
	}
	detail, err := h.service.Get(c.Request.Context(), id)
	if !respondError(c, err) {
		c.JSON(http.StatusOK, detail)
	}
}

func (h *Handler) transactions(c *gin.Context) {
	var p params
	id := p.id(c)
	query := TransactionQuery{
		Page:      p.integer(c, "page", 1, 1, math.MaxInt64/100),
		PageSize:  p.integer(c, "pageSize", 20, 1, 100),
		Status:    p.oneOf(c, "status", "", "performed", "not_performed"),
		Type:      p.oneOf(c, "type", "", "order", "prolong"),
		SortOrder: p.oneOf(c, "sortOrder", "desc", "asc", "desc"),
	}
	if p.respond(c) {
		return
	}
	page, err := h.service.Transactions(c.Request.Context(), id, query)
	if !respondError(c, err) {
		c.JSON(http.StatusOK, page)
	}
}

func respondError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, entity.ErrNotFound):
		handler.NotFound(c)
	default:
		handler.Internal(c, err)
	}
	return true
}

type params []entity.FieldError

func (p *params) fail(path, message string) {
	*p = append(*p, entity.FieldError{Path: path, Message: message})
}

func (p *params) respond(c *gin.Context) bool {
	if len(*p) == 0 {
		return false
	}
	handler.BadRequest(c, *p)
	return true
}

func (p *params) id(c *gin.Context) entity.ID {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		p.fail("id", "Ожидается положительное целое число.")
	}
	return entity.ID(id)
}

func (p *params) integer(c *gin.Context, name string, fallback, low, high int64) int64 {
	text, ok := c.GetQuery(name)
	if !ok {
		return fallback
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < low || value > high {
		p.fail(name, "Ожидается целое число от "+strconv.FormatInt(low, 10)+" до "+strconv.FormatInt(high, 10)+".")
		return fallback
	}
	return value
}

func (p *params) score(c *gin.Context, name string) *float64 {
	text, ok := c.GetQuery(name)
	if !ok {
		return nil
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || value < 0 || value > 100 {
		p.fail(name, "Ожидается число от 0 до 100.")
		return nil
	}
	return &value
}

func (p *params) oneOf(c *gin.Context, name, fallback string, allowed ...string) string {
	text, ok := c.GetQuery(name)
	if !ok {
		return fallback
	}
	if !slices.Contains(allowed, text) {
		p.fail(name, "Допустимы: "+strings.Join(allowed, ", ")+".")
		return fallback
	}
	return text
}
