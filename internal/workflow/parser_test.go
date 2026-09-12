package workflow

import (
	"testing"
)

func TestParseWorkflowContent_Success(t *testing.T) {
	yamlContent := `
name: Build and Test
on:
  push:
    branches: [main]
  pull_request:
jobs:
  build:
    name: Build Job
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Compile
        run: npm run build
      - name: Upload
        uses: actions/upload-artifact@v4
        with:
          name: dist
          path: build/
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: dist
      - run: npm test
`
	wf, err := ParseWorkflowContent("test.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wf.Name != "Build and Test" {
		t.Errorf("expected workflow name 'Build and Test', got %q", wf.Name)
	}

	if len(wf.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(wf.Jobs))
	}

	buildJob := wf.Jobs["build"]
	if buildJob == nil {
		t.Fatal("expected 'build' job")
	}
	if len(buildJob.Steps) != 3 {
		t.Fatalf("expected 3 steps in build job, got %d", len(buildJob.Steps))
	}
	if buildJob.Steps[1].Run != "npm run build" {
		t.Errorf("expected step 1 run 'npm run build', got %q", buildJob.Steps[1].Run)
	}

	testJob := wf.Jobs["test"]
	if testJob == nil {
		t.Fatal("expected 'test' job")
	}
	if len(testJob.Needs) != 1 || testJob.Needs[0] != "build" {
		t.Errorf("expected test job needs [build], got %v", testJob.Needs)
	}
}

func TestParseWorkflowContent_NeedsFormats(t *testing.T) {
	yamlScalar := `
jobs:
  a:
    runs-on: ubuntu-latest
  b:
    needs: a
    runs-on: ubuntu-latest
`
	wf1, err := ParseWorkflowContent("scalar.yml", []byte(yamlScalar))
	if err != nil {
		t.Fatalf("scalar needs failed: %v", err)
	}
	if len(wf1.Jobs["b"].Needs) != 1 || wf1.Jobs["b"].Needs[0] != "a" {
		t.Errorf("expected b.needs == ['a'], got %v", wf1.Jobs["b"].Needs)
	}

	yamlList := `
jobs:
  a:
    runs-on: ubuntu-latest
  b:
    runs-on: ubuntu-latest
  c:
    needs:
      - b
      - a
    runs-on: ubuntu-latest
`
	wf2, err := ParseWorkflowContent("list.yml", []byte(yamlList))
	if err != nil {
		t.Fatalf("list needs failed: %v", err)
	}
	// Needs are sorted deterministically
	if len(wf2.Jobs["c"].Needs) != 2 || wf2.Jobs["c"].Needs[0] != "a" || wf2.Jobs["c"].Needs[1] != "b" {
		t.Errorf("expected sorted c.needs == ['a', 'b'], got %v", wf2.Jobs["c"].Needs)
	}
}

func TestParseWorkflowContent_YAMLQuirks_OnBoolean(t *testing.T) {
	// In YAML 1.1, 'on:' can be misparsed as boolean true.
	// Our parser must handle this correctly without breaking.
	yamlOn := `
name: Quirky
on: push
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo ok
`
	wf, err := ParseWorkflowContent("quirk.yml", []byte(yamlOn))
	if err != nil {
		t.Fatalf("failed to parse workflow with on: push: %v", err)
	}
	if wf.Name != "Quirky" {
		t.Errorf("expected name 'Quirky', got %q", wf.Name)
	}
}

func TestParseWorkflowContent_MatrixStrategy(t *testing.T) {
	yamlMatrix := `
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
        node: [18, 20]
    runs-on: ${{ matrix.os }}
    steps:
      - run: npm test
`
	wf, err := ParseWorkflowContent("matrix.yml", []byte(yamlMatrix))
	if err != nil {
		t.Fatalf("failed to parse matrix workflow: %v", err)
	}
	job := wf.Jobs["test"]
	if !job.HasMatrix {
		t.Error("expected HasMatrix = true")
	}
	if len(job.MatrixParams) != 2 {
		t.Errorf("expected 2 matrix parameters, got %d", len(job.MatrixParams))
	}
}

func TestParseWorkflowContent_MalformedYAML(t *testing.T) {
	yamlBad := `
name: broken
jobs: [broken
`
	_, err := ParseWorkflowContent("broken.yml", []byte(yamlBad))
	if err == nil {
		t.Fatal("expected error parsing malformed YAML, got nil")
	}
}

func TestParseWorkflowFile(t *testing.T) {
	wf, err := ParseWorkflowFile("../../testdata/workflows/04_good_flow.yml")
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}
	if wf.Name != "Good Flow" {
		t.Errorf("expected Good Flow, got %s", wf.Name)
	}

	// Non-existent file
	_, err = ParseWorkflowFile("nonexistent-file.yml")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func FuzzParseWorkflowContent(f *testing.F) {
	f.Add([]byte("name: test\njobs:\n  build:\n    runs-on: ubuntu-latest\n"))
	f.Add([]byte("name: broken\njobs: [broken\n"))
	f.Add([]byte("<<: *defaults\n"))
	f.Add([]byte("on: push\njobs:\n  a:\n    needs: [b, c]\n"))
	f.Add([]byte(""))
	f.Add([]byte("---"))
	f.Add([]byte("jobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/upload-artifact@v4\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// ParseWorkflowContent must never panic on arbitrary input
		_, _ = ParseWorkflowContent("fuzz.yml", data)
	})
}
