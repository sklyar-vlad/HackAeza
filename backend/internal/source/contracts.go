package source

import (
	"context"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

type Repository interface {
	jobs.Receipts

	AcceptSnapshot(ctx context.Context, snapshot entity.SourceSnapshot, submission jobs.Submission) (jobs.Accepted, error)
	Status(ctx context.Context) (entity.SourceStatus, error)
}
