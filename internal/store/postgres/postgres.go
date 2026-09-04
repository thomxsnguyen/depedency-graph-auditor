// Package postgres implements durable job and dead-letter storage in Postgres.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	storecontract "github.com/thomxsnguyen/mini-distributed-job-api/internal/store"
)

// ErrJobExists is returned when CreateJob receives an ID already in the jobs
// table. The insert remains idempotent; callers can avoid dispatching a second
// channel signal for the existing row.
var ErrJobExists = errors.New("job already exists")

// Store persists queue and DLQ state in Postgres.
type Store struct {
	pool *pgxpool.Pool
}

var _ storecontract.Store = (*Store)(nil)

// New creates a Postgres store using an existing connection pool. The caller
// owns the pool and is responsible for closing it.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateJob inserts a pending job. Duplicate IDs are left unchanged and
// reported as ErrJobExists.
func (s *Store) CreateJob(ctx context.Context, j job.Job) error {
	maxAttempts := j.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = job.DefaultMaxAttempts
	}
	scheduledAt := j.ScheduledAt
	if scheduledAt.IsZero() {
		scheduledAt = time.Now()
	}

	tag, err := s.pool.Exec(ctx, `
		INSERT INTO jobs (
			id, type, payload, status, attempts, max_attempts, scheduled_at,
			root_job_id, parent_job_id, internal
		)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7, $8, NULLIF($9, ''), $10)
		ON CONFLICT (id) DO NOTHING
	`, j.ID, j.Type, jsonValue(j.Payload), job.StatusPending, j.Attempts, maxAttempts,
		scheduledAt, firstNonEmpty(j.RootJobID, j.ID), j.ParentJobID, j.Internal)
	if err != nil {
		return fmt.Errorf("create job %q: %w", j.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("create job %q: %w", j.ID, ErrJobExists)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// AcquireJob atomically selects the next eligible pending job and marks it
// running. SKIP LOCKED prevents competing acquirers from selecting the same row.
func (s *Store) AcquireJob(ctx context.Context) (job.Job, bool, error) {
	row := s.pool.QueryRow(ctx, `
		WITH next_job AS (
			SELECT id
			FROM jobs
			WHERE status = $1
			  AND scheduled_at <= NOW()
			ORDER BY scheduled_at, created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE jobs AS j
		SET status = $2
		FROM next_job
		WHERE j.id = next_job.id
		RETURNING j.id, j.type, j.payload, j.status,
		          j.attempts, j.max_attempts, j.scheduled_at
	`, job.StatusPending, job.StatusRunning)

	j, err := scanJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return job.Job{}, false, nil
	}
	if err != nil {
		return job.Job{}, false, fmt.Errorf("acquire job: %w", err)
	}
	return j, true, nil
}

// CompleteJob marks a job completed.
func (s *Store) CompleteJob(ctx context.Context, id string) error {
	return s.updateStatus(ctx, id, job.StatusCompleted)
}

// RetryJob persists the updated attempt count and future delivery time, then
// returns the job to pending status.
func (s *Store) RetryJob(ctx context.Context, j job.Job) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs
		SET status = $1,
		    attempts = $2,
		    scheduled_at = $3
		WHERE id = $4
	`, job.StatusPending, j.Attempts, j.ScheduledAt, j.ID)
	if err != nil {
		return fmt.Errorf("retry job %q: %w", j.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("retry job %q: not found", j.ID)
	}
	return nil
}

// DeadLetterJob atomically marks the job dead-lettered and writes its terminal
// snapshot to the dlq table.
func (s *Store) DeadLetterJob(ctx context.Context, j job.Job, jobErr error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("dead-letter job %q: begin transaction: %w", j.ID, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = $1,
		    attempts = $2
		WHERE id = $3
	`, job.StatusDeadLettered, j.Attempts, j.ID)
	if err != nil {
		return fmt.Errorf("dead-letter job %q: update job: %w", j.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("dead-letter job %q: not found", j.ID)
	}

	errorText := ""
	if jobErr != nil {
		errorText = jobErr.Error()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO dlq (job_id, job_type, payload, attempts, error)
		VALUES ($1, $2, $3::jsonb, $4, $5)
	`, j.ID, j.Type, jsonValue(j.Payload), j.Attempts, errorText); err != nil {
		return fmt.Errorf("dead-letter job %q: insert DLQ entry: %w", j.ID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("dead-letter job %q: commit: %w", j.ID, err)
	}
	return nil
}

// ReclaimStuckJobs returns jobs left running by a previous process to pending.
func (s *Store) ReclaimStuckJobs(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs
		SET status = $1
		WHERE status = $2
	`, job.StatusPending, job.StatusRunning)
	if err != nil {
		return 0, fmt.Errorf("reclaim stuck jobs: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// DLQEntries returns dead-lettered entries in the order they were created.
func (s *Store) DLQEntries(ctx context.Context) ([]dlq.DLQEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT job_id, job_type, payload, attempts, error, dead_at
		FROM dlq
		ORDER BY dead_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("query DLQ entries: %w", err)
	}
	defer rows.Close()

	entries := make([]dlq.DLQEntry, 0)
	for rows.Next() {
		var entry dlq.DLQEntry
		var payload []byte
		if err := rows.Scan(
			&entry.Job.ID,
			&entry.Job.Type,
			&payload,
			&entry.Job.Attempts,
			&entry.Err,
			&entry.DeadAt,
		); err != nil {
			return nil, fmt.Errorf("scan DLQ entry: %w", err)
		}
		entry.Job.Payload = payload
		entry.Job.Status = job.StatusDeadLettered
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate DLQ entries: %w", err)
	}
	return entries, nil
}

func (s *Store) updateStatus(ctx context.Context, id string, status job.Status) error {
	tag, err := s.pool.Exec(ctx, `UPDATE jobs SET status = $1 WHERE id = $2`, status, id)
	if err != nil {
		return fmt.Errorf("set job %q status to %q: %w", id, status, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("set job %q status to %q: not found", id, status)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (job.Job, error) {
	var j job.Job
	var payload []byte
	if err := row.Scan(
		&j.ID,
		&j.Type,
		&payload,
		&j.Status,
		&j.Attempts,
		&j.MaxAttempts,
		&j.ScheduledAt,
	); err != nil {
		return job.Job{}, err
	}
	j.Payload = payload
	return j, nil
}

func jsonValue(payload []byte) any {
	if len(payload) == 0 {
		return nil
	}
	return string(payload)
}
