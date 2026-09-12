package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findRootDir() string {
	if _, err := os.Stat("testdata"); err == nil {
		return "."
	}
	if _, err := os.Stat("../../testdata"); err == nil {
		return "../.."
	}
	return "."
}

func testPath(rel string) string {
	return filepath.Join(findRootDir(), rel)
}

func TestCLI_Version(t *testing.T) {
	code := run([]string{"--version"})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for --version, got %d", ExitSuccess, code)
	}
}

func TestCLI_Help(t *testing.T) {
	code := run([]string{"--help"})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for --help, got %d", ExitSuccess, code)
	}
}

func TestCLI_InvalidArgs(t *testing.T) {
	// Invalid format
	code := run([]string{"--format", "xml", "."})
	if code != ExitInvocation {
		t.Errorf("expected exit code %d for invalid format, got %d", ExitInvocation, code)
	}

	// Non-existent repo
	code = run([]string{"/nonexistent/path/for/sure"})
	if code != ExitInvocation {
		t.Errorf("expected exit code %d for non-existent repo, got %d", ExitInvocation, code)
	}

	// Missing workflow file
	code = run([]string{"--workflow", testPath("testdata/workflows/nonexistent.yml")})
	if code != ExitInvocation {
		t.Errorf("expected exit code %d for missing workflow, got %d", ExitInvocation, code)
	}
}

func TestCLI_Fixture01_NoWorkflows(t *testing.T) {
	dir := testPath("testdata/repos/no_workflows")
	_ = os.MkdirAll(dir, 0755)
	code := run([]string{dir})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for empty repo, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture02_SimpleBuildOnly(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/02_simple_build_only.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for simple build only, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture03_BuildTestNoDeploy(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/03_build_test_no_deploy.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for build+test no deploy, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture04_GoodFlow(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/04_good_flow.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for good flow, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture05_DownstreamRebuild(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/05_downstream_rebuild.yml")})
	if code != ExitIdentIssue {
		t.Errorf("expected exit code %d for downstream rebuild (AI001), got %d", ExitIdentIssue, code)
	}
}

func TestCLI_Fixture06_MismatchedArtifacts(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/06_mismatched_artifacts.yml")})
	if code != ExitIdentIssue {
		t.Errorf("expected exit code %d for artifact mismatch (AI002), got %d", ExitIdentIssue, code)
	}
}

func TestCLI_Fixture07_SymbolicIdentical(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/07_symbolic_identical.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for symbolic identical artifact, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture08_SymbolicUnresolved(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/08_symbolic_unresolved.yml")})
	if code != ExitIdentIssue {
		t.Errorf("expected exit code %d for symbolic unresolved (AI003), got %d", ExitIdentIssue, code)
	}
}

func TestCLI_Fixture09_MultipleArtifacts(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/09_multiple_artifacts.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for multiple artifacts good flow, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture10_MultipleWorkflows(t *testing.T) {
	code := run([]string{testPath("testdata/repos/multiple_workflows")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for multiple workflows repo, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture11_JobNeedsScalar(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/11_job_needs_scalar.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for scalar job needs, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture12_JobNeedsList(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/12_job_needs_list.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for list job needs, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture13_MalformedYAML(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/13_malformed_yaml.yml")})
	if code != ExitInvocation {
		t.Errorf("expected exit code %d for malformed YAML, got %d", ExitInvocation, code)
	}
}

func TestCLI_Fixture14_UploadNoName(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/14_upload_no_name.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for default upload name, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture15_DownloadAll(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/15_download_all.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for download all artifacts, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture16_DockerGoodFlow(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/16_docker_good_flow.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for docker digest good flow, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture17_DockerMutableTag(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/17_docker_mutable_tag.yml")})
	if code != ExitIdentIssue {
		t.Errorf("expected exit code %d for docker mutable tag warning (AI005), got %d", ExitIdentIssue, code)
	}
}

func TestCLI_Fixture18_JobMatrix(t *testing.T) {
	// In 18_job_matrix.yml, test downloads dist-${{ matrix.os }} and deploy downloads dist-ubuntu-latest.
	// Because matrix is unresolved statically in comparison, it produces UNVERIFIED (exit code 1).
	code := run([]string{"--workflow", testPath("testdata/workflows/18_job_matrix.yml")})
	if code != ExitIdentIssue {
		t.Errorf("expected exit code %d for matrix unresolved expression, got %d", ExitIdentIssue, code)
	}
}

func TestCLI_Fixture19_UnrelatedActions(t *testing.T) {
	code := run([]string{"--workflow", testPath("testdata/workflows/19_unrelated_actions.yml")})
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for unrelated actions good flow, got %d", ExitSuccess, code)
	}
}

func TestCLI_Fixture20_DeterminismAcrossExecutions(t *testing.T) {
	// Capture output across 5 executions and ensure exact byte equality
	var previousJSON string
	for i := 0; i < 5; i++ {
		origStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		_ = run([]string{"--format", "json", testPath("testdata/repos/multiple_workflows")})

		w.Close()
		os.Stdout = origStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		currJSON := buf.String()

		if i > 0 && currJSON != previousJSON {
			t.Fatalf("run %d output differs from previous run:\nPrevious:\n%s\nCurrent:\n%s", i, previousJSON, currJSON)
		}
		previousJSON = currJSON
	}
}

func TestCLI_JSONOutputFormat(t *testing.T) {
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run([]string{"--format", "json", "--workflow", testPath("testdata/workflows/04_good_flow.yml")})

	w.Close()
	os.Stdout = origStdout

	if code != ExitSuccess {
		t.Fatalf("expected code 0, got %d", code)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, `"schemaVersion": "1"`) || !strings.Contains(out, `"toolVersion": "0.1.0"`) {
		t.Errorf("expected schemaVersion and toolVersion in JSON output, got:\n%s", out)
	}
	if !strings.Contains(out, `"artifactChains": 1`) {
		t.Errorf("expected artifactChains 1, got:\n%s", out)
	}
}

func TestCLI_Fixtures21to40(t *testing.T) {
	tests := []struct {
		name         string
		fixture      string
		expectedCode int
	}{
		{"21 Unnamed download single artifact", "testdata/workflows/21_unnamed_download_one_artifact.yml", ExitSuccess},
		{"22 Unnamed download multiple artifacts", "testdata/workflows/22_unnamed_download_multiple_artifacts.yml", ExitIdentIssue},
		{"23 Test selects, deploy downloads all", "testdata/workflows/23_test_selects_deploy_downloads_all.yml", ExitIdentIssue},
		{"24 Test downloads all, deploy selects", "testdata/workflows/24_test_downloads_all_deploy_selects.yml", ExitIdentIssue},
		{"25 Both download all, multiple artifacts", "testdata/workflows/25_both_download_all_multiple_artifacts.yml", ExitIdentIssue},
		{"26 Duplicate producers with same name", "testdata/workflows/26_duplicate_producers_same_name.yml", ExitIdentIssue},
		{"27 Incompatible conditions", "testdata/workflows/27_incompatible_conditions.yml", ExitIdentIssue},
		{"28 Continue on error validation", "testdata/workflows/28_continue_on_error_validation.yml", ExitIdentIssue},
		{"29 Matrix domain mismatch", "testdata/workflows/29_matrix_domain_mismatch.yml", ExitIdentIssue},
		{"30 Static artifact name in matrix", "testdata/workflows/30_static_name_in_matrix.yml", ExitIdentIssue},
		{"31 Reusable workflow boundary", "testdata/workflows/31_reusable_workflow_opaque.yml", ExitIdentIssue},
		{"32 YAML merge keys anchor resolution", "testdata/workflows/32_yaml_merge_keys.yml", ExitSuccess},
		{"33 Self dependency error", "testdata/workflows/33_self_dependency.yml", ExitInvocation},
		{"34 Action semver and SHA mixing", "testdata/workflows/34_action_semver_and_sha.yml", ExitSuccess},
		{"35 Unsupported download pattern", "testdata/workflows/35_download_pattern_unsupported.yml", ExitIdentIssue},
		{"36 Unsupported download merge-multiple", "testdata/workflows/36_download_merge_multiple.yml", ExitIdentIssue},
		{"37 Docker digest mismatch", "testdata/workflows/37_docker_digest_mismatch.yml", ExitIdentIssue},
		{"38 Auxiliary tool build in deploy", "testdata/workflows/38_auxiliary_tool_build_no_rebuild_fail.yml", ExitSuccess},
		{"39 Echo and comment commands in deploy", "testdata/workflows/39_echo_and_comment_commands.yml", ExitSuccess},
		{"40 Rebuild docs without false fail", "testdata/workflows/40_rebuild_docs_no_false_fail.yml", ExitSuccess},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			origStdout := os.Stdout
			origStderr := os.Stderr
			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()
			os.Stdout = wOut
			os.Stderr = wErr

			code := run([]string{"--workflow", testPath(tc.fixture)})

			wOut.Close()
			wErr.Close()
			os.Stdout = origStdout
			os.Stderr = origStderr

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(rOut)
			_, _ = buf.ReadFrom(rErr)

			if code != tc.expectedCode {
				t.Errorf("%s: expected exit code %d, got %d. Output:\n%s", tc.fixture, tc.expectedCode, code, buf.String())
			}
		})
	}
}

func TestCLI_GoldenJSONSchema(t *testing.T) {
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run([]string{"--format", "json", "--workflow", testPath("testdata/workflows/04_good_flow.yml")})

	w.Close()
	os.Stdout = origStdout

	if code != ExitSuccess {
		t.Fatalf("expected code 0, got %d", code)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var doc map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}

	// Verify top-level contract
	if doc["schemaVersion"] != "1" {
		t.Errorf("expected schemaVersion '1', got %v", doc["schemaVersion"])
	}
	if doc["toolVersion"] != "0.1.0" {
		t.Errorf("expected toolVersion '0.1.0', got %v", doc["toolVersion"])
	}

	summary, ok := doc["summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing or invalid summary: %v", doc["summary"])
	}
	if summary["workflowsAnalyzed"] != float64(1) {
		t.Errorf("expected workflowsAnalyzed 1, got %v", summary["workflowsAnalyzed"])
	}
	if summary["artifactChains"] != float64(1) {
		t.Errorf("expected artifactChains 1, got %v", summary["artifactChains"])
	}
	if summary["failures"] != float64(0) {
		t.Errorf("expected failures 0, got %v", summary["failures"])
	}

	workflows, ok := doc["workflows"].([]interface{})
	if !ok || len(workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %v", doc["workflows"])
	}
	wf0 := workflows[0].(map[string]interface{})
	if wf0["status"] != "PASS" {
		t.Errorf("expected workflow status PASS, got %v", wf0["status"])
	}

	flows, ok := doc["artifactFlows"].([]interface{})
	if !ok || len(flows) != 1 {
		t.Fatalf("expected 1 artifactFlow, got %v", doc["artifactFlows"])
	}
	f0 := flows[0].(map[string]interface{})
	if f0["artifactName"] != "web-dist" || f0["status"] != "ESTABLISHED" {
		t.Errorf("unexpected flow: %v", f0)
	}
}

func TestCLI_WorkflowDiscoveryDeterminism(t *testing.T) {
	// Execute 10 runs against multiple_workflows repository and verify bit-for-bit identical output
	var baseOutput string
	for i := 0; i < 10; i++ {
		origStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		code := run([]string{"--format", "json", testPath("testdata/repos/multiple_workflows")})

		w.Close()
		os.Stdout = origStdout

		if code != ExitSuccess {
			t.Fatalf("run %d failed with code %d", i, code)
		}

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		out := buf.String()

		if i == 0 {
			baseOutput = out
		} else if out != baseOutput {
			t.Fatalf("determinism failure on iteration %d:\nBase: %s\nCurrent: %s", i, baseOutput, out)
		}
	}
}

func TestCLI_SymlinkEscapingRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "artifact-id-symlink-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	wfDir := filepath.Join(repoDir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}

	outsideFile := filepath.Join(tmpDir, "outside.yml")
	if err := os.WriteFile(outsideFile, []byte("name: Outside\njobs: {}\n"), 0644); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}

	symlinkPath := filepath.Join(wfDir, "symlink.yml")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := run([]string{repoDir})

	w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if code != ExitInvocation {
		t.Errorf("expected ExitInvocation (2) for symlink escaping repo, got %d. Stderr:\n%s", code, buf.String())
	}
	if !strings.Contains(buf.String(), "links outside repository") {
		t.Errorf("expected error message about linking outside repository, got: %s", buf.String())
	}
}

func TestCLI_SyntheticTopologies(t *testing.T) {
	// Representative synthetic GitHub Actions workflow topologies
	tests := []struct {
		name         string
		fixture      string
		expectedCode int
	}{
		{"01 Go CLI Release", "testdata/topologies/01_go_cli.yml", ExitSuccess},
		{"02 Rust Service CI/CD", "testdata/topologies/02_rust_service.yml", ExitSuccess},
		{"03 Next.js Cloudflare Deploy", "testdata/topologies/03_node_nextjs.yml", ExitSuccess},
		{"04 Python FastAPI S3 Deploy", "testdata/topologies/04_python_fastapi.yml", ExitSuccess},
		{"05 Docker Kubernetes Deploy", "testdata/topologies/05_docker_k8s.yml", ExitSuccess},
		{"06 GitHub Pages Deploy", "testdata/topologies/06_github_pages.yml", ExitSuccess},
		{"07 Matrix Cross Compile (Unverified)", "testdata/topologies/07_matrix_cross_compile.yml", ExitIdentIssue},
		{"08 Monorepo Multi Service", "testdata/topologies/08_monorepo_services.yml", ExitSuccess},
		{"09 Java Spring Boot", "testdata/topologies/09_java_spring.yml", ExitSuccess},
		{"10 Rebuild Anti-Pattern (Fail AI001)", "testdata/topologies/10_rebuild_anti_pattern.yml", ExitIdentIssue},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			origStdout := os.Stdout
			origStderr := os.Stderr
			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()
			os.Stdout = wOut
			os.Stderr = wErr

			code := run([]string{"--workflow", testPath(tc.fixture)})

			wOut.Close()
			wErr.Close()
			os.Stdout = origStdout
			os.Stderr = origStderr

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(rOut)
			_, _ = buf.ReadFrom(rErr)

			if code != tc.expectedCode {
				t.Errorf("%s: expected exit code %d, got %d. Output:\n%s", tc.fixture, tc.expectedCode, code, buf.String())
			}
		})
	}
}
