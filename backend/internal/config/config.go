package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	Server          Server
	DatabaseURL     string
	SessionLifetime time.Duration
}

type Server struct {
	Port              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// NewConfig keeps the existing HTTP_PORT setting and the Flagman server timeout defaults.
func NewConfig() (Config, error) {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8081"
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://hackaeza@localhost:5432/hackaeza?sslmode=disable"
	}
	lifetime := 8 * time.Hour
	if text := os.Getenv("AUTH_SESSION_TTL"); text != "" {
		var err error
		lifetime, err = time.ParseDuration(text)
		if err != nil || lifetime < time.Second || lifetime > 30*24*time.Hour {
			return Config{}, errors.New("AUTH_SESSION_TTL must be between 1s and 720h")
		}
	}
	return Config{DatabaseURL: databaseURL, SessionLifetime: lifetime, Server: Server{
		Port:              port,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       time.Minute,
		ShutdownTimeout:   5 * time.Second,
	}}, nil
}
