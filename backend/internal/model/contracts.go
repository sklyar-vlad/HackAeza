package model

import (
	"context"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

type Candidate struct {
	BaseVersion int64
	Settings    entity.ModelSettings
}

type Repository interface {
	jobs.Receipts

	Published(ctx context.Context) (entity.ModelSettingsResponse, error)
	AcceptSettings(ctx context.Context, candidate Candidate, submission jobs.Submission) (jobs.Accepted, error)
}
