package risk

import (
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

type Input struct {
	Account      entity.Account
	Services     []entity.Service
	Transactions []entity.Transaction
}

type Engine interface {
	Calculate(input Input, settings entity.ModelSettings, asOf time.Time) (entity.CustomerDetail, error)
}
