package analysis

import (
	"fmt"
	"testing"
	"time"

	"ci-artifact-identity/internal/workflow"
)

func generateSyntheticWorkflow(index int, numJobs int) *workflow.Workflow {
	jobs := make(map[string]*workflow.Job, numJobs)
	jobOrder := make([]string, 0, numJobs)

	// Build job
	buildID := "build"
	jobOrder = append(jobOrder, buildID)
	jobs[buildID] = &workflow.Job{
		ID:     buildID,
		Name:   "Build Job",
		RunsOn: "ubuntu-latest",
		Steps: []*workflow.Step{
			{
				Index: 0,
				Run:   "npm run build",
			},
			{
				Index:       1,
				Uses:        "actions/upload-artifact@v4",
				ActionOwner: "actions",
				ActionName:  "upload-artifact",
				ActionRef:   "v4",
				With: map[string]interface{}{
					"name": fmt.Sprintf("artifact-%d", index),
					"path": "dist/",
				},
			},
		},
	}

	prevJob := buildID
	// Validation jobs
	for j := 1; j < numJobs-1; j++ {
		testID := fmt.Sprintf("test-%d", j)
		jobOrder = append(jobOrder, testID)
		jobs[testID] = &workflow.Job{
			ID:     testID,
			Name:   fmt.Sprintf("Test Job %d", j),
			RunsOn: "ubuntu-latest",
			Needs:  []string{prevJob},
			Steps: []*workflow.Step{
				{
					Index:       0,
					Uses:        "actions/download-artifact@v4",
					ActionOwner: "actions",
					ActionName:  "download-artifact",
					ActionRef:   "v4",
					With: map[string]interface{}{
						"name": fmt.Sprintf("artifact-%d", index),
					},
				},
				{
					Index: 1,
					Run:   "npm test",
				},
			},
		}
		prevJob = testID
	}

	// Deploy job
	deployID := "deploy"
	jobOrder = append(jobOrder, deployID)
	jobs[deployID] = &workflow.Job{
		ID:     deployID,
		Name:   "Deploy Job",
		RunsOn: "ubuntu-latest",
		Needs:  []string{prevJob},
		Steps: []*workflow.Step{
			{
				Index:       0,
				Uses:        "actions/download-artifact@v4",
				ActionOwner: "actions",
				ActionName:  "download-artifact",
				ActionRef:   "v4",
				With: map[string]interface{}{
					"name": fmt.Sprintf("artifact-%d", index),
				},
			},
			{
				Index: 1,
				Run:   "npm run deploy",
			},
		},
	}

	return &workflow.Workflow{
		Path:     fmt.Sprintf(".github/workflows/wf_%d.yml", index),
		Name:     fmt.Sprintf("Synthetic Workflow %d", index),
		Jobs:     jobs,
		JobOrder: jobOrder,
	}
}

func TestAnalyzeWorkflows_SyntheticStress100Workflows(t *testing.T) {
	numWorkflows := 100
	numJobs := 50

	workflows := make([]*workflow.Workflow, numWorkflows)
	for i := 0; i < numWorkflows; i++ {
		workflows[i] = generateSyntheticWorkflow(i, numJobs)
	}

	start := time.Now()
	res, err := AnalyzeWorkflows("/synthetic/repo", workflows)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("AnalyzeWorkflows failed: %v", err)
	}

	if res.Summary.WorkflowsAnalyzed != numWorkflows {
		t.Errorf("expected %d workflows analyzed, got %d", numWorkflows, res.Summary.WorkflowsAnalyzed)
	}
	if res.Summary.ArtifactChains != numWorkflows {
		t.Errorf("expected %d established artifact chains, got %d", numWorkflows, res.Summary.ArtifactChains)
	}
	if res.Summary.Failures != 0 {
		t.Errorf("expected 0 failures, got %d", res.Summary.Failures)
	}

	t.Logf("Analyzed %d workflows with %d jobs each (total %d jobs) in %v",
		numWorkflows, numJobs, numWorkflows*numJobs, elapsed)

	if elapsed > 3*time.Second {
		t.Errorf("performance regression: analysis took %v (expected < 3s)", elapsed)
	}
}

func BenchmarkAnalyzeWorkflows_100Workflows_50Jobs(b *testing.B) {
	numWorkflows := 100
	numJobs := 50

	workflows := make([]*workflow.Workflow, numWorkflows)
	for i := 0; i < numWorkflows; i++ {
		workflows[i] = generateSyntheticWorkflow(i, numJobs)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := AnalyzeWorkflows("/synthetic/repo", workflows)
		if err != nil {
			b.Fatalf("benchmark failed: %v", err)
		}
	}
}
