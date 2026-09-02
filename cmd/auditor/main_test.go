package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/depfile"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph"
	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/gomod"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/pypi"
)

type mainRoundFetcherFunc func(context.Context, []gomod.Coordinate) (map[gomod.Coordinate]gomod.Metadata, error)

func (f mainRoundFetcherFunc) FetchRound(ctx context.Context, coordinates []gomod.Coordinate) (map[gomod.Coordinate]gomod.Metadata, error) {
	return f(ctx, coordinates)
}

func TestNewSeedJobCarriesRootParent(t *testing.T) {
	seed, err := newSeedJob("personal-portfolio", depfile.Dependency{
		Name: "react", VersionRange: "^19.1.0",
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload auditor.AuditPayload
	if err := json.Unmarshal(seed.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	want := auditor.AuditPayload{
		Name: "react", Version: "^19.1.0", ParentName: "personal-portfolio",
	}
	if payload != want {
		t.Fatalf("payload: got %+v, want %+v", payload, want)
	}
}

func TestParseCLIArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    cliConfig
		wantErr string
	}{
		{name: "input only", args: []string{"package.json"}, want: cliConfig{input: "package.json", analysis: "packages", ecosystem: "npm", manifestPath: "package.json"}},
		{name: "with output", args: []string{"--output", "report.md", "package.json"}, want: cliConfig{input: "package.json", outputPath: "report.md", analysis: "packages", ecosystem: "npm", manifestPath: "package.json"}},
		{
			name: "file analysis",
			args: []string{"--analysis", "files", "--output", "file-graph.json", "personal-portfolio"},
			want: cliConfig{input: "personal-portfolio", outputPath: "file-graph.json", analysis: "files"},
		},
		{
			name: "GitHub file analysis",
			args: []string{"--analysis", "files", "--output", "file-graph.json", "https://github.com/acme/widget"},
			want: cliConfig{
				input: "https://github.com/acme/widget", outputPath: "file-graph.json", analysis: "files",
				repository: githubsource.Repository{Owner: "acme", Name: "widget"}, isGitHub: true,
			},
		},
		{
			name: "GitHub file analysis with ref",
			args: []string{"--analysis", "files", "--output", "file-graph.json", "--ref", "development", "https://github.com/acme/widget"},
			want: cliConfig{
				input: "https://github.com/acme/widget", outputPath: "file-graph.json", analysis: "files", ref: "development",
				repository: githubsource.Repository{Owner: "acme", Name: "widget"}, isGitHub: true,
			},
		},
		{
			name: "GitHub input with options",
			args: []string{"--ref", "development", "--manifest", "packages/web/package.json", "https://github.com/acme/widget.git/"},
			want: cliConfig{
				input: "https://github.com/acme/widget.git/", analysis: "packages", ecosystem: "npm", ref: "development", manifestPath: "packages/web/package.json",
				repository: githubsource.Repository{Owner: "acme", Name: "widget"}, isGitHub: true,
			},
		},
		{
			name: "local Python pyproject",
			args: []string{"--ecosystem", "python", "--python-version", "3.11", "--python-platform", "darwin", "pyproject.toml"},
			want: cliConfig{
				input: "pyproject.toml", analysis: "packages", ecosystem: "python", manifestPath: "pyproject.toml",
				pythonTarget: mustPythonTarget(t, "3.11", "darwin"),
			},
		},
		{
			name: "GitHub Python requirements",
			args: []string{"--ecosystem", "python", "--manifest", "requirements.txt", "https://github.com/acme/widget"},
			want: cliConfig{
				input: "https://github.com/acme/widget", analysis: "packages", ecosystem: "python", manifestPath: "requirements.txt",
				pythonTarget: mustPythonTarget(t, "3.12", "linux"),
				repository:   githubsource.Repository{Owner: "acme", Name: "widget"}, isGitHub: true,
			},
		},
		{
			name: "local Go manifest",
			args: []string{"--ecosystem", "go", "services/api/go.mod"},
			want: cliConfig{input: "services/api/go.mod", analysis: "packages", ecosystem: "go", manifestPath: "go.mod"},
		},
		{
			name: "GitHub Go manifest",
			args: []string{"--ecosystem", "go", "--manifest", "go.mod", "https://github.com/acme/widget"},
			want: cliConfig{
				input: "https://github.com/acme/widget", analysis: "packages", ecosystem: "go", manifestPath: "go.mod",
				repository: githubsource.Repository{Owner: "acme", Name: "widget"}, isGitHub: true,
			},
		},
		{
			name: "nested GitHub Go manifest with ref",
			args: []string{"--ecosystem", "go", "--manifest", "services/api/go.mod", "--ref", "main", "https://github.com/acme/widget"},
			want: cliConfig{
				input: "https://github.com/acme/widget", analysis: "packages", ecosystem: "go", ref: "main", manifestPath: "services/api/go.mod",
				repository: githubsource.Repository{Owner: "acme", Name: "widget"}, isGitHub: true,
			},
		},
		{name: "missing input", wantErr: "missing manifest path or GitHub repository URL"},
		{name: "missing output value", args: []string{"--output"}, wantErr: "flag needs an argument"},
		{name: "empty output value", args: []string{"--output=", "package.json"}, wantErr: "non-empty path"},
		{name: "empty ref", args: []string{"--ref=", "https://github.com/acme/widget"}, wantErr: "--ref requires a non-empty value"},
		{name: "empty manifest", args: []string{"--manifest=", "https://github.com/acme/widget"}, wantErr: "--manifest requires a non-empty path"},
		{name: "ref on local input", args: []string{"--ref", "main", "package.json"}, wantErr: "valid only with GitHub"},
		{name: "manifest on local input", args: []string{"--manifest", "web/package.json", "package.json"}, wantErr: "valid only with GitHub"},
		{name: "invalid GitHub scheme", args: []string{"http://github.com/acme/widget"}, wantErr: "must use https"},
		{name: "invalid GitHub host", args: []string{"https://example.com/acme/widget"}, wantErr: "host must be github.com"},
		{name: "invalid manifest traversal", args: []string{"--manifest", "../package.json", "https://github.com/acme/widget"}, wantErr: "invalid segment"},
		{name: "invalid ecosystem", args: []string{"--ecosystem", "ruby", "package.json"}, wantErr: "npm, python, or go"},
		{name: "invalid analysis", args: []string{"--analysis", "symbols", "package.json"}, wantErr: "packages or files"},
		{name: "file analysis requires output", args: []string{"--analysis", "files", "project"}, wantErr: "--output is required"},
		{name: "file analysis rejects ecosystem", args: []string{"--analysis", "files", "--ecosystem", "npm", "--output", "graph.json", "project"}, wantErr: "not valid"},
		{name: "Python option with npm", args: []string{"--python-version", "3.11", "package.json"}, wantErr: "valid only"},
		{name: "Python option with Go", args: []string{"--ecosystem", "go", "--python-version", "3.11", "go.mod"}, wantErr: "valid only with --ecosystem python"},
		{name: "Python GitHub manifest required", args: []string{"--ecosystem", "python", "https://github.com/acme/widget"}, wantErr: "--manifest is required"},
		{name: "unsupported Python local manifest", args: []string{"--ecosystem", "python", "Pipfile"}, wantErr: "unsupported Python manifest"},
		{name: "unsupported Python platform", args: []string{"--ecosystem", "python", "--python-platform", "solaris", "pyproject.toml"}, wantErr: "unsupported Python platform"},
		{name: "Go GitHub manifest required", args: []string{"--ecosystem", "go", "https://github.com/acme/widget"}, wantErr: "--manifest is required for Go GitHub input"},
		{name: "wrong local Go basename", args: []string{"--ecosystem", "go", "gomod.txt"}, wantErr: "local file named go.mod"},
		{name: "wrong GitHub Go basename", args: []string{"--ecosystem", "go", "--manifest", "package.json", "https://github.com/acme/widget"}, wantErr: "requires a go.mod manifest"},
		{name: "Go manifest option on local input", args: []string{"--ecosystem", "go", "--manifest", "go.mod", "go.mod"}, wantErr: "valid only with GitHub"},
		{name: "extra input", args: []string{"one.json", "two.json"}, wantErr: "unexpected extra positional argument"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCLIArgs(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error: got %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("config: got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func mustPythonTarget(t *testing.T, version, platform string) pypi.Target {
	t.Helper()
	target, err := pypi.NewTarget(version, platform)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestExecuteFileAnalysisUsesQueueAndProducesCompleteGraph(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "src/App.tsx", `import Button from "./components/Button"`)
	writeProjectFile(t, root, "src/components/Button.tsx", `import App from "../App"`)
	writeProjectFile(t, root, "src/orphan.ts", `export const orphan = true`)
	writeProjectFile(t, root, "src/broken.ts", `import "./missing"`)
	writeProjectFile(t, root, "node_modules/ignored.js", `import "../../src/App"`)
	writeProjectFile(t, root, "README.md", "ignored")

	firstOutput := filepath.Join(t.TempDir(), "file-graph.json")
	report, err := executeFileAnalysis(
		context.Background(),
		context.Background(),
		root,
		"",
		firstOutput,
		time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Nodes) != 4 {
		t.Fatalf("nodes: got %+v, want four project files", report.Nodes)
	}
	wantEdges := []filegraph.Edge{
		{From: "src/App.tsx", To: "src/components/Button.tsx"},
		{From: "src/components/Button.tsx", To: "src/App.tsx"},
	}
	if !reflect.DeepEqual(report.Edges, wantEdges) {
		t.Fatalf("edges: got %+v, want %+v", report.Edges, wantEdges)
	}
	if len(report.Diagnostics) != 1 || report.Diagnostics[0].Path != "src/broken.ts" || report.Diagnostics[0].Import != "./missing" {
		t.Fatalf("diagnostics: %+v", report.Diagnostics)
	}

	firstJSON, err := os.ReadFile(firstOutput)
	if err != nil {
		t.Fatal(err)
	}
	secondOutput := filepath.Join(t.TempDir(), "file-graph.json")
	if _, err := executeFileAnalysis(
		context.Background(),
		context.Background(),
		root,
		"",
		secondOutput,
		time.Second,
	); err != nil {
		t.Fatal(err)
	}
	secondJSON, err := os.ReadFile(secondOutput)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("file graph output is not deterministic:\n%s\n%s", firstJSON, secondJSON)
	}
}

func TestExecuteFileAnalysisHandlesEmptyProject(t *testing.T) {
	output := filepath.Join(t.TempDir(), "file-graph.json")
	report, err := executeFileAnalysis(
		context.Background(), context.Background(), t.TempDir(), "", output, time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Nodes) != 0 || len(report.Edges) != 0 || len(report.Diagnostics) != 0 {
		t.Fatalf("report: %+v", report)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("output was not written: %v", err)
	}
}

func TestExecuteFileAnalysisReportsMissingRootGoModule(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "main.go", "package main\n")
	output := filepath.Join(t.TempDir(), "file-graph.json")
	report, err := executeFileAnalysis(
		context.Background(), context.Background(), root, "go-project", output, time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Nodes) != 1 || report.Nodes[0].Path != "main.go" || len(report.Edges) != 0 {
		t.Fatalf("graph: %+v", report)
	}
	if len(report.Diagnostics) != 1 || report.Diagnostics[0].Path != "go.mod" || !strings.Contains(report.Diagnostics[0].Message, "root go.mod is required") {
		t.Fatalf("diagnostics: %+v", report.Diagnostics)
	}
}

func TestExecuteFileAnalysisProducesOnePolyglotGraph(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "frontend/App.tsx", `import "./Button"`)
	writeProjectFile(t, root, "frontend/Button.tsx", "export const Button = true")
	writeProjectFile(t, root, "backend/__init__.py", "")
	writeProjectFile(t, root, "backend/app.py", "from . import models\nimport requests\n")
	writeProjectFile(t, root, "backend/models.py", "class Model:\n    pass\n")
	writeProjectFile(t, root, "go.mod", "module example.com/polyglot\n")
	writeProjectFile(t, root, "cmd/server/main.go", "package main\nimport \"example.com/polyglot/internal/config\"\n")
	writeProjectFile(t, root, "internal/config/load.go", "package config\n")
	writeProjectFile(t, root, "internal/config/types.go", "package config\n")
	writeProjectFile(t, root, "internal/config/config_test.go", "package config_test\n")
	writeProjectFile(t, root, "README.md", "unsupported")

	output := filepath.Join(t.TempDir(), "polyglot-file-graph.json")
	report, err := executeFileAnalysis(
		context.Background(), context.Background(), root, "polyglot", output, time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 1 || report.Root != "polyglot" {
		t.Fatalf("report metadata: %+v", report)
	}
	wantNodes := []filegraph.Node{
		{Path: "backend/__init__.py"},
		{Path: "backend/app.py"},
		{Path: "backend/models.py"},
		{Path: "cmd/server/main.go"},
		{Path: "frontend/App.tsx"},
		{Path: "frontend/Button.tsx"},
		{Path: "internal/config/config_test.go"},
		{Path: "internal/config/load.go"},
		{Path: "internal/config/types.go"},
	}
	if !reflect.DeepEqual(report.Nodes, wantNodes) {
		t.Fatalf("nodes: got %+v, want %+v", report.Nodes, wantNodes)
	}
	wantEdges := []filegraph.Edge{
		{From: "backend/app.py", To: "backend/models.py"},
		{From: "cmd/server/main.go", To: "internal/config/load.go"},
		{From: "cmd/server/main.go", To: "internal/config/types.go"},
		{From: "frontend/App.tsx", To: "frontend/Button.tsx"},
	}
	if !reflect.DeepEqual(report.Edges, wantEdges) {
		t.Fatalf("edges: got %+v, want %+v", report.Edges, wantEdges)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %+v", report.Diagnostics)
	}
}

func TestRunGitHubPolyglotFileAnalysis(t *testing.T) {
	archive := repositoryZIP(t, map[string]string{
		"acme-widget/src/widget/__init__.py":                   "",
		"acme-widget/src/widget/main.py":                       "from widget import models\nimport requests\n",
		"acme-widget/src/widget/models.py":                     "class Model:\n    pass\n",
		"acme-widget/tests/test_models.py":                     "from widget.models import Model\n",
		"acme-widget/.venv/ignored.py":                         "from widget import models\n",
		"acme-widget/go.mod":                                   "module example.com/widget\n",
		"acme-widget/cmd/server/main.go":                       "package main\nimport \"example.com/widget/internal/config\"\n",
		"acme-widget/internal/config/load.go":                  "package config\n",
		"acme-widget/internal/config/types.go":                 "package config\n",
		"acme-widget/internal/config/config_test.go":           "package config_test\n",
		"acme-widget/vendor/example.com/dependency/ignored.go": "package dependency\n",
	})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/acme/widget/zipball" {
			t.Errorf("path: got %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/zip")
		_, _ = writer.Write(archive)
	}))
	defer server.Close()

	t.Setenv("DATABASE_URL", "")
	output := filepath.Join(t.TempDir(), "file-graph.json")
	report, err := runFileAnalysisWithClient(cliConfig{
		input:      "https://github.com/acme/widget",
		outputPath: output,
		analysis:   "files",
		repository: githubsource.Repository{Owner: "acme", Name: "widget"},
		isGitHub:   true,
	}, time.Second, &githubsource.GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if report.Root != "widget" || len(report.Nodes) != 8 {
		t.Fatalf("report root/nodes: %+v", report)
	}
	wantEdges := []filegraph.Edge{
		{From: "cmd/server/main.go", To: "internal/config/load.go"},
		{From: "cmd/server/main.go", To: "internal/config/types.go"},
		{From: "src/widget/main.py", To: "src/widget/models.py"},
		{From: "tests/test_models.py", To: "src/widget/models.py"},
	}
	if !reflect.DeepEqual(report.Edges, wantEdges) {
		t.Fatalf("edges: got %+v, want %+v", report.Edges, wantEdges)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %+v", report.Diagnostics)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("output was not written: %v", err)
	}
}

func repositoryZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for path, contents := range files {
		file, err := writer.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func writeProjectFile(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParsePythonManifestAndSelectRegistry(t *testing.T) {
	config, err := parseCLIArgs([]string{"--ecosystem", "python", "pyproject.toml"})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := parseManifest(config, ManifestSource{Location: "pyproject.toml", Data: []byte(`
[project]
name = "python-app"
dependencies = ["requests>=2"]
`)})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Seed.Name != "python-app" || len(manifest.Seed.Dependencies) != 1 {
		t.Fatalf("manifest: %+v", manifest)
	}
	registry, err := registryForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.(*pypi.Client); !ok {
		t.Fatalf("registry: got %T, want *pypi.Client", registry)
	}
}

func TestManifestSelectionUsesLogicalBasename(t *testing.T) {
	tests := []struct {
		name          string
		config        cliConfig
		data          string
		wantName      string
		wantGoVersion string
		wantVersion   string
		wantErr       string
	}{
		{
			name:   "npm package JSON",
			config: cliConfig{ecosystem: "npm", manifestPath: "package.json"},
			data:   `{"name":"web","dependencies":{}}`, wantName: "web",
		},
		{
			name:   "nested npm package JSON",
			config: cliConfig{ecosystem: "npm", manifestPath: "apps/web/package.json"},
			data:   `{"name":"web","dependencies":{}}`, wantName: "web",
		},
		{
			name:   "npm wrong basename",
			config: cliConfig{ecosystem: "npm", manifestPath: "manifest.json"},
			data:   `{"name":"web","dependencies":{}}`, wantErr: `unsupported npm manifest "manifest.json"`,
		},
		{
			name:     "Go preserves shared seed and Go metadata",
			config:   cliConfig{ecosystem: "go", manifestPath: "services/api/go.mod"},
			data:     "module example.com/service\n\ngo 1.23\n\nrequire example.com/dependency v1.2.3\n",
			wantName: "example.com/service", wantGoVersion: "1.23", wantVersion: "v1.2.3",
		},
		{
			name:   "Go wrong basename",
			config: cliConfig{ecosystem: "go", manifestPath: "services/api/package.json"},
			data:   `{"name":"must-not-be-content-guessed","dependencies":{}}`, wantErr: `unsupported Go manifest "package.json"`,
		},
		{
			name:    "unknown ecosystem",
			config:  cliConfig{ecosystem: "ruby", manifestPath: "Gemfile"},
			wantErr: `unsupported ecosystem "ruby"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := parseManifest(tt.config, ManifestSource{Location: tt.config.manifestPath, Data: []byte(tt.data)})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error: got %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if manifest.Seed.Name != tt.wantName {
				t.Fatalf("manifest name: got %q, want %q", manifest.Seed.Name, tt.wantName)
			}
			if manifest.GoVersion != tt.wantGoVersion {
				t.Fatalf("Go version: got %q, want %q", manifest.GoVersion, tt.wantGoVersion)
			}
			if tt.wantVersion != "" {
				if len(manifest.Seed.Dependencies) != 1 || manifest.Seed.Dependencies[0].VersionRange != tt.wantVersion {
					t.Fatalf("shared dependency seed: got %+v, want exact version %q", manifest.Seed.Dependencies, tt.wantVersion)
				}
			}
		})
	}
}

func TestGoManifestCannotUseLegacyRegistryResolver(t *testing.T) {
	registry, err := registryForConfig(cliConfig{ecosystem: "go"})
	if err == nil || !strings.Contains(err.Error(), `no package registry resolver for ecosystem "go"`) {
		t.Fatalf("error: got %v", err)
	}
	if registry != nil {
		t.Fatalf("registry: got %T, want nil", registry)
	}
}

func TestSelectGoManifestAdaptsSharedSeed(t *testing.T) {
	parsed, err := parseManifest(cliConfig{ecosystem: "go", manifestPath: "go.mod"}, ManifestSource{Data: []byte(`
module example.com/root
go 1.16
require (
	example.com/a v1.0.0
	example.com/b v1.0.0
)
`)})
	if err != nil {
		t.Fatal(err)
	}
	fixtures := map[gomod.Coordinate]gomod.Metadata{
		{ModulePath: "example.com/a", Version: "v1.0.0"}: {ModulePath: "example.com/a"},
		{ModulePath: "example.com/b", Version: "v1.0.0"}: {ModulePath: "example.com/b"},
	}
	fetcher := mainRoundFetcherFunc(func(_ context.Context, coordinates []gomod.Coordinate) (map[gomod.Coordinate]gomod.Metadata, error) {
		result := make(map[gomod.Coordinate]gomod.Metadata, len(coordinates))
		for _, coordinate := range coordinates {
			result[coordinate] = fixtures[coordinate]
		}
		return result, nil
	})
	selection, err := selectGoManifest(context.Background(), parsed, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	want := []gomod.Coordinate{
		{ModulePath: "example.com/a", Version: "v1.0.0"},
		{ModulePath: "example.com/b", Version: "v1.0.0"},
	}
	if !reflect.DeepEqual(selection.Modules, want) {
		t.Fatalf("modules: got %+v, want %+v", selection.Modules, want)
	}
}

func TestGitHubGoManifestUsesExistingSourceBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/acme/widget/contents/services/api/go.mod" {
			t.Errorf("path: got %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte("module example.com/service\ngo 1.23\n"))
	}))
	defer server.Close()
	config, err := parseCLIArgs([]string{
		"--ecosystem", "go", "--manifest", "services/api/go.mod", "https://github.com/acme/widget",
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := readManifestSource(context.Background(), config, &githubsource.GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseManifest(config, source)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Seed.Name != "example.com/service" || parsed.GoVersion != "1.23" {
		t.Fatalf("manifest: %+v", parsed)
	}
}

func TestGoReportMetadataIdentifiesLicenseLimitation(t *testing.T) {
	metadata := reportMetadata(cliConfig{ecosystem: "go"}, "1.23")
	if metadata["Go version"] != "1.23" || !strings.Contains(metadata["License metadata"], "UNKNOWN") {
		t.Fatalf("metadata: %+v", metadata)
	}
}

func TestRunGoAuditMapsSelectionAndWritesReport(t *testing.T) {
	parsed, err := parseManifest(cliConfig{ecosystem: "go", manifestPath: "go.mod"}, ManifestSource{Data: []byte(`
module example.com/root
go 1.16
require example.com/a v1.0.0
`)})
	if err != nil {
		t.Fatal(err)
	}
	coordinate := gomod.Coordinate{ModulePath: "example.com/a", Version: "v1.0.0"}
	fetcher := mainRoundFetcherFunc(func(_ context.Context, coordinates []gomod.Coordinate) (map[gomod.Coordinate]gomod.Metadata, error) {
		return map[gomod.Coordinate]gomod.Metadata{coordinate: {ModulePath: coordinate.ModulePath}}, nil
	})
	outputPath := filepath.Join(t.TempDir(), "go-audit.md")
	report, err := runGoAudit(context.Background(), cliConfig{ecosystem: "go", outputPath: outputPath}, parsed, parsed.Seed.Name, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalPackages != 1 || len(report.PolicyViolations) != 1 {
		t.Fatalf("report: %+v", report)
	}
	markdown, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(markdown, []byte("example.com/a")) || !bytes.Contains(markdown, []byte("UNKNOWN")) || !bytes.Contains(markdown, []byte("Go version")) {
		t.Fatalf("Markdown omitted Go graph metadata:\n%s", markdown)
	}
}

func TestRunGoAuditDoesNotWriteIncompleteReport(t *testing.T) {
	parsed, err := parseManifest(cliConfig{ecosystem: "go", manifestPath: "go.mod"}, ManifestSource{Data: []byte(`
module example.com/root
go 1.16
require example.com/a v1.0.0
`)})
	if err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(t.TempDir(), "incomplete.md")
	fetcher := mainRoundFetcherFunc(func(context.Context, []gomod.Coordinate) (map[gomod.Coordinate]gomod.Metadata, error) {
		return nil, errors.New("proxy unavailable")
	})
	if _, err := runGoAudit(context.Background(), cliConfig{ecosystem: "go", outputPath: outputPath}, parsed, parsed.Seed.Name, fetcher); err == nil {
		t.Fatal("expected incomplete Go audit to fail")
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("incomplete report was written or stat failed unexpectedly: %v", err)
	}
}

func TestGitHubPythonRequirementsUseRepositoryAsRoot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/acme/widget/contents/config/requirements.txt" {
			t.Errorf("path: got %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte("requests>=2\n"))
	}))
	defer server.Close()
	config, err := parseCLIArgs([]string{
		"--ecosystem", "python",
		"--manifest", "config/requirements.txt",
		"https://github.com/acme/widget",
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := readManifestSource(context.Background(), config, &githubsource.GitHubClient{
		HTTPClient: server.Client(), BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := parseManifest(config, source)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Seed.Name != "widget" || len(manifest.Seed.Dependencies) != 1 {
		t.Fatalf("manifest: %+v", manifest)
	}
}

func TestGitHubPythonPyProjectUsesManifestName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/acme/widget/contents/pyproject.toml" {
			t.Errorf("path: got %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte("[project]\nname = \"github-python-app\"\ndependencies = [\"requests>=2\"]\n"))
	}))
	defer server.Close()
	config, err := parseCLIArgs([]string{
		"--ecosystem", "python",
		"--manifest", "pyproject.toml",
		"https://github.com/acme/widget",
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := readManifestSource(context.Background(), config, &githubsource.GitHubClient{
		HTTPClient: server.Client(), BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := parseManifest(config, source)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Seed.Name != "github-python-app" || len(manifest.Seed.Dependencies) != 1 {
		t.Fatalf("manifest: %+v", manifest)
	}
}

func TestReadManifestSourceLocalAndGitHub(t *testing.T) {
	const packageJSON = `{"name":"example","dependencies":{"react":"^19.1.0"}}`
	localPath := filepath.Join(t.TempDir(), "package.json")
	if err := os.WriteFile(localPath, []byte(packageJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	localConfig, err := parseCLIArgs([]string{localPath})
	if err != nil {
		t.Fatal(err)
	}
	local, err := readManifestSource(context.Background(), localConfig, &githubsource.GitHubClient{})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/acme/widget/contents/package.json" {
			t.Errorf("path: got %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte(packageJSON))
	}))
	defer server.Close()
	remoteConfig, err := parseCLIArgs([]string{"https://github.com/acme/widget"})
	if err != nil {
		t.Fatal(err)
	}
	remote, err := readManifestSource(context.Background(), remoteConfig, &githubsource.GitHubClient{
		HTTPClient: server.Client(), BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(local.Data, remote.Data) {
		t.Fatalf("source bytes differ: local=%q remote=%q", local.Data, remote.Data)
	}
	manifest, err := depfile.ParsePackageJSON(bytes.NewReader(remote.Data), true)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Name != "example" || len(manifest.Dependencies) != 1 {
		t.Fatalf("manifest: %+v", manifest)
	}
}

func TestGitHubManifestInvalidJSONFailsDuringPreflight(t *testing.T) {
	server := newGitHubManifestServer(t, `not valid json{{`)
	config, err := parseCLIArgs([]string{"https://github.com/acme/widget"})
	if err != nil {
		t.Fatal(err)
	}

	source, err := readManifestSource(context.Background(), config, &githubsource.GitHubClient{
		HTTPClient: server.Client(), BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := depfile.ParsePackageJSON(bytes.NewReader(source.Data), true); err == nil {
		t.Fatal("expected malformed downloaded manifest to fail during preflight parsing")
	}
}

func TestGitHubManifestNameBecomesReportRoot(t *testing.T) {
	server := newGitHubManifestServer(t, `{"name":"remote-app","dependencies":{}}`)
	manifest := readTestGitHubManifest(t, server)

	report := auditor.GenerateReport(auditor.NewPackageStore(), auditor.NewEdgeStore(), manifest.Name)
	markdown, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{
		Root: manifest.Name, Report: report,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markdown, "- Root: `remote-app`") {
		t.Fatalf("downloaded manifest name was not used as report root:\n%s", markdown)
	}
}

func TestGitHubManifestReachesAuditSeedJob(t *testing.T) {
	server := newGitHubManifestServer(t, `{"name":"remote-app","dependencies":{"react":"^19.1.0"}}`)
	manifest := readTestGitHubManifest(t, server)
	if len(manifest.Dependencies) != 1 {
		t.Fatalf("dependencies: got %d, want 1", len(manifest.Dependencies))
	}

	seed, err := newSeedJob(manifest.Name, manifest.Dependencies[0])
	if err != nil {
		t.Fatal(err)
	}
	var payload auditor.AuditPayload
	if err := json.Unmarshal(seed.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	want := auditor.AuditPayload{Name: "react", Version: "^19.1.0", ParentName: "remote-app"}
	if payload != want {
		t.Fatalf("payload: got %+v, want %+v", payload, want)
	}
}

func TestGitHubManifestWithNoDependenciesPreservesNoWorkBehavior(t *testing.T) {
	server := newGitHubManifestServer(t, `{"name":"empty-remote-app","dependencies":{},"devDependencies":{}}`)
	manifest := readTestGitHubManifest(t, server)

	if len(manifest.Dependencies) != 0 {
		t.Fatalf("dependencies: got %d, want 0", len(manifest.Dependencies))
	}
}

func newGitHubManifestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/acme/widget/contents/package.json" {
			t.Errorf("path: got %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func readTestGitHubManifest(t *testing.T, server *httptest.Server) depfile.Manifest {
	t.Helper()
	config, err := parseCLIArgs([]string{"https://github.com/acme/widget"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := readManifestSource(context.Background(), config, &githubsource.GitHubClient{
		HTTPClient: server.Client(), BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := depfile.ParsePackageJSON(bytes.NewReader(source.Data), true)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestWriteMarkdownReport(t *testing.T) {
	packages := auditor.NewPackageStore()
	packages.Add(auditor.Package{Name: "alpha", Version: "1.0.0", License: "MIT", Verdict: auditor.VerdictPass})
	edges := auditor.NewEdgeStore()
	report := auditor.GenerateReport(packages, edges, "app")
	outputPath := filepath.Join(t.TempDir(), "audit.md")

	if err := writeMarkdownReport(outputPath, "app", packages, edges, report); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Dependency Audit Report", "- Root: `app`", "| `alpha` | `1.0.0` | `MIT` | `pass` |"} {
		if !strings.Contains(string(contents), want) {
			t.Errorf("report missing %q:\n%s", want, contents)
		}
	}
}

func TestWriteMarkdownReportIncludesOutputPathInError(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "missing", "audit.md")
	err := writeMarkdownReport(outputPath, "app", auditor.NewPackageStore(), auditor.NewEdgeStore(), &auditor.Report{})
	if err == nil || !strings.Contains(err.Error(), outputPath) {
		t.Fatalf("error: got %v, want path %q", err, outputPath)
	}
}

func TestFinalizeAuditDoesNotWriteAfterShutdownFailure(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "audit.md")
	report, err := finalizeAudit(errors.New("shutdown timeout"), outputPath, "app", auditor.NewPackageStore(), auditor.NewEdgeStore())
	if err == nil || report != nil {
		t.Fatalf("finalizeAudit: report=%v error=%v, want nil report and shutdown error", report, err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("output file exists after shutdown failure: %v", statErr)
	}
}

func TestCLIOutputContract(t *testing.T) {
	tempDir := t.TempDir()
	packagePath := filepath.Join(tempDir, "package.json")
	if err := os.WriteFile(packagePath, []byte(`{"name":"empty-app","dependencies":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pyprojectPath := filepath.Join(tempDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte("[project]\nname = \"empty-python-app\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("no output option creates no file", func(t *testing.T) {
		unusedOutput := filepath.Join(tempDir, "not-created.md")
		result := runCLIProcess(t, packagePath)
		if result.err != nil {
			t.Fatalf("CLI error: %v\nstderr: %s", result.err, result.stderr)
		}
		if _, err := os.Stat(unusedOutput); !os.IsNotExist(err) {
			t.Fatalf("unexpected output file: %v", err)
		}
		if !strings.Contains(result.stdout, "No dependencies found") {
			t.Fatalf("existing terminal output missing: %q", result.stdout)
		}
	})

	t.Run("output option writes markdown", func(t *testing.T) {
		outputPath := filepath.Join(tempDir, "audit-report")
		result := runCLIProcess(t, "--output", outputPath, packagePath)
		if result.err != nil {
			t.Fatalf("CLI error: %v\nstderr: %s", result.err, result.stderr)
		}
		contents, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), "# Dependency Audit Report") {
			t.Fatalf("invalid Markdown report:\n%s", contents)
		}
	})

	t.Run("empty Python project reports deterministic target without PostgreSQL", func(t *testing.T) {
		outputPath := filepath.Join(tempDir, "python-audit.md")
		result := runCLIProcess(t,
			"--ecosystem", "python",
			"--python-version", "3.11",
			"--python-platform", "darwin",
			"--output", outputPath,
			pyprojectPath,
		)
		if result.err != nil {
			t.Fatalf("CLI error: %v\nstderr: %s", result.err, result.stderr)
		}
		if !strings.Contains(result.stdout, "Python target: 3.11 on darwin") || !strings.Contains(result.stdout, "No dependencies found in pyproject.toml") {
			t.Fatalf("stdout: %q", result.stdout)
		}
		contents, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"- Root: `empty-python-app`", "- Python platform: `darwin`", "- Python version: `3.11`"} {
			if !strings.Contains(string(contents), want) {
				t.Errorf("Python report missing %q:\n%s", want, contents)
			}
		}
	})

	t.Run("missing output value exits non-zero", func(t *testing.T) {
		result := runCLIProcess(t, "--output")
		if result.err == nil || !strings.Contains(result.stderr, "flag needs an argument") {
			t.Fatalf("error=%v stderr=%q, want non-zero missing-value error", result.err, result.stderr)
		}
	})

	t.Run("unwritable output exits non-zero with path", func(t *testing.T) {
		outputPath := filepath.Join(tempDir, "missing", "audit.md")
		result := runCLIProcess(t, "--output", outputPath, packagePath)
		if result.err == nil || !strings.Contains(result.stderr, outputPath) {
			t.Fatalf("error=%v stderr=%q, want non-zero path-specific error", result.err, result.stderr)
		}
	})
}

type cliProcessResult struct {
	stdout string
	stderr string
	err    error
}

func runCLIProcess(t *testing.T, args ...string) cliProcessResult {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	commandArgs := append([]string{"-test.run=^TestCLIHelperProcess$", "--"}, args...)
	command := exec.Command(executable, commandArgs...)
	command.Env = append(os.Environ(), "AUDITOR_CLI_HELPER=1")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	return cliProcessResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("AUDITOR_CLI_HELPER") != "1" {
		return
	}

	separator := 0
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	os.Args = append([]string{"auditor"}, os.Args[separator+1:]...)
	main()
}

func TestShutdownTimeoutFromEnvUsesDefault(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	got, err := shutdownTimeoutFromEnv()
	if err != nil {
		t.Fatalf("shutdownTimeoutFromEnv: %v", err)
	}
	if got != 30*time.Second {
		t.Fatalf("timeout: got %v, want 30s", got)
	}
}

func TestShutdownTimeoutFromEnvParsesDuration(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "45s")

	got, err := shutdownTimeoutFromEnv()
	if err != nil {
		t.Fatalf("shutdownTimeoutFromEnv: %v", err)
	}
	if got != 45*time.Second {
		t.Fatalf("timeout: got %v, want 45s", got)
	}
}

func TestShutdownTimeoutFromEnvRejectsInvalidDuration(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "later")

	_, err := shutdownTimeoutFromEnv()
	if err == nil || !strings.Contains(err.Error(), "SHUTDOWN_TIMEOUT") {
		t.Fatalf("error: got %v, want clear SHUTDOWN_TIMEOUT error", err)
	}
}

func TestShutdownTimeoutFromEnvRejectsNonPositiveDuration(t *testing.T) {
	for _, value := range []string{"0s", "-1s"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("SHUTDOWN_TIMEOUT", value)

			_, err := shutdownTimeoutFromEnv()
			if err == nil || !strings.Contains(err.Error(), "must be positive") {
				t.Fatalf("error: got %v, want positive-duration error", err)
			}
		})
	}
}
