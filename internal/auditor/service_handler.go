package auditor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
)

type AuditPackageServiceHandler struct {
	Registry  RegistryClient
	Policy    PolicyChecker
	JobType   string
	Ecosystem string
}

type auditPackageResult struct {
	Ecosystem     string  `json:"ecosystem"`
	Name          string  `json:"name"`
	Version       string  `json:"version"`
	License       string  `json:"license"`
	Verdict       Verdict `json:"verdict"`
	ParentName    string  `json:"parentName,omitempty"`
	ParentVersion string  `json:"parentVersion,omitempty"`
}

func (h AuditPackageServiceHandler) Handle(ctx context.Context, value job.Job) (job.HandlerResult, error) {
	var payload AuditPayload
	if err := json.Unmarshal(value.Payload, &payload); err != nil {
		return job.HandlerResult{}, job.Failure(job.ErrorPermanent, fmt.Errorf("invalid audit package payload: %w", err))
	}
	metadata, err := h.Registry.FetchPackage(ctx, payload.Name, payload.Version)
	if err != nil {
		return job.HandlerResult{}, classifyServiceError(err)
	}
	verdict := h.Policy.Check(*metadata)
	result, err := json.Marshal(auditPackageResult{
		Ecosystem: h.Ecosystem,
		Name:      metadata.Name, Version: metadata.Version, License: metadata.License, Verdict: verdict,
		ParentName: payload.ParentName, ParentVersion: payload.ParentVersion,
	})
	if err != nil {
		return job.HandlerResult{}, job.Failure(job.ErrorPermanent, err)
	}
	children := make([]job.Submission, 0, len(metadata.Dependencies))
	for name, version := range metadata.Dependencies {
		body, err := json.Marshal(AuditPayload{Name: name, Version: version, ParentName: metadata.Name, ParentVersion: metadata.Version})
		if err != nil {
			return job.HandlerResult{}, job.Failure(job.ErrorPermanent, err)
		}
		children = append(children, auditChild(value.RootJobID, value.ID, body, h.JobType, h.Ecosystem, name, version, value.MaxAttempts))
	}
	return job.HandlerResult{Result: result, Children: children}, nil
}

func auditChild(rootID, parentID string, payload json.RawMessage, jobType, ecosystem, name, version string, maxAttempts int) job.Submission {
	sum := sha256.Sum256([]byte(rootID + "\x00" + ecosystem + "\x00" + name + "\x00" + version))
	return job.Submission{Type: jobType, Payload: payload, MaxAttempts: maxAttempts,
		RootJobID: rootID, ParentJobID: parentID, Internal: true,
		IdempotencyKey: "audit-child:" + hex.EncodeToString(sum[:]),
	}
}

func classifyServiceError(err error) error {
	var networkError net.Error
	if strings.Contains(strings.ToLower(err.Error()), "rate limit") ||
		strings.Contains(err.Error(), "status 429") || strings.Contains(err.Error(), "status 502") ||
		strings.Contains(err.Error(), "status 503") || strings.Contains(err.Error(), "status 504") {
		return job.Failure(job.ErrorTransient, err)
	}
	if errors.As(err, &networkError) {
		return job.Failure(job.ErrorTransient, err)
	}
	return job.Failure(job.ErrorPermanent, err)
}
