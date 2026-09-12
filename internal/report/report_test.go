package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"ci-artifact-identity/internal/analysis"
)

func sampleResult() *analysis.AnalysisResult {
	stepIdx := 1
	return &analysis.AnalysisResult{
		SchemaVersion: "1",
		ToolVersion:   "0.1.0",
		Repository:    "/mock/repo",
		WorkflowCount: 1,
		Workflows: []*analysis.WorkflowReport{
			{
				Path:   ".github/workflows/ci.yml",
				Name:   "CI",
				Status: "PASS",
				Jobs:   []string{"build", "test", "deploy"},
				ArtifactFlows: []*analysis.ArtifactFlow{
					{
						Workflow:     ".github/workflows/ci.yml",
						ArtifactName: "web-dist",
						Producer: &analysis.ArtifactProducer{
							JobID: "build",
						},
						Validators: []*analysis.ArtifactConsumer{
							{JobID: "test"},
						},
						Deployers: []*analysis.ArtifactConsumer{
							{JobID: "deploy"},
						},
						Status: analysis.FlowEstablished,
					},
				},
				Findings: []*analysis.Finding{
					{
						RuleID:       analysis.RuleArtifactChainEstablished,
						Status:       analysis.StatusPass,
						WorkflowFile: ".github/workflows/ci.yml",
						JobID:        "deploy",
						StepIndex:    &stepIdx,
						Message:      "Artifact chain established.",
					},
				},
			},
		},
		ArtifactFlows: []*analysis.ArtifactFlow{
			{
				Workflow:     ".github/workflows/ci.yml",
				ArtifactName: "web-dist",
				Status:       analysis.FlowEstablished,
			},
		},
		Findings: []*analysis.Finding{},
		Summary: analysis.Summary{
			WorkflowsAnalyzed: 1,
			ArtifactChains:    1,
			Failures:          0,
			Warnings:          0,
			Unverified:        0,
		},
	}
}

func TestRenderText(t *testing.T) {
	var buf bytes.Buffer
	res := sampleResult()

	if err := RenderText(&buf, res); err != nil {
		t.Fatalf("unexpected error rendering text: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Artifact-ID 0.1.0") {
		t.Errorf("expected header 'Artifact-ID 0.1.0', got:\n%s", out)
	}
	if !strings.Contains(out, "web-dist") {
		t.Errorf("expected artifact name 'web-dist', got:\n%s", out)
	}
	if !strings.Contains(out, "Artifact identity established") {
		t.Errorf("expected 'Artifact identity established', got:\n%s", out)
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	res := sampleResult()

	if err := RenderJSON(&buf, res); err != nil {
		t.Fatalf("unexpected error rendering JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("rendered output is not valid JSON: %v\nOutput:\n%s", err, buf.String())
	}

	if parsed["schemaVersion"] != "1" {
		t.Errorf("expected schemaVersion '1', got %v", parsed["schemaVersion"])
	}
	if parsed["toolVersion"] != "0.1.0" {
		t.Errorf("expected toolVersion '0.1.0', got %v", parsed["toolVersion"])
	}
	if parsed["workflowCount"] != float64(1) {
		t.Errorf("expected workflowCount 1, got %v", parsed["workflowCount"])
	}
}
