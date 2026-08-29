// Package pypi implements the bounded Python packaging behavior used by the
// auditor's public PyPI registry client.
package pypi

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var pythonVersionPattern = regexp.MustCompile(`(?i)^v?(?:(\d+)!)?(\d+(?:[._-]\d+)*)(?:[-_.]?(a|b|c|rc|alpha|beta|pre|preview)[-_.]?(\d+)?)?(?:(?:-(\d+))|(?:[-_.]?(post|rev|r)[-_.]?(\d+)?))?(?:[-_.]?(dev)[-_.]?(\d+)?)?(?:\+([a-z0-9]+(?:[-_.][a-z0-9]+)*))?$`)

// Version is a normalized PEP 440 version used for constraint evaluation.
type Version struct {
	raw     string
	epoch   int
	release []int
	preKind int
	preNum  int
	hasPre  bool
	postNum int
	hasPost bool
	devNum  int
	hasDev  bool
	local   []string
}

// ParseVersion parses the public-version forms needed by PyPI metadata.
func ParseVersion(value string) (Version, error) {
	value = strings.TrimSpace(value)
	matches := pythonVersionPattern.FindStringSubmatch(value)
	if matches == nil {
		return Version{}, fmt.Errorf("pypi: invalid PEP 440 version %q", value)
	}

	version := Version{raw: value}
	var err error
	if matches[1] != "" {
		version.epoch, err = strconv.Atoi(matches[1])
		if err != nil {
			return Version{}, fmt.Errorf("pypi: invalid epoch in version %q", value)
		}
	}
	for _, part := range regexp.MustCompile(`[._-]`).Split(matches[2], -1) {
		number, parseErr := strconv.Atoi(part)
		if parseErr != nil {
			return Version{}, fmt.Errorf("pypi: invalid release in version %q", value)
		}
		version.release = append(version.release, number)
	}

	if matches[3] != "" {
		version.hasPre = true
		switch strings.ToLower(matches[3]) {
		case "a", "alpha":
			version.preKind = 0
		case "b", "beta":
			version.preKind = 1
		default:
			version.preKind = 2
		}
		version.preNum = parseImplicitNumber(matches[4])
	}
	if matches[5] != "" {
		version.hasPost = true
		version.postNum, _ = strconv.Atoi(matches[5])
	} else if matches[6] != "" {
		version.hasPost = true
		version.postNum = parseImplicitNumber(matches[7])
	}
	if matches[8] != "" {
		version.hasDev = true
		version.devNum = parseImplicitNumber(matches[9])
	}
	if matches[10] != "" {
		version.local = regexp.MustCompile(`[-_.]`).Split(strings.ToLower(matches[10]), -1)
	}
	return version, nil
}

func parseImplicitNumber(value string) int {
	if value == "" {
		return 0
	}
	number, _ := strconv.Atoi(value)
	return number
}

// Compare returns -1, 0, or 1 according to PEP 440 ordering.
func (v Version) Compare(other Version) int {
	if compared := compareInt(v.epoch, other.epoch); compared != 0 {
		return compared
	}
	if compared := compareRelease(v.release, other.release); compared != 0 {
		return compared
	}
	if compared := comparePre(v, other); compared != 0 {
		return compared
	}
	if compared := compareOptionalNumber(v.hasPost, v.postNum, other.hasPost, other.postNum, -1); compared != 0 {
		return compared
	}
	if compared := compareOptionalNumber(v.hasDev, v.devNum, other.hasDev, other.devNum, 1); compared != 0 {
		return compared
	}
	return compareLocal(v.local, other.local)
}

func comparePre(left, right Version) int {
	leftRank, leftKind, leftNum := 1, 0, 0
	rightRank, rightKind, rightNum := 1, 0, 0
	if left.hasPre {
		leftRank, leftKind, leftNum = 0, left.preKind, left.preNum
	} else if left.hasDev && !left.hasPost {
		leftRank = -1
	}
	if right.hasPre {
		rightRank, rightKind, rightNum = 0, right.preKind, right.preNum
	} else if right.hasDev && !right.hasPost {
		rightRank = -1
	}
	if compared := compareInt(leftRank, rightRank); compared != 0 {
		return compared
	}
	if compared := compareInt(leftKind, rightKind); compared != 0 {
		return compared
	}
	return compareInt(leftNum, rightNum)
}

func compareOptionalNumber(leftSet bool, left int, rightSet bool, right int, missingRank int) int {
	if !leftSet && !rightSet {
		return 0
	}
	if !leftSet {
		return missingRank
	}
	if !rightSet {
		return -missingRank
	}
	return compareInt(left, right)
}

func compareRelease(left, right []int) int {
	length := max(len(left), len(right))
	for index := 0; index < length; index++ {
		leftPart, rightPart := 0, 0
		if index < len(left) {
			leftPart = left[index]
		}
		if index < len(right) {
			rightPart = right[index]
		}
		if compared := compareInt(leftPart, rightPart); compared != 0 {
			return compared
		}
	}
	return 0
}

func compareLocal(left, right []string) int {
	if len(left) == 0 && len(right) == 0 {
		return 0
	}
	if len(left) == 0 {
		return -1
	}
	if len(right) == 0 {
		return 1
	}
	length := min(len(left), len(right))
	for index := 0; index < length; index++ {
		leftNumber, leftErr := strconv.Atoi(left[index])
		rightNumber, rightErr := strconv.Atoi(right[index])
		if leftErr == nil && rightErr == nil {
			if compared := compareInt(leftNumber, rightNumber); compared != 0 {
				return compared
			}
			continue
		}
		if leftErr == nil {
			return 1
		}
		if rightErr == nil {
			return -1
		}
		if compared := strings.Compare(left[index], right[index]); compared != 0 {
			return compared
		}
	}
	return compareInt(len(left), len(right))
}

func compareInt(left, right int) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

type versionClause struct {
	operator    string
	value       string
	version     Version
	prefixEpoch int
	prefix      []int
}

// NormalizeConstraint validates and renders a deterministic PEP 440
// constraint string for storage in jobs and dependency metadata.
func NormalizeConstraint(raw string) (string, error) {
	clauses, _, err := parseConstraint(raw)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		parts = append(parts, clause.operator+clause.value)
	}
	return strings.Join(parts, ","), nil
}

// ResolveVersion selects the highest available PEP 440 version satisfying all
// comma-separated clauses. Stable releases are preferred unless the constraint
// explicitly names a prerelease or no stable candidate matches.
func ResolveVersion(constraint string, available []string) (string, error) {
	clauses, permitsPrerelease, err := parseConstraint(constraint)
	if err != nil {
		return "", err
	}
	if len(clauses) == 1 && clauses[0].operator == "===" {
		for _, raw := range available {
			if raw == clauses[0].value {
				return raw, nil
			}
		}
		return "", fmt.Errorf("pypi: no version satisfies constraint %q", constraint)
	}

	type candidate struct {
		original string
		version  Version
	}
	var stable, prerelease []candidate
	for _, raw := range available {
		version, parseErr := ParseVersion(raw)
		if parseErr != nil || !matchesClauses(version, raw, clauses) {
			continue
		}
		entry := candidate{original: raw, version: version}
		if version.hasPre || version.hasDev {
			prerelease = append(prerelease, entry)
		} else {
			stable = append(stable, entry)
		}
	}
	candidates := stable
	if permitsPrerelease || len(candidates) == 0 {
		candidates = append(candidates, prerelease...)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("pypi: no version satisfies constraint %q", constraint)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].version.Compare(candidates[j].version) < 0
	})
	return candidates[len(candidates)-1].original, nil
}

func parseConstraint(raw string) ([]versionClause, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return nil, false, nil
	}
	parts := strings.Split(raw, ",")
	clauses := make([]versionClause, 0, len(parts))
	permitsPrerelease := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		operator := "=="
		value := part
		for _, candidate := range []string{"===", "~=", "==", "!=", "<=", ">=", "<", ">"} {
			if strings.HasPrefix(part, candidate) {
				operator = candidate
				value = strings.TrimSpace(strings.TrimPrefix(part, candidate))
				break
			}
		}
		if value == "" {
			return nil, false, fmt.Errorf("pypi: missing version in constraint %q", raw)
		}
		if operator == "~=" {
			releasePart := strings.SplitN(value, "+", 2)[0]
			releasePart = strings.SplitN(releasePart, "-", 2)[0]
			if strings.Count(releasePart, ".") < 1 {
				return nil, false, fmt.Errorf("pypi: compatible constraint requires at least two release segments in %q", part)
			}
		}
		clause := versionClause{operator: operator, value: value}
		if operator == "===" {
			if len(parts) != 1 {
				return nil, false, fmt.Errorf("pypi: arbitrary equality cannot be combined in %q", raw)
			}
			clauses = append(clauses, clause)
			continue
		}
		if strings.HasSuffix(value, ".*") {
			if operator != "==" && operator != "!=" {
				return nil, false, fmt.Errorf("pypi: wildcard requires == or != in %q", part)
			}
			prefixText := strings.TrimSuffix(value, ".*")
			if epochParts := strings.Split(prefixText, "!"); len(epochParts) == 2 {
				epoch, parseErr := strconv.Atoi(epochParts[0])
				if parseErr != nil {
					return nil, false, fmt.Errorf("pypi: invalid wildcard constraint %q", part)
				}
				clause.prefixEpoch = epoch
				prefixText = epochParts[1]
			} else if len(epochParts) > 2 {
				return nil, false, fmt.Errorf("pypi: invalid wildcard constraint %q", part)
			}
			for _, item := range strings.Split(prefixText, ".") {
				number, parseErr := strconv.Atoi(item)
				if parseErr != nil {
					return nil, false, fmt.Errorf("pypi: invalid wildcard constraint %q", part)
				}
				clause.prefix = append(clause.prefix, number)
			}
		} else {
			parsed, parseErr := ParseVersion(value)
			if parseErr != nil {
				return nil, false, fmt.Errorf("pypi: invalid constraint %q: %w", part, parseErr)
			}
			clause.version = parsed
			permitsPrerelease = permitsPrerelease || parsed.hasPre || parsed.hasDev
		}
		clauses = append(clauses, clause)
	}
	return clauses, permitsPrerelease, nil
}

func matchesClauses(version Version, raw string, clauses []versionClause) bool {
	for _, clause := range clauses {
		if clause.operator == "===" {
			if raw != clause.value {
				return false
			}
			continue
		}
		if len(clause.prefix) > 0 {
			matches := version.epoch == clause.prefixEpoch && len(version.release) >= len(clause.prefix)
			for index := range clause.prefix {
				matches = matches && version.release[index] == clause.prefix[index]
			}
			if (clause.operator == "==") != matches {
				return false
			}
			continue
		}
		compared := compareSpecifierVersion(version, clause.version)
		switch clause.operator {
		case "==":
			if compared != 0 {
				return false
			}
		case "!=":
			if compared == 0 {
				return false
			}
		case "<":
			if compared >= 0 {
				return false
			}
		case "<=":
			if compared > 0 {
				return false
			}
		case ">":
			if compared <= 0 {
				return false
			}
		case ">=":
			if compared < 0 {
				return false
			}
		case "~=":
			if compared < 0 || !compatiblePrefix(version, clause.version) {
				return false
			}
		}
	}
	return true
}

func compatiblePrefix(candidate, minimum Version) bool {
	if candidate.epoch != minimum.epoch {
		return false
	}
	prefixLength := len(minimum.release) - 1
	if prefixLength < 1 {
		prefixLength = 1
	}
	if len(candidate.release) < prefixLength {
		return false
	}
	for index := 0; index < prefixLength; index++ {
		if candidate.release[index] != minimum.release[index] {
			return false
		}
	}
	return true
}

func compareSpecifierVersion(candidate, specified Version) int {
	if len(specified.local) == 0 {
		candidate.local = nil
	}
	return candidate.Compare(specified)
}
