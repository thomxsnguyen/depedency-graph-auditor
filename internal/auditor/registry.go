package auditor

import "context"

// PackageMetadata is the data returned by the registry for a single (name, version) pair.
type PackageMetadata struct {
	Name         string
	Version      string
	License      string
	Dependencies map[string]string // dep name → version range (e.g., "^4.18.0")
}

// RegistryClient fetches package metadata from an external registry (e.g., npm).
// It is responsible for resolving version ranges to exact versions before returning.
type RegistryClient interface {
	// FetchPackage returns the resolved metadata for the given name and exact version.
	// The version passed in is already resolved — this does not do range resolution.
	FetchPackage(ctx context.Context, name, version string) (*PackageMetadata, error)
}
