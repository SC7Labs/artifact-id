package workflow

import (
	"reflect"
	"testing"
)

func TestBuildDAG_DeterministicOrder(t *testing.T) {
	// Jobs with identical in-degree should be sorted alphabetically:
	// alpha, beta, gamma all in-degree 0 -> order alpha, beta, gamma
	jobs := map[string]*Job{
		"gamma": {ID: "gamma"},
		"alpha": {ID: "alpha"},
		"beta":  {ID: "beta"},
	}

	dag, err := BuildDAG(jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"alpha", "beta", "gamma"}
	if !reflect.DeepEqual(dag.TopologicalOrder, expected) {
		t.Errorf("expected deterministic order %v, got %v", expected, dag.TopologicalOrder)
	}
}

func TestBuildDAG_LinearAndDiamond(t *testing.T) {
	// Diamond DAG:
	//       build
	//      /     \
	//   test-a  test-b
	//      \     /
	//       deploy
	jobs := map[string]*Job{
		"build":  {ID: "build"},
		"test-a": {ID: "test-a", Needs: []string{"build"}},
		"test-b": {ID: "test-b", Needs: []string{"build"}},
		"deploy": {ID: "deploy", Needs: []string{"test-a", "test-b"}},
	}

	dag, err := BuildDAG(jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"build", "test-a", "test-b", "deploy"}
	if !reflect.DeepEqual(dag.TopologicalOrder, expected) {
		t.Errorf("expected order %v, got %v", expected, dag.TopologicalOrder)
	}

	if !dag.IsDownstream("build", "deploy") {
		t.Error("expected deploy to be downstream of build")
	}
	if !dag.IsDownstream("test-a", "deploy") {
		t.Error("expected deploy to be downstream of test-a")
	}
	if dag.IsDownstream("deploy", "build") {
		t.Error("did not expect build to be downstream of deploy")
	}

	upstream := dag.GetTransitiveUpstream("deploy")
	if !upstream["build"] || !upstream["test-a"] || !upstream["test-b"] {
		t.Errorf("expected all 3 jobs upstream of deploy, got %v", upstream)
	}
}

func TestBuildDAG_CycleDetection(t *testing.T) {
	jobs := map[string]*Job{
		"a": {ID: "a", Needs: []string{"b"}},
		"b": {ID: "b", Needs: []string{"a"}},
	}

	_, err := BuildDAG(jobs)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestBuildDAG_MissingDependency(t *testing.T) {
	jobs := map[string]*Job{
		"a": {ID: "a", Needs: []string{"nonexistent"}},
	}
	_, err := BuildDAG(jobs)
	if err == nil {
		t.Fatal("expected missing dependency error, got nil")
	}
}

func TestDAG_GetTransitiveDownstream(t *testing.T) {
	jobs := map[string]*Job{
		"build":  {ID: "build"},
		"test":   {ID: "test", Needs: []string{"build"}},
		"deploy": {ID: "deploy", Needs: []string{"test"}},
	}
	dag, err := BuildDAG(jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	down := dag.GetTransitiveDownstream("build")
	if !down["test"] || !down["deploy"] {
		t.Errorf("expected test and deploy downstream of build, got %v", down)
	}
}
