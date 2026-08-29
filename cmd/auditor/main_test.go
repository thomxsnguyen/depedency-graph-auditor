package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/depfile"
	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
)

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
		{name: "input only", args: []string{"package.json"}, want: cliConfig{input: "package.json", manifestPath: "package.json"}},
		{name: "with output", args: []string{"--output", "report.md", "package.json"}, want: cliConfig{input: "package.json", outputPath: "report.md", manifestPath: "package.json"}},
		{
			name: "GitHub input with options",
			args: []string{"--ref", "development", "--manifest", "packages/web/package.json", "https://github.com/acme/widget.git/"},
			want: cliConfig{
				input: "https://github.com/acme/widget.git/", ref: "development", manifestPath: "packages/web/package.json",
				repository: githubsource.Repository{Owner: "acme", Name: "widget"}, isGitHub: true,
			},
		},
		{name: "missing input", wantErr: "missing package.json path or GitHub repository URL"},
		{name: "missing output value", args: []string{"--output"}, wantErr: "flag needs an argument"},
		{name: "empty output value", args: []string{"--output=", "package.json"}, wantErr: "non-empty path"},
		{name: "empty ref", args: []string{"--ref=", "https://github.com/acme/widget"}, wantErr: "--ref requires a non-empty value"},
		{name: "empty manifest", args: []string{"--manifest=", "https://github.com/acme/widget"}, wantErr: "--manifest requires a non-empty path"},
		{name: "ref on local input", args: []string{"--ref", "main", "package.json"}, wantErr: "valid only with GitHub"},
		{name: "manifest on local input", args: []string{"--manifest", "web/package.json", "package.json"}, wantErr: "valid only with GitHub"},
		{name: "invalid GitHub scheme", args: []string{"http://github.com/acme/widget"}, wantErr: "must use https"},
		{name: "invalid GitHub host", args: []string{"https://example.com/acme/widget"}, wantErr: "host must be github.com"},
		{name: "invalid manifest traversal", args: []string{"--manifest", "../package.json", "https://github.com/acme/widget"}, wantErr: "invalid segment"},
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
