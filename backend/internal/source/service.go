package source

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"sort"
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
	return s.ImportTracked(ctx, payload, submission, nil)
}

func (s *Service) ImportTracked(ctx context.Context, payload []byte, submission jobs.Submission, track func(int64)) (entity.JobAccepted, error) {
	hash, err := snapshotHash(payload)
	if err != nil {
		return entity.JobAccepted{}, err
	}
	submission.PayloadHash = hash
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
	defer cancel()
	if previous, err := s.repository.Lookup(ctx, entity.JobSourceSnapshot, submission); err != nil {
		return entity.JobAccepted{}, err
	} else if previous != nil {
		return acceptedResponse(*previous), nil
	}
	var reader io.Reader = bytes.NewReader(payload)
	if track != nil {
		reader = &countingReader{reader: reader, track: track}
	}
	snapshot, err := DecodeSnapshot(reader)
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
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil || fields == nil {
		if !json.Valid(payload) {
			return [32]byte{}, ErrInvalidJSON
		}
		return sha256.Sum256(bytes.TrimSpace(payload)), nil
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	hash := sha256.New()
	_, _ = hash.Write([]byte("{"))
	for i, key := range keys {
		if i > 0 {
			_, _ = hash.Write([]byte(","))
		}
		encoded, _ := json.Marshal(key)
		_, _ = hash.Write(encoded)
		_, _ = hash.Write([]byte(":"))
		raw := bytes.TrimSpace(fields[key])
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if len(raw) > 0 && raw[0] == '[' {
			if _, err := decoder.Token(); err != nil {
				return [32]byte{}, ErrInvalidJSON
			}
			_, _ = hash.Write([]byte("["))
			first := true
			for decoder.More() {
				if !first {
					_, _ = hash.Write([]byte(","))
				}
				first = false
				var value any
				if err := decoder.Decode(&value); err != nil {
					return [32]byte{}, ErrInvalidJSON
				}
				encoded, err := json.Marshal(value)
				if err != nil {
					return [32]byte{}, err
				}
				_, _ = hash.Write(encoded)
			}
			if _, err := decoder.Token(); err != nil {
				return [32]byte{}, ErrInvalidJSON
			}
			_, _ = hash.Write([]byte("]"))
		} else {
			var value any
			if err := decoder.Decode(&value); err != nil {
				return [32]byte{}, ErrInvalidJSON
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				return [32]byte{}, err
			}
			_, _ = hash.Write(encoded)
		}
	}
	_, _ = hash.Write([]byte("}"))
	var sum [32]byte
	copy(sum[:], hash.Sum(nil))
	return sum, nil
}

type countingReader struct {
	reader io.Reader
	read   int64
	track  func(int64)
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += int64(n)
	r.track(r.read)
	return n, err
}
