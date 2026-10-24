package config

import (
	"os"
	"time"
)

type Config struct {
	Server Server
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
	return Config{Server: Server{
		Port:              port,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       time.Minute,
		ShutdownTimeout:   5 * time.Second,
	}}
}
