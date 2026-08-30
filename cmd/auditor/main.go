package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/depfile"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/dlq"
	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/job"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/pypi"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/queue"
	storepg "github.com/thomxsnguyen/mini-distributed-job-api/internal/store/postgres"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/worker"
)

const defaultShutdownTimeout = 30 * time.Second

type cliConfig struct {
	input        string
	outputPath   string
	ecosystem    string
	ref          string
	manifestPath string
	pythonTarget pypi.Target
	repository   githubsource.Repository
	isGitHub     bool
}

type ManifestSource struct {
	Location string
	Data     []byte
}

func main() {
	// 1. Read a local dependency manifest path or GitHub repository URL.
	config, err := parseCLIArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration: %v\n", err)
		fmt.Fprintln(os.Stderr, "usage: auditor [--ecosystem <npm|python|go>] [--output <path>] [--ref <value>] [--manifest <path>] <manifest-path-or-github-url>")
		os.Exit(1)
	}
	shutdownTimeout, err := shutdownTimeoutFromEnv()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}

	// 2. Read and parse the manifest before opening PostgreSQL.
	githubClient := &githubsource.GitHubClient{Token: os.Getenv("GITHUB_TOKEN")}
	source, err := readManifestSource(context.Background(), config, githubClient)
	if err != nil {
		log.Fatalf("manifest source: %v", err)
	}
	manifest, err := parseManifest(config, source)
	if err != nil {
		log.Fatalf("parse manifest from %s: %v", source.Location, err)
	}
	deps := manifest.Dependencies
	rootName := manifest.Name
	if rootName == "" {
		rootName = source.Location
	}
	if len(deps) == 0 {
		if config.outputPath != "" {
			packages := auditor.NewPackageStore()
			edges := auditor.NewEdgeStore()
			report := auditor.GenerateReport(packages, edges, rootName)
			if err := writeMarkdownReport(config.outputPath, rootName, packages, edges, report, reportMetadata(config)); err != nil {
				log.Fatal(err)
			}
		}
		printPythonTarget(config)
		fmt.Printf("No dependencies found in %s — nothing to audit.\n", filepath.Base(config.manifestPath))
		return
	}

	// 3. Version ranges are resolved lazily by the registry client when each
	//    job is processed — no pre-flight resolution is needed here.

	// 4. Create infrastructure.
	workCtx := context.Background()
	signalCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required (see .env.example)")
	}
	dbPool, err := pgxpool.New(workCtx, databaseURL)
	if err != nil {
		log.Fatalf("postgres: open pool: %v", err)
	}
	defer dbPool.Close()
	if err := dbPool.Ping(workCtx); err != nil {
		log.Fatalf("postgres: ping: %v", err)
	}

	jobStore := storepg.New(dbPool)
	const bufferSize = 100
	q := queue.New(bufferSize, jobStore)
	deadLetters := dlq.New(jobStore)

	pkgStore := auditor.NewPackageStore()
	edgeStore := auditor.NewEdgeStore()
	reg := registryForConfig(config)
	policy := auditor.LicensePolicy{}
	handler := auditor.NewAuditHandler(reg, policy, pkgStore, edgeStore)

	// 5. Seed the queue: one job per direct dependency.
	const poolSize = 10
	pool := worker.NewWithOptions(poolSize, q, handler,
		worker.WithStore(jobStore),
		worker.WithDLQ(deadLetters),
	)

	for _, d := range deps {
		seedJob, err := newSeedJob(rootName, d)
		if err != nil {
			log.Fatalf("marshal seed job for %s: %v", d.Name, err)
		}
		pool.Submit(seedJob)
	}

	// 6 & 7. Start the worker pool.
	pool.Start(workCtx)

	// 8. Wait for normal completion or a shutdown signal. The signal context is
	// deliberately not passed to the pool, so active handlers keep their work
	// context while the pool drains.
	select {
	case <-pool.Done():
	case <-signalCtx.Done():
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	shutdownErr := pool.Shutdown(shutdownCtx)
	report, err := finalizeAudit(shutdownErr, config.outputPath, rootName, pkgStore, edgeStore, reportMetadata(config))
	if shutdownErr != nil {
		log.Printf("shutdown deadline %s exceeded: %v", shutdownTimeout, shutdownErr)
		os.Exit(1)
	}
	if err != nil {
		log.Fatal(err)
	}

	// 9 & 10. Print the report generated after successful shutdown.
	printPythonTarget(config)
	fmt.Print(report.Summary)
}

func newSeedJob(rootName string, dependency depfile.Dependency) (job.Job, error) {
	payload, err := json.Marshal(auditor.AuditPayload{
		Name:       dependency.Name,
		Version:    dependency.VersionRange,
		ParentName: rootName,
	})
	if err != nil {
		return job.Job{}, err
	}
	return job.NewJob("audit_package", payload), nil
}

func finalizeAudit(shutdownErr error, outputPath, root string, packages *auditor.PackageStore, edges *auditor.EdgeStore, metadata ...map[string]string) (*auditor.Report, error) {
	if shutdownErr != nil {
		return nil, shutdownErr
	}

	report := auditor.GenerateReport(packages, edges, root)
	if outputPath != "" {
		if err := writeMarkdownReport(outputPath, root, packages, edges, report, metadata...); err != nil {
			return nil, err
		}
	}
	return report, nil
}

func parseCLIArgs(args []string) (cliConfig, error) {
	flags := flag.NewFlagSet("auditor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	outputPath := flags.String("output", "", "write a Markdown report to this path")
	ecosystem := flags.String("ecosystem", "npm", "dependency ecosystem: npm, python, or go")
	ref := flags.String("ref", "", "GitHub branch, tag, or commit")
	manifestPath := flags.String("manifest", "package.json", "repository-relative dependency manifest path")
	pythonVersion := flags.String("python-version", "3.12", "Python target version for dependency markers")
	pythonPlatform := flags.String("python-platform", "linux", "Python target platform: linux, windows, or darwin")
	if err := flags.Parse(args); err != nil {
		return cliConfig{}, err
	}

	setOptions := make(map[string]bool)
	flags.Visit(func(current *flag.Flag) {
		setOptions[current.Name] = true
	})
	if setOptions["output"] && *outputPath == "" {
		return cliConfig{}, errors.New("--output requires a non-empty path")
	}
	if setOptions["ref"] && *ref == "" {
		return cliConfig{}, errors.New("--ref requires a non-empty value")
	}
	if *manifestPath == "" {
		return cliConfig{}, errors.New("--manifest requires a non-empty path")
	}
	if *ecosystem != "npm" && *ecosystem != "python" && *ecosystem != "go" {
		return cliConfig{}, fmt.Errorf("--ecosystem must be npm, python, or go, got %q", *ecosystem)
	}
	if *ecosystem != "python" && (setOptions["python-version"] || setOptions["python-platform"]) {
		return cliConfig{}, errors.New("--python-version and --python-platform are valid only with --ecosystem python")
	}

	positional := flags.Args()
	if len(positional) == 0 {
		return cliConfig{}, errors.New("missing manifest path or GitHub repository URL")
	}
	if len(positional) > 1 {
		return cliConfig{}, fmt.Errorf("unexpected extra positional argument %q", positional[1])
	}

	config := cliConfig{input: positional[0], outputPath: *outputPath, ecosystem: *ecosystem, manifestPath: *manifestPath}
	if config.ecosystem == "python" {
		target, err := pypi.NewTarget(*pythonVersion, *pythonPlatform)
		if err != nil {
			return cliConfig{}, err
		}
		config.pythonTarget = target
	}
	if strings.Contains(config.input, "://") {
		repository, err := githubsource.ParseRepositoryURL(config.input)
		if err != nil {
			return cliConfig{}, err
		}
		if err := githubsource.ValidateManifestPath(*manifestPath); err != nil {
			return cliConfig{}, fmt.Errorf("--manifest: %w", err)
		}
		switch config.ecosystem {
		case "python":
			if !setOptions["manifest"] {
				return cliConfig{}, errors.New("--manifest is required for Python GitHub input")
			}
			if !isPythonManifest(filepath.Base(*manifestPath)) {
				return cliConfig{}, fmt.Errorf("unsupported Python manifest %q", filepath.Base(*manifestPath))
			}
		case "go":
			if !setOptions["manifest"] {
				return cliConfig{}, errors.New("--manifest is required for Go GitHub input")
			}
			if filepath.Base(*manifestPath) != "go.mod" {
				return cliConfig{}, errors.New("Go input requires a go.mod manifest")
			}
		case "npm":
			if filepath.Base(*manifestPath) != "package.json" {
				return cliConfig{}, errors.New("npm input requires a package.json manifest")
			}
		}
		config.ref = *ref
		config.repository = repository
		config.isGitHub = true
		return config, nil
	}
	if setOptions["ref"] || setOptions["manifest"] {
		return cliConfig{}, errors.New("--ref and --manifest are valid only with GitHub repository input")
	}
	if config.ecosystem == "python" {
		config.manifestPath = filepath.Base(config.input)
		if !isPythonManifest(config.manifestPath) {
			return cliConfig{}, fmt.Errorf("unsupported Python manifest %q", config.manifestPath)
		}
	} else if config.ecosystem == "go" {
		config.manifestPath = filepath.Base(config.input)
		if config.manifestPath != "go.mod" {
			return cliConfig{}, errors.New("Go input requires a local file named go.mod")
		}
	}
	return config, nil
}

func isPythonManifest(name string) bool {
	return name == "pyproject.toml" || name == "requirements.txt"
}

func parseManifest(config cliConfig, source ManifestSource) (depfile.Manifest, error) {
	reader := bytes.NewReader(source.Data)
	manifestName := filepath.Base(config.manifestPath)
	switch config.ecosystem {
	case "npm":
		if manifestName != "package.json" {
			return depfile.Manifest{}, fmt.Errorf("unsupported npm manifest %q", manifestName)
		}
		return depfile.ParsePackageJSON(reader, true)
	case "python":
		switch manifestName {
		case "pyproject.toml":
			return depfile.ParsePyProject(reader, config.pythonTarget)
		case "requirements.txt":
			return depfile.ParseRequirements(reader, requirementsRoot(config), config.pythonTarget)
		default:
			return depfile.Manifest{}, fmt.Errorf("unsupported Python manifest %q", manifestName)
		}
	case "go":
		if manifestName != "go.mod" {
			return depfile.Manifest{}, fmt.Errorf("unsupported Go manifest %q", manifestName)
		}
		manifest, err := depfile.ParseGoMod(reader)
		if err != nil {
			return depfile.Manifest{}, err
		}
		return manifest.Manifest, nil
	default:
		return depfile.Manifest{}, fmt.Errorf("unsupported ecosystem %q", config.ecosystem)
	}
}

func requirementsRoot(config cliConfig) string {
	if config.isGitHub {
		return config.repository.Name
	}
	absolute, err := filepath.Abs(config.input)
	if err != nil {
		return filepath.Base(filepath.Dir(config.input))
	}
	return filepath.Base(filepath.Dir(absolute))
}

func registryForConfig(config cliConfig) auditor.RegistryClient {
	if config.ecosystem == "python" {
		return pypi.NewClient(config.pythonTarget)
	}
	return auditor.NewNpmClient()
}

func reportMetadata(config cliConfig) map[string]string {
	if config.ecosystem != "python" {
		return nil
	}
	return map[string]string{
		"Python version":  config.pythonTarget.PythonVersion,
		"Python platform": config.pythonTarget.Platform,
	}
}

func printPythonTarget(config cliConfig) {
	if config.ecosystem == "python" {
		fmt.Printf("Python target: %s on %s\n", config.pythonTarget.PythonVersion, config.pythonTarget.Platform)
	}
}

func readManifestSource(ctx context.Context, config cliConfig, githubClient *githubsource.GitHubClient) (ManifestSource, error) {
	if config.isGitHub {
		data, err := githubClient.FetchManifest(ctx, config.repository, config.manifestPath, config.ref)
		if err != nil {
			return ManifestSource{}, err
		}
		return ManifestSource{Location: config.input + "/" + config.manifestPath, Data: data}, nil
	}

	data, err := os.ReadFile(config.input)
	if err != nil {
		return ManifestSource{}, fmt.Errorf("read local manifest %q: %w", config.input, err)
	}
	return ManifestSource{Location: config.input, Data: data}, nil
}

func writeMarkdownReport(outputPath, root string, packages *auditor.PackageStore, edges *auditor.EdgeStore, report *auditor.Report, metadata ...map[string]string) error {
	reportMetadata := map[string]string(nil)
	if len(metadata) > 0 {
		reportMetadata = metadata[0]
	}
	markdown, err := auditor.GenerateMarkdownReport(auditor.MarkdownReportInput{
		Root:     root,
		Packages: packages.All(),
		Edges:    edges.All(),
		Report:   report,
		Metadata: reportMetadata,
	})
	if err != nil {
		return fmt.Errorf("generate Markdown report for %q: %w", outputPath, err)
	}
	if err := os.WriteFile(outputPath, []byte(markdown), 0o644); err != nil {
		return fmt.Errorf("write Markdown report %q: %w", outputPath, err)
	}
	return nil
}

func shutdownTimeoutFromEnv() (time.Duration, error) {
	value := os.Getenv("SHUTDOWN_TIMEOUT")
	if value == "" {
		return defaultShutdownTimeout, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("SHUTDOWN_TIMEOUT %q is not a valid duration: %w", value, err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive, got %q", value)
	}
	return timeout, nil
}
