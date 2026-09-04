package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/gomod"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

type GoMetadataClient interface {
	Fetch(context.Context, string, string) (gomod.Metadata, error)
}

type GoModuleServiceHandler struct{ Client GoMetadataClient }

type goAuditResult struct {
	Ecosystem     string          `json:"ecosystem"`
	Name          string          `json:"name"`
	Version       string          `json:"version"`
	License       string          `json:"license"`
	Verdict       auditor.Verdict `json:"verdict"`
	ParentName    string          `json:"parentName,omitempty"`
	ParentVersion string          `json:"parentVersion,omitempty"`
}

type goModulePayload struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	ParentName    string `json:"parentName,omitempty"`
	ParentVersion string `json:"parentVersion,omitempty"`
}

func (h GoModuleServiceHandler) Handle(ctx context.Context, value job.Job) (job.HandlerResult, error) {
	var payload goModulePayload
	if err := json.Unmarshal(value.Payload, &payload); err != nil {
		return job.HandlerResult{}, job.Failure(job.ErrorPermanent, fmt.Errorf("invalid Go module payload: %w", err))
	}
	metadata, err := h.Client.Fetch(ctx, payload.Name, payload.Version)
	if err != nil {
		var classified interface{ Retryable() bool }
		if errors.As(err, &classified) && classified.Retryable() {
			return job.HandlerResult{}, job.Failure(job.ErrorTransient, err)
		}
		return job.HandlerResult{}, job.Failure(job.ErrorPermanent, err)
	}
	result, _ := json.Marshal(goAuditResult{Ecosystem: "go", Name: metadata.ModulePath,
		Version: payload.Version, License: "UNKNOWN", Verdict: auditor.VerdictPolicyViolation,
		ParentName: payload.ParentName, ParentVersion: payload.ParentVersion})
	children := make([]job.Submission, 0, len(metadata.Requirements))
	for _, dependency := range metadata.Requirements {
		body, _ := json.Marshal(goModulePayload{Name: dependency.ModulePath, Version: dependency.Version,
			ParentName: metadata.ModulePath, ParentVersion: payload.Version})
		sum := sha256.Sum256([]byte(value.RootJobID + "\x00go\x00" + dependency.ModulePath + "\x00" + dependency.Version))
		children = append(children, job.Submission{Type: "audit_go_module", Payload: body,
			MaxAttempts: value.MaxAttempts, RootJobID: value.RootJobID, ParentJobID: value.ID,
			Internal: true, IdempotencyKey: "audit-child:" + hex.EncodeToString(sum[:])})
	}
	return job.HandlerResult{Result: result, Children: children}, nil
}
