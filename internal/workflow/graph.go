package workflow

import (
	"fmt"
	"sort"
)

// DAG represents a directed acyclic graph of jobs within a workflow.
type DAG struct {
	Jobs             map[string]*Job
	AdjacencyList    map[string][]string // from dependency -> list of dependent jobs
	ReverseAdjList   map[string][]string // from dependent job -> list of dependencies (needs)
	TopologicalOrder []string
}

// BuildDAG constructs and validates a DAG from the jobs map.
func BuildDAG(jobs map[string]*Job) (*DAG, error) {
	dag := &DAG{
		Jobs:           jobs,
		AdjacencyList:  make(map[string][]string),
		ReverseAdjList: make(map[string][]string),
	}

	for id := range jobs {
		dag.AdjacencyList[id] = make([]string, 0)
		dag.ReverseAdjList[id] = make([]string, 0)
	}

	inDegree := make(map[string]int)
	for id := range jobs {
		inDegree[id] = 0
	}

	for id, job := range jobs {
		seenNeeds := make(map[string]bool)
		for _, dep := range job.Needs {
			if dep == id {
				return nil, fmt.Errorf("job %q has an invalid self-dependency on itself", id)
			}
			if _, exists := jobs[dep]; !exists {
				return nil, fmt.Errorf("job %q references non-existent job dependency %q", id, dep)
			}
			if seenNeeds[dep] {
				continue // deduplicate
			}
			seenNeeds[dep] = true

			dag.AdjacencyList[dep] = append(dag.AdjacencyList[dep], id)
			dag.ReverseAdjList[id] = append(dag.ReverseAdjList[id], dep)
			inDegree[id]++
		}
	}

	// Sort adjacency lists for determinism
	for id := range dag.AdjacencyList {
		sort.Strings(dag.AdjacencyList[id])
	}
	for id := range dag.ReverseAdjList {
		sort.Strings(dag.ReverseAdjList[id])
	}

	// Deterministic Kahn's algorithm: priority queue sorted alphabetically
	var ready []string
	for id, deg := range inDegree {
		if deg == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)

	var topo []string
	for len(ready) > 0 {
		curr := ready[0]
		ready = ready[1:]
		topo = append(topo, curr)

		for _, next := range dag.AdjacencyList[curr] {
			inDegree[next]--
			if inDegree[next] == 0 {
				ready = append(ready, next)
				sort.Strings(ready)
			}
		}
	}

	if len(topo) != len(jobs) {
		return nil, fmt.Errorf("cycle detected in job dependencies")
	}

	dag.TopologicalOrder = topo
	return dag, nil
}

// IsDownstream returns true if targetJobID transitively depends on sourceJobID.
func (d *DAG) IsDownstream(sourceJobID, targetJobID string) bool {
	if sourceJobID == targetJobID {
		return false
	}
	visited := make(map[string]bool)
	queue := []string{sourceJobID}
	visited[sourceJobID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, next := range d.AdjacencyList[curr] {
			if next == targetJobID {
				return true
			}
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

// GetTransitiveUpstream returns all jobs that the given job depends on (directly or transitively).
func (d *DAG) GetTransitiveUpstream(jobID string) map[string]bool {
	upstream := make(map[string]bool)
	queue := []string{jobID}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, dep := range d.ReverseAdjList[curr] {
			if !upstream[dep] {
				upstream[dep] = true
				queue = append(queue, dep)
			}
		}
	}
	return upstream
}

// GetTransitiveDownstream returns all jobs that depend on the given job (directly or transitively).
func (d *DAG) GetTransitiveDownstream(jobID string) map[string]bool {
	downstream := make(map[string]bool)
	queue := []string{jobID}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, next := range d.AdjacencyList[curr] {
			if !downstream[next] {
				downstream[next] = true
				queue = append(queue, next)
			}
		}
	}
	return downstream
}
