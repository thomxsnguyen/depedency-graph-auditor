package depfile

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/pypi"
)

type pyProjectDocument struct {
	Project struct {
		Name         string   `toml:"name"`
		Dependencies []string `toml:"dependencies"`
		Dynamic      []string `toml:"dynamic"`
	} `toml:"project"`
}

// ParsePyProject parses standardized [project] metadata without invoking a
// Python build backend.
func ParsePyProject(reader io.Reader, target pypi.Target) (Manifest, error) {
	var document pyProjectDocument
	if _, err := toml.NewDecoder(reader).Decode(&document); err != nil {
		return Manifest{}, fmt.Errorf("depfile: decode pyproject.toml: %w", err)
	}
	name := strings.TrimSpace(document.Project.Name)
	if name == "" {
		return Manifest{}, fmt.Errorf("depfile: pyproject.toml requires [project].name")
	}
	for _, field := range document.Project.Dynamic {
		if strings.EqualFold(strings.TrimSpace(field), "dependencies") {
			return Manifest{}, fmt.Errorf("depfile: dynamic pyproject.toml dependencies are not supported")
		}
	}
	dependencies, err := parsePythonRequirements(document.Project.Dependencies, target, "pyproject.toml")
	if err != nil {
		return Manifest{}, err
	}
	return Manifest{Name: name, Dependencies: dependencies}, nil
}

// ParseRequirements parses the supported requirements.txt subset. rootName is
// supplied by source orchestration because requirements files contain no
// project-name field.
func ParseRequirements(reader io.Reader, rootName string, target pypi.Target) (Manifest, error) {
	if strings.TrimSpace(rootName) == "" {
		return Manifest{}, fmt.Errorf("depfile: requirements.txt root name is required")
	}

	scanner := bufio.NewScanner(reader)
	const maxRequirementsLine = 1 << 20
	scanner.Buffer(make([]byte, 4096), maxRequirementsLine)
	type numberedRequirement struct {
		value string
		line  int
	}
	var requirements []numberedRequirement
	var continued strings.Builder
	continuedLine := 0
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Text()
		if continued.Len() == 0 {
			continuedLine = lineNumber
		}
		if hasLineContinuation(line) {
			trimmed := strings.TrimRight(line, " \t\r")
			continued.WriteString(strings.TrimSpace(strings.TrimSuffix(trimmed, `\`)))
			continued.WriteByte(' ')
			continue
		}
		continued.WriteString(line)
		logicalLine := strings.TrimSpace(stripRequirementsComment(continued.String()))
		continued.Reset()
		if logicalLine == "" {
			continue
		}
		if err := validateRequirementsLine(logicalLine); err != nil {
			return Manifest{}, fmt.Errorf("depfile: requirements.txt line %d: %w", continuedLine, err)
		}
		requirements = append(requirements, numberedRequirement{value: logicalLine, line: continuedLine})
	}
	if err := scanner.Err(); err != nil {
		return Manifest{}, fmt.Errorf("depfile: read requirements.txt: %w", err)
	}
	if continued.Len() != 0 {
		return Manifest{}, fmt.Errorf("depfile: requirements.txt line %d: unterminated line continuation", continuedLine)
	}

	parsed := make([]pypi.Requirement, 0, len(requirements))
	for _, item := range requirements {
		requirement, err := pypi.ParseRequirement(item.value, target)
		if err != nil {
			return Manifest{}, fmt.Errorf("depfile: requirements.txt line %d: %w", item.line, err)
		}
		parsed = append(parsed, requirement)
	}
	return Manifest{Name: rootName, Dependencies: pythonDependencies(parsed)}, nil
}

func parsePythonRequirements(values []string, target pypi.Target, source string) ([]Dependency, error) {
	parsed := make([]pypi.Requirement, 0, len(values))
	for index, value := range values {
		requirement, err := pypi.ParseRequirement(value, target)
		if err != nil {
			return nil, fmt.Errorf("depfile: %s requirement %d: %w", source, index+1, err)
		}
		parsed = append(parsed, requirement)
	}
	return pythonDependencies(parsed), nil
}

func pythonDependencies(requirements []pypi.Requirement) []Dependency {
	constraints := make(map[string][]string)
	for _, requirement := range requirements {
		if !requirement.Active {
			continue
		}
		constraint := strings.TrimSpace(requirement.Constraint)
		if constraint != "" && !containsString(constraints[requirement.Name], constraint) {
			constraints[requirement.Name] = append(constraints[requirement.Name], constraint)
		} else if _, exists := constraints[requirement.Name]; !exists {
			constraints[requirement.Name] = nil
		}
	}

	dependencies := make([]Dependency, 0, len(constraints))
	for name, values := range constraints {
		sort.Strings(values)
		dependencies = append(dependencies, Dependency{Name: name, VersionRange: strings.Join(values, ",")})
	}
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].Name == dependencies[j].Name {
			return dependencies[i].VersionRange < dependencies[j].VersionRange
		}
		return dependencies[i].Name < dependencies[j].Name
	})
	return dependencies
}

func hasLineContinuation(line string) bool {
	trimmed := strings.TrimRight(line, " \t\r")
	count := 0
	for index := len(trimmed) - 1; index >= 0 && trimmed[index] == '\\'; index-- {
		count++
	}
	return count%2 == 1
}

func stripRequirementsComment(line string) string {
	quote := byte(0)
	for index := 0; index < len(line); index++ {
		current := line[index]
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
		if current == '#' && (index == 0 || line[index-1] == ' ' || line[index-1] == '\t') {
			return line[:index]
		}
	}
	return line
}

func validateRequirementsLine(line string) error {
	lower := strings.ToLower(strings.TrimSpace(line))
	if strings.HasPrefix(lower, "-") {
		return fmt.Errorf("pip directives and options are not supported: %q", line)
	}
	if strings.Contains(line, "${") {
		return fmt.Errorf("environment-variable substitution is not supported")
	}
	if strings.Contains(lower, " --hash") || strings.Contains(lower, " --") {
		return fmt.Errorf("per-requirement pip options are not supported: %q", line)
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
