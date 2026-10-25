package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"go.uber.org/zap"
)

const heartbeat = 25 * time.Second

type Update struct {
	Type        string             `json:"type"`
	Calculation entity.Calculation `json:"calculation"`
	Changed     []string           `json:"changed"`
}

type Hub struct {
	pool    *pgxpool.Pool
	logger  *zap.Logger
	mu      sync.Mutex
	current Update
	clients map[chan Update]struct{}
}

func NewHub(pool *pgxpool.Pool, logger *zap.Logger) *Hub {
	return &Hub{pool: pool, logger: logger, clients: map[chan Update]struct{}{}}
}

func (h *Hub) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := h.listen(ctx); err != nil && ctx.Err() == nil {
			h.logger.Warn("calculation events listener stopped", zap.Error(err))
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}

func (h *Hub) listen(ctx context.Context) error {
	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN calculation_published"); err != nil {
		return err
	}
	if err := h.refresh(ctx); err != nil {
		return err
	}
	for {
		waitCtx, cancel := context.WithTimeout(ctx, time.Minute)
		_, err := conn.Conn().WaitForNotification(waitCtx)
		cancel()
		if ctx.Err() != nil {
			return nil
		}
		if err != nil && waitCtx.Err() == nil {
			return err
		}
		if err := h.refresh(ctx); err != nil {
			return err
		}
	}
}

func (h *Hub) refresh(ctx context.Context) error {
	next, err := h.read(ctx)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	previous := h.current.Calculation
	if previous.Version == next.Calculation.Version && h.current.Type != "" {
		return nil
	}
	next.Changed = []string{}
	if h.current.Type != "" {
		if previous.SourceVersion != next.Calculation.SourceVersion {
			next.Changed = append(next.Changed, "source")
		}
		if previous.ModelVersion != next.Calculation.ModelVersion {
			next.Changed = append(next.Changed, "model")
		}
		if len(next.Changed) == 0 {
			next.Changed = append(next.Changed, "time")
		}
	}
	h.current = next
	for client := range h.clients {
		select {
		case client <- next:
		default:
		}
	}
	return nil
}

func (h *Hub) read(ctx context.Context) (Update, error) {
	update := Update{Type: "calculation.updated", Changed: []string{}}
	c := &update.Calculation
	err := h.pool.QueryRow(ctx, `SELECT coalesce(c.version, 0), coalesce(c.source_version, 0), p.active_model_version, c.calculated_at
		FROM published_state p LEFT JOIN calculation_versions c ON c.version = p.active_calculation_version
		WHERE p.id = 1`).Scan(&c.Version, &c.SourceVersion, &c.ModelVersion, &c.CalculatedAt)
	if err != nil {
		return Update{}, fmt.Errorf("read published calculation: %w", err)
	}
	if c.CalculatedAt != nil {
		*c.CalculatedAt = c.CalculatedAt.UTC()
	}
	return update, nil
}

func (h *Hub) subscribe() (chan Update, Update) {
	h.mu.Lock()
	defer h.mu.Unlock()
	client := make(chan Update, 4)
	h.clients[client] = struct{}{}
	return client, h.current
}

func (h *Hub) unsubscribe(client chan Update) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, client)
}

type Validate func(ctx context.Context, token string) error

func (h *Hub) Handler(validate Validate) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer"))
		client, current := h.subscribe()
		defer h.unsubscribe(client)
		if current.Type == "" {
			update, err := h.read(c.Request.Context())
			if err != nil {
				_ = c.Error(err)
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
			current = update
		}
		_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
		header := c.Writer.Header()
		header.Set("Content-Type", "text/event-stream")
		header.Set("Cache-Control", "no-cache")
		header.Set("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
		if !send(c, current) {
			return
		}
		ticker := time.NewTicker(heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case update := <-client:
				if !send(c, update) {
					return
				}
			case <-ticker.C:
				if validate(c.Request.Context(), token) != nil {
					return
				}
				if _, err := c.Writer.WriteString(": heartbeat\n\n"); err != nil {
					return
				}
				c.Writer.Flush()
			}
		}
	}
}

func send(c *gin.Context, update Update) bool {
	data, err := json.Marshal(update)
	if err != nil {
		return false
	}
	if _, err := fmt.Fprintf(c.Writer, "event: calculation.updated\nid: %d\ndata: %s\n\n", update.Calculation.Version, data); err != nil {
		return false
	}
	c.Writer.Flush()
	return true
}
