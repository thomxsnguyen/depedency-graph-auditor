// Package semver provides version range parsing and resolution backed by
// github.com/Masterminds/semver/v3. It wraps the library behind a thin interface
// so the rest of the codebase stays independent of the underlying implementation.
package semver

import (
	"fmt"
	"sort"

	ms "github.com/Masterminds/semver/v3"
)

// Constraint is a parsed semver range (e.g. "^4.18.0", "~1.2.3", ">=1.0.0 <2.0.0").
// The zero value is not usable; always obtain one via ParseRange.
type Constraint struct {
	inner *ms.Constraints
	raw   string
}

// ParseRange parses a version range string into a Constraint.
// Supported operators: ^ (caret), ~ (tilde), exact versions, >=/<= ranges.
// Returns an error if the range string is not recognised.
func ParseRange(rangeStr string) (Constraint, error) {
	c, err := ms.NewConstraint(rangeStr)
	if err != nil {
		return Constraint{}, fmt.Errorf("semver: parse range %q: %w", rangeStr, err)
	}
	return Constraint{inner: c, raw: rangeStr}, nil
}

// Resolve returns the highest version in available that satisfies the constraint.
// Versions that cannot be parsed are silently skipped.
// Returns an error if no version in available satisfies the constraint.
func Resolve(c Constraint, available []string) (string, error) {
	// Parse and collect all valid candidate versions.
	candidates := make([]*ms.Version, 0, len(available))
	for _, v := range available {
		parsed, err := ms.NewVersion(v)
		if err != nil {
			continue // skip malformed entries
		}
		if c.inner.Check(parsed) {
			candidates = append(candidates, parsed)
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("semver: no version satisfies constraint %q", c.raw)
	}

	// Sort ascending and return the last (highest) match.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LessThan(candidates[j])
	})
	return candidates[len(candidates)-1].Original(), nil
}
