package gomod

import (
	"fmt"

	"golang.org/x/mod/module"
	modsemver "golang.org/x/mod/semver"
)

// ValidateVersion requires an exact canonical Go module semantic version.
func ValidateVersion(version string) error {
	if !modsemver.IsValid(version) {
		return fmt.Errorf("invalid Go module version %q", version)
	}
	if module.CanonicalVersion(version) != version {
		return fmt.Errorf("Go module version %q must be canonical", version)
	}
	if module.IsPseudoVersion(version) {
		if _, err := module.PseudoVersionTime(version); err != nil {
			return fmt.Errorf("invalid Go pseudo-version %q: %w", version, err)
		}
		if _, err := module.PseudoVersionBase(version); err != nil {
			return fmt.Errorf("invalid Go pseudo-version %q: %w", version, err)
		}
	}
	return nil
}

// ValidateCoordinate validates an exact version and its compatibility with the
// module path's major-version suffix.
func ValidateCoordinate(modulePath, version string) error {
	if err := ValidateVersion(version); err != nil {
		return err
	}
	if err := module.Check(modulePath, version); err != nil {
		return err
	}
	return nil
}

// CompareVersions compares two exact canonical versions using Go module
// semantic-version ordering.
func CompareVersions(left, right string) (int, error) {
	if err := ValidateVersion(left); err != nil {
		return 0, fmt.Errorf("left version: %w", err)
	}
	if err := ValidateVersion(right); err != nil {
		return 0, fmt.Errorf("right version: %w", err)
	}
	return compareCanonicalVersions(left, right), nil
}

func compareCanonicalVersions(left, right string) int {
	return modsemver.Compare(left, right)
}
