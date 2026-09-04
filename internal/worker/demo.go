package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

type DemoHandler struct{}

type DemoPayload struct {
	DurationMS        int             `json:"durationMs"`
	TransientFailures int             `json:"transientFailures"`
	PermanentFailure  bool            `json:"permanentFailure"`
	Result            json.RawMessage `json:"result,omitempty"`
}

func (DemoHandler) Handle(ctx context.Context, value job.Job) (job.HandlerResult, error) {
	var payload DemoPayload
	if err := json.Unmarshal(value.Payload, &payload); err != nil {
		return job.HandlerResult{}, job.Failure(job.ErrorPermanent, fmt.Errorf("invalid demo payload: %w", err))
	}
	if payload.DurationMS < 0 || payload.DurationMS > 120000 || payload.TransientFailures < 0 || payload.TransientFailures > 20 {
		return job.HandlerResult{}, job.Failure(job.ErrorPermanent, fmt.Errorf("demo controls are outside allowed bounds"))
	}
	if payload.PermanentFailure {
		return job.HandlerResult{}, job.Failure(job.ErrorPermanent, fmt.Errorf("configured permanent failure"))
	}
	if value.Attempts <= payload.TransientFailures {
		return job.HandlerResult{}, job.Failure(job.ErrorTransient, fmt.Errorf("configured transient failure %d of %d", value.Attempts, payload.TransientFailures))
	}
	select {
	case <-ctx.Done():
		return job.HandlerResult{}, job.Failure(job.ErrorCancelled, ctx.Err())
	case <-time.After(time.Duration(payload.DurationMS) * time.Millisecond):
	}
	result := payload.Result
	if len(result) == 0 {
		result = json.RawMessage(`{"message":"demo job completed"}`)
	}
	return job.HandlerResult{Result: result}, nil
}
