package config_test

import (
	"testing"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/config"
)

func TestSessionLifetime(t *testing.T) {
	t.Setenv("AUTH_SESSION_TTL", "")
	cfg, err := config.NewConfig()
	if err != nil || cfg.SessionLifetime != 8*time.Hour {
		t.Fatal("incorrect default session lifetime")
	}
	for _, text := range []string{"garbage", "0", "-1h", "1ms", "721h"} {
		t.Setenv("AUTH_SESSION_TTL", text)
		if _, err := config.NewConfig(); err == nil {
			t.Fatalf("accepted invalid lifetime %q", text)
		}
	}
	t.Setenv("AUTH_SESSION_TTL", "15m")
	cfg, err = config.NewConfig()
	if err != nil || cfg.SessionLifetime != 15*time.Minute {
		t.Fatal("configured lifetime was ignored")
	}
}
