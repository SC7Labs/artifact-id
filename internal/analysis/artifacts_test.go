package analysis

import (
	"testing"

	"ci-artifact-identity/internal/workflow"
)

func TestNormalizeExpression(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"dist", "dist"},
		{"${{ github.sha }}", "${{ github.sha }}"},
		{"${{    github.sha   }}", "${{ github.sha }}"},
		{"app-${{   github.sha   }}", "app-${{ github.sha }}"},
		{"${{  inputs.name  }}-dist", "${{ inputs.name }}-dist"},
	}

	for _, c := range cases {
		got := NormalizeExpression(c.input)
		if got != c.expected {
			t.Errorf("NormalizeExpression(%q) = %q, expected %q", c.input, got, c.expected)
		}
	}
}

func TestCompareArtifactNames(t *testing.T) {
	tests := []struct {
		producer  string
		consumer  string
		matched   bool
		ambiguous bool
	}{
		// 1. Exact static strings
		{"web-dist", "web-dist", true, false},
		// 2. Different static strings
		{"frontend-dist", "production-dist", false, false},
		// 3. Wildcard download
		{"web-dist", "*", true, false},
		{"web-dist", "", true, false},
		// 4. Identical symbolic expressions
		{"app-${{ github.sha }}", "app-${{ github.sha }}", true, false},
		{"app-${{    github.sha    }}", "app-${{ github.sha }}", true, false},
		// 5. Unresolved different expressions
		{"${{ needs.build.outputs.artifact }}", "${{ inputs.release_artifact }}", false, true},
		{"web-dist", "${{ inputs.artifact_name }}", false, true},
	}

	for _, tc := range tests {
		m, a := CompareArtifactNames(tc.producer, tc.consumer, nil)
		if m != tc.matched || a != tc.ambiguous {
			t.Errorf("CompareArtifactNames(%q, %q) = (%v, %v); expected (%v, %v)",
				tc.producer, tc.consumer, m, a, tc.matched, tc.ambiguous)
		}
	}
}

func TestExtractArtifactProducersAndConsumers(t *testing.T) {
	job := &workflow.Job{
		ID: "test-job",
		Steps: []*workflow.Step{
			{
				Index: 0,
				Uses:  "actions/upload-artifact@v4",
				With: map[string]interface{}{
					"name": "my-artifact",
					"path": "dist/",
				},
			},
			{
				Index: 1,
				Uses:  "actions/download-artifact@v4",
				With: map[string]interface{}{
					"name": "my-artifact",
				},
			},
			{
				Index: 2,
				Uses:  "actions/upload-artifact@v4",
				// No name specified: defaults to "artifact"
				With: map[string]interface{}{
					"path": "out/",
				},
			},
			{
				Index: 3,
				Uses:  "actions/download-artifact@v4",
				// No name specified: downloads all
			},
		},
	}

	producers := ExtractArtifactProducers("wf.yml", job)
	if len(producers) != 2 {
		t.Fatalf("expected 2 producers, got %d", len(producers))
	}
	if producers[0].ArtifactName != "my-artifact" {
		t.Errorf("expected 'my-artifact', got %q", producers[0].ArtifactName)
	}
	if producers[1].ArtifactName != "artifact" {
		t.Errorf("expected default 'artifact', got %q", producers[1].ArtifactName)
	}

	consumers := ExtractArtifactConsumers("wf.yml", job)
	if len(consumers) != 2 {
		t.Fatalf("expected 2 consumers, got %d", len(consumers))
	}
	if consumers[0].ArtifactName != "my-artifact" {
		t.Errorf("expected 'my-artifact', got %q", consumers[0].ArtifactName)
	}
	if consumers[1].ArtifactName != "*" || !consumers[1].DownloadsAll {
		t.Errorf("expected wildcard consumer, got %+v", consumers[1])
	}
}

func TestCompareMatrixContexts(t *testing.T) {
	prodMatrix := map[string][]string{"os": {"ubuntu-latest", "macos-latest"}}
	consMatrixIdentical := map[string][]string{"os": {"macos-latest", "ubuntu-latest"}}
	consMatrixDifferent := map[string][]string{"os": {"ubuntu-latest", "windows-latest"}}
	consMatrixMissing := map[string][]string{"arch": {"amd64"}}

	// Matching matrix references
	comp, amb := CompareMatrixContexts(prodMatrix, consMatrixIdentical, "cli-${{ matrix.os }}", "cli-${{ matrix.os }}")
	if !comp || amb {
		t.Errorf("expected compatible without ambiguity, got comp=%v amb=%v", comp, amb)
	}

	// Different domain
	comp, amb = CompareMatrixContexts(prodMatrix, consMatrixDifferent, "cli-${{ matrix.os }}", "cli-${{ matrix.os }}")
	if comp || !amb {
		t.Errorf("expected incompatible and ambiguous for different domains, got comp=%v amb=%v", comp, amb)
	}

	// Missing parameter
	comp, amb = CompareMatrixContexts(prodMatrix, consMatrixMissing, "cli-${{ matrix.os }}", "cli-${{ matrix.os }}")
	if comp || !amb {
		t.Errorf("expected incompatible and ambiguous for missing param, got comp=%v amb=%v", comp, amb)
	}

	// No matrix expressions
	comp, amb = CompareMatrixContexts(prodMatrix, consMatrixIdentical, "static-artifact", "static-artifact")
	if !comp || amb {
		t.Errorf("expected compatible for non-matrix artifacts, got comp=%v amb=%v", comp, amb)
	}
}

func TestCompareConditions(t *testing.T) {
	// Empty conditions
	comp, amb := CompareConditions("", "")
	if !comp || amb {
		t.Errorf("expected compatible for empty conditions, got (%v, %v)", comp, amb)
	}

	// Identical conditions
	comp, amb = CompareConditions("github.ref == 'refs/heads/main'", "github.ref == 'refs/heads/main'")
	if !comp || amb {
		t.Errorf("expected compatible for identical conditions, got (%v, %v)", comp, amb)
	}

	// Incompatible conditions
	comp, amb = CompareConditions("github.ref == 'refs/heads/main'", "github.ref == 'refs/heads/release'")
	if comp || !amb {
		t.Errorf("expected ambiguous for different conditions, got (%v, %v)", comp, amb)
	}
}

func FuzzNormalizeExpression(f *testing.F) {
	f.Add("dist")
	f.Add("${{ github.sha }}")
	f.Add("${{    github.sha   }}")
	f.Add("app-${{ github.ref }}-v1")
	f.Add("${{ inputs.name }}")
	f.Add("${{")
	f.Add("}}")
	f.Add("${{ }}")
	f.Add("")
	f.Add("${{   matrix.os   }}-${{  matrix.node  }}")

	f.Fuzz(func(t *testing.T, input string) {
		res := NormalizeExpression(input)
		res2 := NormalizeExpression(res)
		if res != res2 {
			t.Errorf("NormalizeExpression is not idempotent: %q != %q", res, res2)
		}
	})
}
