package analysis

import (
	"testing"

	"ci-artifact-identity/internal/workflow"
)

func TestClassifyBuildStep(t *testing.T) {
	tests := []struct {
		run      string
		expected string // build family
	}{
		{"npm run build", "npm"},
		{"pnpm build", "pnpm"},
		{"yarn run build", "yarn"},
		{"bun run build", "bun"},
		{"go build ./...", "go"},
		{"cargo build --release", "cargo"},
		{"dotnet build -c Release", "dotnet"},
		{"mvn clean package", "maven"},
		{"./gradlew build -x test", "gradle"},
		{"python3 -m build", "python"},
		{"docker build -t app:latest .", "docker"},
	}

	for _, tc := range tests {
		step := &workflow.Step{Index: 0, Run: tc.run}
		events := ClassifyBuildStep("test-job", step)
		if len(events) == 0 {
			t.Errorf("expected build event for command %q, got none", tc.run)
			continue
		}
		if events[0].BuildFamily != tc.expected {
			t.Errorf("expected family %q for %q, got %q", tc.expected, tc.run, events[0].BuildFamily)
		}
	}

	// Negative test: arbitrary commands should NOT be flagged as build
	negStep := &workflow.Step{Index: 0, Run: "echo 'hello world'\ncat file.txt\nls -la"}
	negEvents := ClassifyBuildStep("job", negStep)
	if len(negEvents) > 0 {
		t.Errorf("expected 0 build events for arbitrary shell commands, got %d", len(negEvents))
	}
}

func TestClassifyValidationStep(t *testing.T) {
	tests := []struct {
		run      string
		expected string
		isLint   bool
	}{
		{"npm test", "npm", false},
		{"pnpm run test", "pnpm", false},
		{"pytest tests/", "pytest", false},
		{"python -m pytest", "pytest", false},
		{"go test -v ./...", "go", false},
		{"cargo test", "cargo", false},
		{"dotnet test", "dotnet", false},
		{"mvn test", "maven", false},
		{"./gradlew test", "gradle", false},
		{"golangci-lint run", "lint", true},
		{"npm run lint", "lint", true},
	}

	for _, tc := range tests {
		step := &workflow.Step{Index: 0, Run: tc.run}
		events := ClassifyValidationStep("test-job", step)
		if len(events) == 0 {
			t.Errorf("expected validation event for %q, got none", tc.run)
			continue
		}
		if events[0].ValidationFamily != tc.expected {
			t.Errorf("expected family %q, got %q", tc.expected, events[0].ValidationFamily)
		}
		if events[0].IsSecondaryLint != tc.isLint {
			t.Errorf("expected IsSecondaryLint=%v for %q, got %v", tc.isLint, tc.run, events[0].IsSecondaryLint)
		}
	}
}

func TestClassifyDeploymentStep(t *testing.T) {
	tests := []struct {
		run      string
		uses     string
		expected string
	}{
		{"npm run deploy", "", "npm"},
		{"firebase deploy --only hosting", "", "firebase"},
		{"vercel --prod", "", "vercel"},
		{"kubectl apply -f k8s/", "", "kubectl"},
		{"helm upgrade --install myapp ./chart", "", "helm"},
		{"aws s3 sync dist/ s3://mybucket", "", "aws"},
		{"docker push myregistry/app:v1", "", "docker"},
		{"", "actions/deploy-pages@v4", "github-pages"},
		{"", "cloudflare/wrangler-action@v3", "cloudflare"},
		{"", "google-github-actions/deploy-cloudrun@v2", "gcloud"},
		{"", "softprops/action-gh-release@v1", "gh-release"},
		{"gh release create v1.0.0 --title v1.0.0", "", "gh-release"},
		{"gh release upload v1.0.0 dist/*", "", "gh-release"},
		{"cat assets.txt | xargs gh release upload v1.0.0", "", "gh-release"},
	}

	for _, tc := range tests {
		step := &workflow.Step{Index: 0, Run: tc.run, Uses: tc.uses}
		events := ClassifyDeploymentStep("deploy-job", step)
		if len(events) == 0 {
			t.Errorf("expected deployment event for command %q uses %q, got none", tc.run, tc.uses)
			continue
		}
		if events[0].DeploymentFamily != tc.expected {
			t.Errorf("expected family %q, got %q", tc.expected, events[0].DeploymentFamily)
		}
	}
}

func TestExtractCommands_PipeSplitting(t *testing.T) {
	script := "echo 'start' | cat\nnpm test | tee test.log\nscript/label-assets dist/* | xargs gh release create v1.0.0"
	cmds := ExtractCommands(script)
	// echo should be ignored, cat, npm test, tee test.log, script/label-assets dist/*, xargs gh release create v1.0.0
	foundNpmTest := false
	foundGhRelease := false
	for _, c := range cmds {
		if c == "npm test" {
			foundNpmTest = true
		}
		if c == "xargs gh release create v1.0.0" {
			foundGhRelease = true
		}
	}
	if !foundNpmTest {
		t.Errorf("expected 'npm test' to be extracted through pipe, got: %v", cmds)
	}
	if !foundGhRelease {
		t.Errorf("expected 'xargs gh release create v1.0.0' to be extracted through pipe, got: %v", cmds)
	}
}

func FuzzCommandClassifier(f *testing.F) {
	f.Add("npm run build")
	f.Add("go test -v ./...")
	f.Add("gcloud run deploy --image myapp")
	f.Add("FOO=bar ; (cmd1 && cmd2 || cmd3)")
	f.Add("# comment\necho 'hello'")
	f.Add("docker build -t app:latest .")
	f.Add("")

	f.Fuzz(func(t *testing.T, cmd string) {
		step := &workflow.Step{Index: 0, Run: cmd}
		_ = ClassifyBuildStep("job", step)
		_ = ClassifyValidationStep("job", step)
		_ = ClassifyDeploymentStep("job", step)
		_ = ExtractCommands(cmd)
	})
}
