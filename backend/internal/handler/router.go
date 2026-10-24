package handler

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"go.uber.org/zap"
)

func NewRouter(logger *zap.Logger, pool *pgxpool.Pool) *gin.Engine {
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.Info("http request",
			zap.String("method", c.Request.Method),
			zap.String("route", c.FullPath()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", time.Since(started)),
		)
	}, gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		logger.Error("http panic recovered")
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	repository := jobs.NewRepository(pool)
	jobHandler := jobs.NewHandler(repository, repository.SessionActor, logger)
	router.GET("/api/v1/jobs/:jobId", jobHandler.Get)
	router.GET("/healthz", Healthcheck)
	return router
}
