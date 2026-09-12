# Contributing to Artifact-ID

Thank you for your interest in contributing to `artifact-id`!

## Philosophy & Principles

1. **False-Pass-Resistant by Design**: Avoiding false passes is our core design goal. If static analysis cannot determine that the tested bytes are the deployed bytes, report `UNVERIFIED` (`AI003`) with a machine-readable reason code.
2. **Minimal Dependencies**: We rely solely on the Go standard library and `gopkg.in/yaml.v3` for parsing YAML. We avoid heavy frameworks, external network dependencies, or runtime CI emulators.
3. **Deterministic Output**: All CLI outputs (terminal text and JSON) must be bit-for-bit deterministic across repeated runs and operating systems.
4. **Focused Scope**: `artifact-id` specifically analyzes whether the artifact deployed is the artifact validated. It does not perform security auditing, secret scanning, or general CI linting.

## Development Setup

Requirements:
- Go 1.22 or higher

### Running Tests

```bash
# Run all unit and integration tests
go test -v ./...

# Run tests with race detector
go test -v -race ./...

# Run fuzz tests
go test -fuzz=FuzzParseWorkflowContent -fuzztime=5s ./internal/workflow
go test -fuzz=FuzzNormalizeExpression -fuzztime=5s ./internal/analysis
go test -fuzz=FuzzCommandClassifier -fuzztime=5s ./internal/analysis

# Run stress and benchmark tests
go test -bench=. ./internal/analysis
```

### Code Style

- Format all Go source files with standard `gofmt`:
  ```bash
  gofmt -w .
  ```
- Run static checks:
  ```bash
  go vet ./...
  ```

### Adding New Test Fixtures

When adding support for a new workflow pattern or fixing an edge case:
1. Create a minimal YAML workflow fixture in `testdata/workflows/`.
2. Add a corresponding test case in `cmd/artifact-id/main_test.go` or `internal/analysis/analyzer_test.go`.
3. Verify both text and JSON outputs meet determinism requirements.
