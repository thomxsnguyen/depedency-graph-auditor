package pypi

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	pythonNamePattern    = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?`)
	pythonNameNormalizer = regexp.MustCompile(`[-_.]+`)
	directURLScheme      = regexp.MustCompile(`(?i)^(?:https?|file|git\+|hg\+|svn\+|bzr\+):`)
)

// Target is the deterministic Python environment used for marker evaluation.
type Target struct {
	PythonVersion     string
	PythonFullVersion string
	Platform          string
	SysPlatform       string
	OSName            string
	PlatformSystem    string
	Implementation    string
}

// NewTarget validates and constructs a supported Python marker environment.
func NewTarget(pythonVersion, platform string) (Target, error) {
	parsed, err := ParseVersion(pythonVersion)
	if err != nil || len(parsed.release) < 2 || len(parsed.release) > 3 || parsed.epoch != 0 || parsed.hasPre || parsed.hasPost || parsed.hasDev || len(parsed.local) != 0 {
		return Target{}, fmt.Errorf("python target version must be stable major.minor or major.minor.patch, got %q", pythonVersion)
	}
	fullVersion := fmt.Sprintf("%d.%d", parsed.release[0], parsed.release[1])
	if len(parsed.release) == 2 {
		fullVersion += ".0"
	} else {
		fullVersion += fmt.Sprintf(".%d", parsed.release[2])
	}
	target := Target{
		PythonVersion:     fmt.Sprintf("%d.%d", parsed.release[0], parsed.release[1]),
		PythonFullVersion: fullVersion,
		Platform:          strings.ToLower(platform),
		Implementation:    "cpython",
	}
	switch target.Platform {
	case "linux":
		target.SysPlatform = "linux"
		target.OSName = "posix"
		target.PlatformSystem = "Linux"
	case "windows":
		target.SysPlatform = "win32"
		target.OSName = "nt"
		target.PlatformSystem = "Windows"
	case "darwin":
		target.SysPlatform = "darwin"
		target.OSName = "posix"
		target.PlatformSystem = "Darwin"
	default:
		return Target{}, fmt.Errorf("unsupported Python platform %q (want linux, windows, or darwin)", platform)
	}
	return target, nil
}

// Requirement is one parsed PEP 508 name-based requirement in the supported
// subset. Marker retains the source expression for diagnostics.
type Requirement struct {
	Name       string
	Constraint string
	Marker     string
	Active     bool
}

// NormalizeName applies Python package-name normalization.
func NormalizeName(name string) string {
	return strings.ToLower(pythonNameNormalizer.ReplaceAllString(name, "-"))
}

// ParseRequirement parses the name, PEP 440 constraint, and optional marker.
// Direct URLs and extras are rejected by the first implementation contract.
func ParseRequirement(raw string, target Target) (Requirement, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Requirement{}, fmt.Errorf("empty Python requirement")
	}
	if directURLScheme.MatchString(raw) || strings.HasPrefix(raw, ".") || strings.HasPrefix(raw, "/") {
		return Requirement{}, fmt.Errorf("unsupported local, VCS, or URL requirement %q", raw)
	}

	nameMatch := pythonNamePattern.FindString(raw)
	if nameMatch == "" {
		return Requirement{}, fmt.Errorf("invalid Python package name in requirement %q", raw)
	}
	remainder := strings.TrimSpace(strings.TrimPrefix(raw, nameMatch))
	if strings.HasPrefix(remainder, "[") {
		return Requirement{}, fmt.Errorf("requirement extras are not supported in %q", raw)
	}
	if strings.HasPrefix(remainder, "@") || strings.Contains(remainder, " @ ") {
		return Requirement{}, fmt.Errorf("direct URL requirements are not supported in %q", raw)
	}

	constraint, marker := remainder, ""
	if separator := markerSeparator(remainder); separator >= 0 {
		constraint = strings.TrimSpace(remainder[:separator])
		marker = strings.TrimSpace(remainder[separator+1:])
	}
	if strings.HasPrefix(constraint, "(") && strings.HasSuffix(constraint, ")") {
		constraint = strings.TrimSpace(constraint[1 : len(constraint)-1])
	}
	normalizedConstraint, err := NormalizeConstraint(constraint)
	if err != nil {
		return Requirement{}, fmt.Errorf("invalid constraint for %s: %w", nameMatch, err)
	}

	requirement := Requirement{
		Name:       NormalizeName(nameMatch),
		Constraint: normalizedConstraint,
		Marker:     marker,
		Active:     true,
	}
	if marker != "" {
		active, err := EvaluateMarker(marker, target)
		if err != nil {
			return Requirement{}, fmt.Errorf("invalid marker for %s: %w", nameMatch, err)
		}
		requirement.Active = active
	}
	return requirement, nil
}

func markerSeparator(value string) int {
	quote := rune(0)
	for index, current := range value {
		if quote != 0 {
			if current == quote {
				quote = 0
			}
			continue
		}
		if current == '\'' || current == '"' {
			quote = current
			continue
		}
		if current == ';' {
			return index
		}
	}
	return -1
}

type markerTokenKind int

const (
	markerEOF markerTokenKind = iota
	markerWord
	markerString
	markerOperator
	markerLeftParen
	markerRightParen
)

type markerToken struct {
	kind  markerTokenKind
	value string
}

func tokenizeMarker(input string) ([]markerToken, error) {
	var tokens []markerToken
	for index := 0; index < len(input); {
		if unicode.IsSpace(rune(input[index])) {
			index++
			continue
		}
		switch input[index] {
		case '(':
			tokens = append(tokens, markerToken{kind: markerLeftParen, value: "("})
			index++
			continue
		case ')':
			tokens = append(tokens, markerToken{kind: markerRightParen, value: ")"})
			index++
			continue
		case '\'', '"':
			quote := input[index]
			index++
			start := index
			for index < len(input) && input[index] != quote {
				index++
			}
			if index >= len(input) {
				return nil, fmt.Errorf("unterminated marker string")
			}
			tokens = append(tokens, markerToken{kind: markerString, value: input[start:index]})
			index++
			continue
		}
		if strings.ContainsRune("<>=!~", rune(input[index])) {
			start := index
			index++
			if index < len(input) && input[index] == '=' {
				index++
			}
			tokens = append(tokens, markerToken{kind: markerOperator, value: input[start:index]})
			continue
		}
		start := index
		for index < len(input) && !unicode.IsSpace(rune(input[index])) && !strings.ContainsRune("()<>!=~'\"", rune(input[index])) {
			index++
		}
		if start == index {
			return nil, fmt.Errorf("unexpected marker character %q", input[index])
		}
		tokens = append(tokens, markerToken{kind: markerWord, value: input[start:index]})
	}
	return append(tokens, markerToken{kind: markerEOF}), nil
}

type markerParser struct {
	tokens []markerToken
	index  int
	target Target
}

// EvaluateMarker evaluates the supported PEP 508 boolean expression without
// executing arbitrary code.
func EvaluateMarker(expression string, target Target) (bool, error) {
	tokens, err := tokenizeMarker(expression)
	if err != nil {
		return false, err
	}
	parser := markerParser{tokens: tokens, target: target}
	result, err := parser.parseOr()
	if err != nil {
		return false, err
	}
	if parser.current().kind != markerEOF {
		return false, fmt.Errorf("unexpected marker token %q", parser.current().value)
	}
	return result, nil
}

func (p *markerParser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for strings.EqualFold(p.current().value, "or") {
		p.index++
		right, parseErr := p.parseAnd()
		if parseErr != nil {
			return false, parseErr
		}
		left = left || right
	}
	return left, nil
}

func (p *markerParser) parseAnd() (bool, error) {
	left, err := p.parseTerm()
	if err != nil {
		return false, err
	}
	for strings.EqualFold(p.current().value, "and") {
		p.index++
		right, parseErr := p.parseTerm()
		if parseErr != nil {
			return false, parseErr
		}
		left = left && right
	}
	return left, nil
}

func (p *markerParser) parseTerm() (bool, error) {
	if p.current().kind == markerLeftParen {
		p.index++
		value, err := p.parseOr()
		if err != nil {
			return false, err
		}
		if p.current().kind != markerRightParen {
			return false, fmt.Errorf("missing closing marker parenthesis")
		}
		p.index++
		return value, nil
	}
	left, leftVariable, err := p.parseOperand()
	if err != nil {
		return false, err
	}
	operator, err := p.parseOperator()
	if err != nil {
		return false, err
	}
	right, rightVariable, err := p.parseOperand()
	if err != nil {
		return false, err
	}
	return compareMarkerValues(left, right, operator, leftVariable, rightVariable), nil
}

func (p *markerParser) parseOperand() (string, string, error) {
	token := p.current()
	if token.kind != markerWord && token.kind != markerString {
		return "", "", fmt.Errorf("expected marker operand, got %q", token.value)
	}
	p.index++
	if token.kind == markerString {
		return token.value, "", nil
	}
	value, ok := markerVariableValue(token.value, p.target)
	if !ok {
		return "", "", fmt.Errorf("unsupported marker variable %q", token.value)
	}
	return value, token.value, nil
}

func (p *markerParser) parseOperator() (string, error) {
	token := p.current()
	if token.kind == markerOperator {
		p.index++
		switch token.value {
		case "==", "!=", "<", "<=", ">", ">=", "~=", "===":
			return token.value, nil
		default:
			return "", fmt.Errorf("unsupported marker operator %q", token.value)
		}
	}
	if strings.EqualFold(token.value, "in") {
		p.index++
		return "in", nil
	}
	if strings.EqualFold(token.value, "not") && strings.EqualFold(p.peek().value, "in") {
		p.index += 2
		return "not in", nil
	}
	return "", fmt.Errorf("expected marker comparison operator, got %q", token.value)
}

func (p *markerParser) current() markerToken { return p.tokens[p.index] }
func (p *markerParser) peek() markerToken {
	if p.index+1 >= len(p.tokens) {
		return markerToken{kind: markerEOF}
	}
	return p.tokens[p.index+1]
}

func markerVariableValue(name string, target Target) (string, bool) {
	switch name {
	case "python_version":
		return target.PythonVersion, true
	case "python_full_version":
		return target.PythonFullVersion, true
	case "sys_platform":
		return target.SysPlatform, true
	case "os_name":
		return target.OSName, true
	case "platform_system":
		return target.PlatformSystem, true
	case "implementation_name":
		return target.Implementation, true
	case "platform_python_implementation":
		return "CPython", true
	case "implementation_version":
		return target.PythonFullVersion, true
	case "extra":
		return "", true
	default:
		return "", false
	}
}

func compareMarkerValues(left, right, operator, leftVariable, rightVariable string) bool {
	if operator == "in" {
		return strings.Contains(right, left)
	}
	if operator == "not in" {
		return !strings.Contains(right, left)
	}
	compared := strings.Compare(left, right)
	if leftVariable == "python_version" || leftVariable == "python_full_version" || leftVariable == "implementation_version" ||
		rightVariable == "python_version" || rightVariable == "python_full_version" || rightVariable == "implementation_version" {
		if leftVersion, leftErr := ParseVersion(left); leftErr == nil {
			if rightVersion, rightErr := ParseVersion(right); rightErr == nil {
				compared = leftVersion.Compare(rightVersion)
			}
		}
	}
	switch operator {
	case "==", "===":
		return compared == 0
	case "!=":
		return compared != 0
	case "<":
		return compared < 0
	case "<=":
		return compared <= 0
	case ">":
		return compared > 0
	case ">=":
		return compared >= 0
	case "~=":
		leftVersion, leftErr := ParseVersion(left)
		rightVersion, rightErr := ParseVersion(right)
		return leftErr == nil && rightErr == nil && leftVersion.Compare(rightVersion) >= 0 && compatiblePrefix(leftVersion, rightVersion)
	default:
		return false
	}
}
