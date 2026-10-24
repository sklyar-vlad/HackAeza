package config_test

import (
	"testing"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/config"
)

func TestSessionLifetime(t *testing.T) {
	t.Setenv("AUTH_SESSION_TTL", "")
	lifetime, err := config.SessionLifetime()
	if err != nil || lifetime != 8*time.Hour {
		t.Fatal("incorrect default session lifetime")
	}
	for _, text := range []string{"garbage", "0", "-1h", "1ms", "721h"} {
		t.Setenv("AUTH_SESSION_TTL", text)
		if _, err := config.SessionLifetime(); err == nil {
			t.Fatalf("accepted invalid lifetime %q", text)
		}
	}
	t.Setenv("AUTH_SESSION_TTL", "15m")
	lifetime, err = config.SessionLifetime()
	if err != nil || lifetime != 15*time.Minute {
		t.Fatal("configured lifetime was ignored")
	}
}
