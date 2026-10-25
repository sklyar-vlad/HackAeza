package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	Server   Server
	Database Database
	Redis    Redis
	Admin    Admin
}

type Admin struct {
	Username string
	Password string
}

type Redis struct{ URL string }

type Database struct {
	URL            string
	MaxConnections int32
	ConnectTimeout time.Duration
}

type Server struct {
	Port              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

func NewConfig() Config {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8081"
	}
	return Config{Database: Database{
		URL:            os.Getenv("POSTGRES_URL"),
		MaxConnections: 10,
		ConnectTimeout: 5 * time.Second,
	}, Redis: Redis{URL: os.Getenv("REDIS_URL")}, Admin: Admin{
		Username: os.Getenv("ADMIN_USERNAME"),
		Password: os.Getenv("ADMIN_PASSWORD"),
	}, Server: Server{
		Port:              port,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       time.Minute,
		ShutdownTimeout:   5 * time.Second,
	}}
}

func SessionLifetime() (time.Duration, error) {
	lifetime := 8 * time.Hour
	if text := os.Getenv("AUTH_SESSION_TTL"); text != "" {
		var err error
		lifetime, err = time.ParseDuration(text)
		if err != nil || lifetime < time.Second || lifetime > 30*24*time.Hour {
			return 0, errors.New("AUTH_SESSION_TTL must be between 1s and 720h")
		}
	}
	return lifetime, nil
}
