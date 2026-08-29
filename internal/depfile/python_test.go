package depfile_test

import (
	"strings"
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/depfile"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/pypi"
)

func pythonTarget(t *testing.T, platform string) pypi.Target {
	t.Helper()
	target, err := pypi.NewTarget("3.12", platform)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestParsePyProject(t *testing.T) {
	manifest, err := depfile.ParsePyProject(strings.NewReader(`
[project]
name = "python-app"
dependencies = [
  "Requests>=2.31,<3",
  "colorama==0.4.6; sys_platform == 'win32'",
  "flask~=3.0",
]
`), pythonTarget(t, "linux"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Name != "python-app" {
		t.Fatalf("name: got %q", manifest.Name)
	}
	want := []depfile.Dependency{
		{Name: "flask", VersionRange: "~=3.0"},
		{Name: "requests", VersionRange: ">=2.31,<3"},
	}
	if len(manifest.Dependencies) != len(want) {
		t.Fatalf("dependencies: %+v", manifest.Dependencies)
	}
	for index := range want {
		if manifest.Dependencies[index] != want[index] {
			t.Fatalf("dependency %d: got %+v, want %+v", index, manifest.Dependencies[index], want[index])
		}
	}
}

func TestParsePyProjectRejectsDynamicDependencies(t *testing.T) {
	_, err := depfile.ParsePyProject(strings.NewReader(`
[project]
name = "dynamic-app"
dynamic = ["dependencies"]
`), pythonTarget(t, "linux"))
	if err == nil || !strings.Contains(err.Error(), "dynamic") {
		t.Fatalf("error: %v", err)
	}
}

func TestParsePyProjectRequiresNameAndValidTOML(t *testing.T) {
	for name, input := range map[string]string{
		"missing name": "[project]\ndependencies = [\"requests\"]",
		"invalid TOML": "[project\nname = \"broken\"",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := depfile.ParsePyProject(strings.NewReader(input), pythonTarget(t, "linux")); err == nil {
				t.Fatal("expected pyproject error")
			}
		})
	}
}

func TestParsePythonManifestsWithNoDependencies(t *testing.T) {
	pyproject, err := depfile.ParsePyProject(strings.NewReader("[project]\nname = \"empty\"\n"), pythonTarget(t, "linux"))
	if err != nil {
		t.Fatal(err)
	}
	requirements, err := depfile.ParseRequirements(strings.NewReader("# empty\n"), "empty", pythonTarget(t, "linux"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pyproject.Dependencies) != 0 || len(requirements.Dependencies) != 0 {
		t.Fatalf("pyproject=%+v requirements=%+v", pyproject.Dependencies, requirements.Dependencies)
	}
}

func TestParseRequirementsSubset(t *testing.T) {
	manifest, err := depfile.ParseRequirements(strings.NewReader(`
# comment
Requests>=2.31,\
  <3 # trailing comment
colorama==0.4.6 ; sys_platform == "win32"
flask~=3.0
`), "requirements-root", pythonTarget(t, "linux"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Name != "requirements-root" || len(manifest.Dependencies) != 2 {
		t.Fatalf("manifest: %+v", manifest)
	}
	if manifest.Dependencies[0].Name != "flask" || manifest.Dependencies[1].Name != "requests" {
		t.Fatalf("order: %+v", manifest.Dependencies)
	}
}

func TestParseRequirementsRejectsUnsupportedLines(t *testing.T) {
	for _, line := range []string{
		"-r other.txt",
		"--index-url https://example.com/simple",
		"requests[socks]>=2",
		"demo @ https://example.com/demo.whl",
		"../local-package",
	} {
		t.Run(line, func(t *testing.T) {
			_, err := depfile.ParseRequirements(strings.NewReader(line+"\n"), "root", pythonTarget(t, "linux"))
			if err == nil || !strings.Contains(err.Error(), "line 1") {
				t.Fatalf("error: %v", err)
			}
		})
	}
}
