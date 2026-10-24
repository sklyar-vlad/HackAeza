package source

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Import(ctx context.Context, payload []byte, submission jobs.Submission) (entity.JobAccepted, error) {
	hash, err := snapshotHash(payload)
	if err != nil {
		return entity.JobAccepted{}, err
	}
	submission.PayloadHash = hash
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if previous, err := s.repository.Lookup(ctx, entity.JobSourceSnapshot, submission); err != nil {
		return entity.JobAccepted{}, err
	} else if previous != nil {
		return acceptedResponse(*previous), nil
	}
	snapshot, err := DecodeSnapshot(bytes.NewReader(payload))
	if err != nil {
		return entity.JobAccepted{}, err
	}
	if _, err := ValidateSnapshot(snapshot, time.Now().UTC()); err != nil {
		return entity.JobAccepted{}, err
	}
	accepted, err := s.repository.AcceptSnapshot(ctx, snapshot, submission)
	if err != nil {
		return entity.JobAccepted{}, err
	}
	return acceptedResponse(accepted), nil
}

func (s *Service) Status(ctx context.Context) (entity.SourceStatus, error) {
	return s.repository.Status(ctx)
}

func acceptedResponse(accepted jobs.Accepted) entity.JobAccepted {
	return entity.JobAccepted{JobID: accepted.JobID, StatusURL: "/api/v1/jobs/" + string(accepted.JobID),
		CreatedAt: accepted.CreatedAt, Calculation: accepted.Calculation}
}

func snapshotHash(payload []byte) ([32]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value, extra any
	if err := decoder.Decode(&value); err != nil {
		return [32]byte{}, ErrInvalidJSON
	}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return [32]byte{}, ErrInvalidJSON
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(canonical), nil
}
