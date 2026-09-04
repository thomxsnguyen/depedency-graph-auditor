package worker

import (
	"context"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

func TestDemoHandlerFailureControls(t *testing.T) {
	handler := DemoHandler{}
	transient := job.Job{Payload: []byte(`{"transientFailures":1}`), Attempts: 1}
	if _, err := handler.Handle(context.Background(), transient); job.KindOf(err) != job.ErrorTransient {
		t.Fatalf("transient kind=%s err=%v", job.KindOf(err), err)
	}
	permanent := job.Job{Payload: []byte(`{"permanentFailure":true}`), Attempts: 1}
	if _, err := handler.Handle(context.Background(), permanent); job.KindOf(err) != job.ErrorPermanent {
		t.Fatalf("permanent kind=%s err=%v", job.KindOf(err), err)
	}
	completed := job.Job{Payload: []byte(`{"transientFailures":1}`), Attempts: 2}
	if _, err := handler.Handle(context.Background(), completed); err != nil {
		t.Fatal(err)
	}
}
