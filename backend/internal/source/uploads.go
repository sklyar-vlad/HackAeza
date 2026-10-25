package source

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

const ChunkBytes = 8 << 20

var (
	ErrUploadNotFound   = errors.New("upload not found")
	ErrUploadOffset     = errors.New("chunk offset does not match received bytes")
	ErrUploadState      = errors.New("upload is not accepting this operation")
	ErrUploadIncomplete = errors.New("upload has missing bytes")
)

type Upload struct {
	ID             string        `json:"uploadId"`
	TotalBytes     int64         `json:"totalBytes"`
	ReceivedBytes  int64         `json:"receivedBytes"`
	ProcessedBytes int64         `json:"processedBytes"`
	Stage          string        `json:"stage"`
	ChunkBytes     int64         `json:"chunkBytes"`
	JobID          *entity.JobID `json:"jobId"`
}

type Uploads struct {
	pool    *pgxpool.Pool
	service *Service
}

func NewUploads(pool *pgxpool.Pool, service *Service) *Uploads {
	return &Uploads{pool: pool, service: service}
}

const uploadColumns = `id::text, total_bytes, received_bytes, processed_bytes, stage, job_id::text`

func scanUpload(row pgx.Row) (Upload, error) {
	upload := Upload{ChunkBytes: ChunkBytes}
	err := row.Scan(&upload.ID, &upload.TotalBytes, &upload.ReceivedBytes, &upload.ProcessedBytes, &upload.Stage, &upload.JobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Upload{}, ErrUploadNotFound
	}
	return upload, err
}

func (u *Uploads) Start(ctx context.Context, actorID string, totalBytes int64) (Upload, error) {
	if totalBytes <= 0 || totalBytes > MaxSnapshotBytes {
		return Upload{}, jobs.ErrInvalidInput
	}
	return scanUpload(u.pool.QueryRow(ctx, `INSERT INTO source_uploads (actor_id, total_bytes)
		VALUES ($1, $2) RETURNING `+uploadColumns, actorID, totalBytes))
}

func (u *Uploads) Get(ctx context.Context, actorID, id string) (Upload, error) {
	return scanUpload(u.pool.QueryRow(ctx, `SELECT `+uploadColumns+` FROM source_uploads
		WHERE id = $1 AND actor_id = $2`, id, actorID))
}

func (u *Uploads) Append(ctx context.Context, actorID, id string, offset int64, data []byte) (Upload, error) {
	var upload Upload
	err := pgx.BeginFunc(ctx, u.pool, func(tx pgx.Tx) error {
		var err error
		upload, err = scanUpload(tx.QueryRow(ctx, `SELECT `+uploadColumns+` FROM source_uploads
			WHERE id = $1 AND actor_id = $2 FOR UPDATE`, id, actorID))
		switch {
		case err != nil:
			return err
		case upload.Stage != "receiving":
			return ErrUploadState
		case len(data) == 0 || offset+int64(len(data)) > upload.TotalBytes:
			return jobs.ErrInvalidInput
		case offset < upload.ReceivedBytes:
			var stored bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM source_upload_chunks
				WHERE upload_id = $1 AND byte_offset = $2 AND octet_length(data) = $3)`, id, offset, len(data)).Scan(&stored); err != nil {
				return err
			}
			if stored {
				return nil
			}
			return ErrUploadOffset
		case offset != upload.ReceivedBytes:
			return ErrUploadOffset
		}
		if _, err := tx.Exec(ctx, `INSERT INTO source_upload_chunks (upload_id, byte_offset, data) VALUES ($1, $2, $3)`,
			id, offset, data); err != nil {
			return err
		}
		upload.ReceivedBytes += int64(len(data))
		_, err = tx.Exec(ctx, `UPDATE source_uploads SET received_bytes = $2, updated_at = now() WHERE id = $1`,
			id, upload.ReceivedBytes)
		return err
	})
	return upload, err
}

func (u *Uploads) Complete(ctx context.Context, actorID, id string, submission jobs.Submission) (entity.JobAccepted, error) {
	tag, err := u.pool.Exec(ctx, `UPDATE source_uploads SET stage = 'processing', updated_at = now()
		WHERE id = $1 AND actor_id = $2 AND stage = 'receiving' AND received_bytes = total_bytes`, id, actorID)
	if err != nil {
		return entity.JobAccepted{}, err
	}
	if tag.RowsAffected() == 0 {
		upload, err := u.Get(ctx, actorID, id)
		if err != nil {
			return entity.JobAccepted{}, err
		}
		if upload.ReceivedBytes < upload.TotalBytes {
			return entity.JobAccepted{}, ErrUploadIncomplete
		}
		return entity.JobAccepted{}, ErrUploadState
	}
	accepted, err := u.process(ctx, id, submission)
	finish := context.WithoutCancel(ctx)
	if err != nil {
		_, _ = u.pool.Exec(finish, `UPDATE source_uploads SET stage = 'failed', updated_at = now() WHERE id = $1`, id)
	} else {
		_, _ = u.pool.Exec(finish, `UPDATE source_uploads SET stage = 'accepted', processed_bytes = total_bytes,
			job_id = $2, updated_at = now() WHERE id = $1`, id, accepted.JobID)
	}
	_, _ = u.pool.Exec(finish, `DELETE FROM source_upload_chunks WHERE upload_id = $1`, id)
	return accepted, err
}

func (u *Uploads) process(ctx context.Context, id string, submission jobs.Submission) (entity.JobAccepted, error) {
	var total int64
	if err := u.pool.QueryRow(ctx, `SELECT total_bytes FROM source_uploads WHERE id = $1`, id).Scan(&total); err != nil {
		return entity.JobAccepted{}, err
	}
	payload := make([]byte, 0, total)
	rows, err := u.pool.Query(ctx, `SELECT data FROM source_upload_chunks WHERE upload_id = $1 ORDER BY byte_offset`, id)
	if err != nil {
		return entity.JobAccepted{}, err
	}
	for rows.Next() {
		var chunk []byte
		if err := rows.Scan(&chunk); err != nil {
			rows.Close()
			return entity.JobAccepted{}, err
		}
		payload = append(payload, chunk...)
	}
	if err := rows.Err(); err != nil {
		return entity.JobAccepted{}, err
	}
	if int64(len(payload)) != total {
		return entity.JobAccepted{}, fmt.Errorf("assembled %d of %d bytes: %w", len(payload), total, ErrUploadIncomplete)
	}
	var reported time.Time
	return u.service.ImportTracked(ctx, payload, submission, func(processed int64) {
		if processed < total && time.Since(reported) < 300*time.Millisecond {
			return
		}
		reported = time.Now()
		_, _ = u.pool.Exec(context.WithoutCancel(ctx), `UPDATE source_uploads SET processed_bytes = $2, updated_at = now()
			WHERE id = $1`, id, processed)
	})
}
