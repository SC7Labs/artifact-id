package analysis

import (
	"testing"

	"ci-artifact-identity/internal/workflow"
)

func TestAnalyzeWorkflow_AI004_GoodFlow(t *testing.T) {
	yamlContent := `
name: CI Good
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: npm run build
      - uses: actions/upload-artifact@v4
        with:
          name: web-dist
          path: dist/
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: web-dist
      - run: npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: web-dist
      - run: npm run deploy
`
	wf, err := workflow.ParseWorkflowContent("good.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Status != "PASS" {
		t.Errorf("expected PASS status, got %s", report.Status)
	}

	if len(report.ArtifactFlows) != 1 {
		t.Fatalf("expected 1 artifact flow, got %d", len(report.ArtifactFlows))
	}

	flow := report.ArtifactFlows[0]
	if flow.Status != FlowEstablished || flow.ArtifactName != "web-dist" {
		t.Errorf("expected established flow for web-dist, got %+v", flow)
	}
}

func TestAnalyzeWorkflow_AI001_RebuildAfterValidation(t *testing.T) {
	yamlContent := `
name: CI Bad Rebuild
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: npm run build
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - run: npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - run: npm run build
      - run: npm run deploy
`
	wf, err := workflow.ParseWorkflowContent("bad_rebuild.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Status != "FAIL" {
		t.Errorf("expected FAIL status, got %s", report.Status)
	}

	foundAI001 := false
	for _, f := range report.Findings {
		if f.RuleID == RuleRebuildAfterValidation && f.Status == StatusFail {
			foundAI001 = true
			break
		}
	}
	if !foundAI001 {
		t.Errorf("expected AI001 FAIL finding, got %v", report.Findings)
	}
}

func TestAnalyzeWorkflow_AI002_ArtifactMismatch(t *testing.T) {
	yamlContent := `
name: CI Mismatch
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: npm run build
      - uses: actions/upload-artifact@v4
        with:
          name: frontend-dist
          path: dist/
      - uses: actions/upload-artifact@v4
        with:
          name: production-dist
          path: prod/
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: frontend-dist
      - run: npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: production-dist
      - run: npm run deploy
`
	wf, err := workflow.ParseWorkflowContent("mismatch.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Status != "FAIL" {
		t.Errorf("expected FAIL status, got %s", report.Status)
	}

	foundAI002 := false
	for _, f := range report.Findings {
		if f.RuleID == RuleTestedAndDeployedArtifactMismatch && f.Status == StatusFail {
			foundAI002 = true
			break
		}
	}
	if !foundAI002 {
		t.Errorf("expected AI002 FAIL finding, got %v", report.Findings)
	}
}

func TestAnalyzeWorkflow_AI003_UnverifiedExpressions(t *testing.T) {
	yamlContent := `
name: CI Unverified
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: npm run build
      - uses: actions/upload-artifact@v4
        with:
          name: ${{ github.ref }}-dist
          path: dist/
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: ${{ needs.build.outputs.artifact_name }}
      - run: npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: ${{ inputs.release_artifact }}
      - run: npm run deploy
`
	wf, err := workflow.ParseWorkflowContent("unverified.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Status != "UNVERIFIED" {
		t.Errorf("expected UNVERIFIED status, got %s", report.Status)
	}

	foundAI003 := false
	for _, f := range report.Findings {
		if f.RuleID == RuleArtifactIdentityUnverified && f.Status == StatusUnverified {
			foundAI003 = true
			break
		}
	}
	if !foundAI003 {
		t.Errorf("expected AI003 UNVERIFIED finding, got %v", report.Findings)
	}
}

func TestAnalyzeWorkflow_AI005_DockerMutableTag(t *testing.T) {
	yamlContent := `
name: Docker Flow
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - id: docker_build
        uses: docker/build-push-action@v5
        with:
          push: true
          tags: myregistry/app:latest
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - run: docker run myregistry/app@${{ steps.docker_build.outputs.digest }} npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - run: gcloud run deploy my-app --image myregistry/app:latest
`
	wf, err := workflow.ParseWorkflowContent("docker.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundAI005 := false
	for _, f := range report.Findings {
		if f.RuleID == RuleMutableImageReference && f.Status == StatusWarning {
			foundAI005 = true
			break
		}
	}
	if !foundAI005 {
		t.Errorf("expected AI005 WARNING finding, got %v", report.Findings)
	}
}

func TestAnalyzeWorkflow_NotApplicable(t *testing.T) {
	yamlContent := `
name: CI Only
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: npm run build
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - run: npm test
`
	wf, err := workflow.ParseWorkflowContent("not_applicable.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Status != "NOT_APPLICABLE" {
		t.Errorf("expected NOT_APPLICABLE status, got %s", report.Status)
	}
}

func TestAnalyzeWorkflow_UnnamedDownload_MultipleArtifacts(t *testing.T) {
	yamlContent := `
name: Ambiguous Download
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/upload-artifact@v4
        with: { name: client, path: a/ }
      - uses: actions/upload-artifact@v4
        with: { name: server, path: b/ }
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with: { name: client }
      - run: npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
      - run: npm run deploy
`
	wf, err := workflow.ParseWorkflowContent("ambig.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if report.Status != "UNVERIFIED" {
		t.Errorf("expected UNVERIFIED for unnamed download with multiple artifacts, got %s", report.Status)
	}
}

func TestAnalyzeWorkflow_DuplicateProducers(t *testing.T) {
	yamlContent := `
name: Duplicate Producers
jobs:
  build-a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/upload-artifact@v4
        with: { name: dist, path: a/ }
  build-b:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/upload-artifact@v4
        with: { name: dist, path: b/ }
  test:
    needs: [build-a, build-b]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with: { name: dist }
      - run: npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with: { name: dist }
      - run: npm run deploy
`
	wf, err := workflow.ParseWorkflowContent("dup.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if report.Status != "UNVERIFIED" {
		t.Errorf("expected UNVERIFIED for duplicate producers with same name, got %s", report.Status)
	}
}

func TestAnalyzeWorkflow_AuxiliaryBuildNoFail(t *testing.T) {
	yamlContent := `
name: Auxiliary Tool
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: npm run build
      - uses: actions/upload-artifact@v4
        with: { name: web-dist, path: dist/ }
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with: { name: web-dist }
      - run: npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with: { name: web-dist }
      - name: Build custom uploader
        run: go build -o uploader ./tools/uploader
      - run: npm run deploy
`
	wf, err := workflow.ParseWorkflowContent("aux.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if report.Status != "PASS" {
		t.Errorf("expected PASS when deploy only builds auxiliary tool, got %s", report.Status)
	}
}

func TestAnalyzeWorkflow_OpaqueAction_ValidationPath(t *testing.T) {
	yamlContent := `
name: Opaque Validation Action
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: npm run build
      - uses: actions/upload-artifact@v4
        with: { name: app-dist, path: dist/ }
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with: { name: app-dist }
      - uses: ./actions/run-custom-tests
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with: { name: app-dist }
      - run: npm run deploy
`
	wf, err := workflow.ParseWorkflowContent("opaque_val.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if report.Status != "UNVERIFIED" {
		t.Errorf("expected UNVERIFIED status, got %s", report.Status)
	}

	foundOpaque := false
	for _, f := range report.Findings {
		if f.RuleID == RuleArtifactIdentityUnverified && f.Reason == ReasonOpaqueAction {
			foundOpaque = true
			break
		}
	}
	if !foundOpaque {
		t.Errorf("expected AI003 with reason opaque-action, got findings: %+v", report.Findings)
	}
}

func TestAnalyzeWorkflow_OpaqueAction_DeployPath(t *testing.T) {
	yamlContent := `
name: Opaque Deploy Action
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: npm run build
      - uses: actions/upload-artifact@v4
        with: { name: app-dist, path: dist/ }
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with: { name: app-dist }
      - run: npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with: { name: app-dist }
      - uses: custom-vendor/deploy-action@v2
`
	wf, err := workflow.ParseWorkflowContent("opaque_dep.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if report.Status != "UNVERIFIED" {
		t.Errorf("expected UNVERIFIED status, got %s", report.Status)
	}

	foundOpaque := false
	for _, f := range report.Findings {
		if f.RuleID == RuleArtifactIdentityUnverified && f.Reason == ReasonOpaqueAction {
			foundOpaque = true
			break
		}
	}
	if !foundOpaque {
		t.Errorf("expected AI003 with reason opaque-action in deploy job, got findings: %+v", report.Findings)
	}
}

func TestAnalyzeWorkflow_DockerTags_MutableWarning(t *testing.T) {
	mutableTags := []string{
		"latest",
		"main",
		"dev",
		"staging",
		"prod",
		"production",
	}

	for _, tag := range mutableTags {
		t.Run("tag_"+tag, func(t *testing.T) {
			yamlContent := `
name: Docker Flow
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - id: docker_build
        uses: docker/build-push-action@v5
        with:
          push: true
          tags: myregistry/app:` + tag + `
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - run: docker run myregistry/app@${{ steps.docker_build.outputs.digest }} npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - run: gcloud run deploy my-app --image myregistry/app:` + tag + `
`
			wf, err := workflow.ParseWorkflowContent("docker_tag.yml", []byte(yamlContent))
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			report, err := AnalyzeWorkflow(wf)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			foundAI005 := false
			for _, f := range report.Findings {
				if f.RuleID == RuleMutableImageReference && f.Status == StatusWarning {
					foundAI005 = true
					break
				}
			}
			if !foundAI005 {
				t.Errorf("tag %q: expected AI005 WARNING finding, got %v", tag, report.Findings)
			}
		})
	}
}

func TestAnalyzeWorkflow_DockerDynamicDigest(t *testing.T) {
	yamlContent := `
name: Docker Dynamic Digest
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - id: docker_build
        uses: docker/build-push-action@v5
        with:
          push: true
          tags: myregistry/app:latest
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - run: docker run myregistry/app@${{ steps.docker_build.outputs.digest }} npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - run: gcloud run deploy my-app --image myregistry/app@${{ env.DYNAMIC_DIGEST }}
`
	wf, err := workflow.ParseWorkflowContent("docker_dynamic.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundDynamic := false
	for _, f := range report.Findings {
		if f.RuleID == RuleArtifactIdentityUnverified && f.Reason == ReasonDynamicDigest {
			foundDynamic = true
			break
		}
	}
	if !foundDynamic {
		t.Errorf("expected AI003 UNVERIFIED with reason dynamic-digest, got: %+v", report.Findings)
	}
}

func TestAnalyzeWorkflow_JSONEmptyArraysNotNull(t *testing.T) {
	yamlContent := `
name: Simple No Findings Deploy
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: npm run deploy
`
	wf, err := workflow.ParseWorkflowContent("no_findings.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if report.Findings == nil {
		t.Errorf("expected non-nil report.Findings")
	}
	if report.ArtifactFlows == nil {
		t.Errorf("expected non-nil report.ArtifactFlows")
	}
}

func TestAnalyzeWorkflow_UnvalidatedArtifactInDAGWithUnrelatedValidation(t *testing.T) {
	// Job 'test_other' runs tests on an unrelated component.
	// Job 'build_pkg' builds an artifact 'pkg-dist' without testing it.
	// Job 'deploy' needs both and deploys 'pkg-dist'.
	// The deployed artifact was NEVER tested, so it must NOT pass.
	yamlContent := `
name: Monorepo Deploy
jobs:
  test_other:
    runs-on: ubuntu-latest
    steps:
      - run: pytest tests/other/
  build_pkg:
    runs-on: ubuntu-latest
    steps:
      - run: python -m build
      - uses: actions/upload-artifact@v4
        with:
          name: pkg-dist
          path: dist/
  deploy:
    needs: [test_other, build_pkg]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: pkg-dist
      - uses: pypa/gh-action-pypi-publish@release/v1
`
	wf, err := workflow.ParseWorkflowContent("unvalidated_monorepo.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	report, err := AnalyzeWorkflow(wf)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if report.Status != "UNVERIFIED" {
		t.Errorf("expected UNVERIFIED status for unvalidated artifact in multi-job DAG, got: %s", report.Status)
	}

	foundUnverified := false
	for _, f := range report.Findings {
		if f.RuleID == RuleArtifactIdentityUnverified && f.Reason == ReasonUnsupportedArtifactSemantics {
			foundUnverified = true
			break
		}
	}
	if !foundUnverified {
		t.Errorf("expected AI003 finding with reason unsupported-artifact-semantics, got: %+v", report.Findings)
	}
}
