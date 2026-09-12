package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseWorkflowFile reads and parses a GitHub Actions workflow from a YAML file.
func ParseWorkflowFile(filePath string) (*Workflow, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat workflow file %q: %w", filePath, err)
	}
	if info.Size() > MaxWorkflowFileSize {
		return nil, fmt.Errorf("workflow file %q exceeds maximum allowed size of %d bytes", filePath, MaxWorkflowFileSize)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file %q: %w", filePath, err)
	}

	return ParseWorkflowContent(filePath, data)
}

// ParseWorkflowContent parses GitHub Actions workflow YAML content.
func ParseWorkflowContent(filePath string, data []byte) (*Workflow, error) {
	if len(data) > MaxWorkflowFileSize {
		return nil, fmt.Errorf("workflow content in %q exceeds maximum allowed size of %d bytes", filePath, MaxWorkflowFileSize)
	}

	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return nil, fmt.Errorf("invalid YAML in %q: %w", filePath, err)
	}

	if len(rootNode.Content) == 0 {
		return nil, fmt.Errorf("empty YAML document in %q", filePath)
	}

	docNode := derefNode(rootNode.Content[0])
	if docNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("workflow root must be a YAML mapping in %q", filePath)
	}

	wf := &Workflow{
		Path: filePath,
		Jobs: make(map[string]*Job),
	}

	entries := flattenMapping(docNode)
	for key, valNode := range entries {
		switch key {
		case "name":
			wf.Name = strings.TrimSpace(valNode.Value)
		case "jobs":
			valNode = derefNode(valNode)
			if valNode.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("'jobs' must be a mapping in %q", filePath)
			}
			jobs, err := parseJobs(valNode)
			if err != nil {
				return nil, fmt.Errorf("failed to parse jobs in %q: %w", filePath, err)
			}
			if len(jobs) > MaxJobsPerWorkflow {
				return nil, fmt.Errorf("workflow %q exceeds maximum job limit of %d", filePath, MaxJobsPerWorkflow)
			}
			wf.Jobs = jobs
		}
	}

	if wf.Name == "" {
		wf.Name = filepath.Base(filePath)
	}

	dag, err := BuildDAG(wf.Jobs)
	if err != nil {
		return nil, fmt.Errorf("invalid job dependency graph in %q: %w", filePath, err)
	}
	wf.JobOrder = dag.TopologicalOrder

	return wf, nil
}

func parseJobs(jobsNode *yaml.Node) (map[string]*Job, error) {
	jobs := make(map[string]*Job)
	entries := flattenMapping(jobsNode)

	for jobID, jobValNode := range entries {
		jobValNode = derefNode(jobValNode)
		if jobValNode.Kind != yaml.MappingNode {
			continue
		}

		job, err := parseJob(jobID, jobValNode)
		if err != nil {
			return nil, fmt.Errorf("job %q: %w", jobID, err)
		}
		jobs[jobID] = job
	}

	return jobs, nil
}

func parseJob(jobID string, node *yaml.Node) (*Job, error) {
	job := &Job{
		ID:            jobID,
		Needs:         make([]string, 0),
		Steps:         make([]*Step, 0),
		Outputs:       make(map[string]string),
		MatrixParams:  make(map[string][]string),
		MatrixInclude: make([]map[string]string, 0),
		MatrixExclude: make([]map[string]string, 0),
		With:          make(map[string]interface{}),
	}

	entries := flattenMapping(node)
	for k, vNode := range entries {
		vNode = derefNode(vNode)
		switch k {
		case "name":
			job.Name = strings.TrimSpace(vNode.Value)
		case "runs-on":
			if vNode.Kind == yaml.ScalarNode {
				job.RunsOn = strings.TrimSpace(vNode.Value)
			} else if vNode.Kind == yaml.SequenceNode {
				var items []string
				for _, item := range vNode.Content {
					items = append(items, derefNode(item).Value)
				}
				job.RunsOn = strings.Join(items, ", ")
			}
		case "if":
			job.If = strings.TrimSpace(vNode.Value)
		case "environment":
			if vNode.Kind == yaml.ScalarNode {
				job.Environment = strings.TrimSpace(vNode.Value)
			} else if vNode.Kind == yaml.MappingNode {
				envEntries := flattenMapping(vNode)
				if nameNode, ok := envEntries["name"]; ok {
					job.Environment = strings.TrimSpace(derefNode(nameNode).Value)
				}
			}
		case "continue-on-error":
			job.ContinueOnError = parseBool(vNode)
		case "uses":
			job.Uses = strings.TrimSpace(vNode.Value)
		case "needs":
			job.Needs = parseNeeds(vNode)
		case "strategy":
			if vNode.Kind == yaml.MappingNode {
				parseStrategy(job, vNode)
			}
		case "outputs":
			if vNode.Kind == yaml.MappingNode {
				job.Outputs = parseOutputs(vNode)
			}
		case "steps":
			if vNode.Kind == yaml.SequenceNode {
				steps, err := parseSteps(vNode)
				if err != nil {
					return nil, err
				}
				if len(steps) > MaxStepsPerJob {
					return nil, fmt.Errorf("job %q exceeds step limit of %d", jobID, MaxStepsPerJob)
				}
				job.Steps = steps
			}
		case "with":
			if vNode.Kind == yaml.MappingNode {
				withEntries := flattenMapping(vNode)
				for wk, wv := range withEntries {
					job.With[wk] = nodeToInterface(wv)
				}
			}
		}
	}

	return job, nil
}

func parseNeeds(node *yaml.Node) []string {
	node = derefNode(node)
	rawMap := make(map[string]bool)

	switch node.Kind {
	case yaml.ScalarNode:
		val := strings.TrimSpace(node.Value)
		if val != "" {
			rawMap[val] = true
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			val := strings.TrimSpace(derefNode(item).Value)
			if val != "" {
				rawMap[val] = true
			}
		}
	}

	var needs []string
	for k := range rawMap {
		needs = append(needs, k)
	}
	sort.Strings(needs)
	return needs
}

func parseStrategy(job *Job, node *yaml.Node) {
	entries := flattenMapping(node)
	matrixNode, ok := entries["matrix"]
	if !ok {
		return
	}
	matrixNode = derefNode(matrixNode)
	if matrixNode.Kind != yaml.MappingNode {
		return
	}

	job.HasMatrix = true
	matrixEntries := flattenMapping(matrixNode)

	for k, v := range matrixEntries {
		v = derefNode(v)
		switch k {
		case "include":
			if v.Kind == yaml.SequenceNode {
				for _, item := range v.Content {
					item = derefNode(item)
					if item.Kind == yaml.MappingNode {
						m := make(map[string]string)
						for ik, iv := range flattenMapping(item) {
							m[ik] = strings.TrimSpace(derefNode(iv).Value)
						}
						job.MatrixInclude = append(job.MatrixInclude, m)
					}
				}
			}
		case "exclude":
			if v.Kind == yaml.SequenceNode {
				for _, item := range v.Content {
					item = derefNode(item)
					if item.Kind == yaml.MappingNode {
						m := make(map[string]string)
						for ik, iv := range flattenMapping(item) {
							m[ik] = strings.TrimSpace(derefNode(iv).Value)
						}
						job.MatrixExclude = append(job.MatrixExclude, m)
					}
				}
			}
		default:
			var vals []string
			if v.Kind == yaml.SequenceNode {
				for _, item := range v.Content {
					vals = append(vals, strings.TrimSpace(derefNode(item).Value))
				}
			} else if v.Kind == yaml.ScalarNode {
				vals = append(vals, strings.TrimSpace(v.Value))
			}
			job.MatrixParams[k] = vals
		}
	}
}

func parseOutputs(node *yaml.Node) map[string]string {
	outputs := make(map[string]string)
	for k, v := range flattenMapping(node) {
		outputs[k] = strings.TrimSpace(derefNode(v).Value)
	}
	return outputs
}

func parseSteps(node *yaml.Node) ([]*Step, error) {
	var steps []*Step
	for idx, itemNode := range node.Content {
		itemNode = derefNode(itemNode)
		if itemNode.Kind != yaml.MappingNode {
			continue
		}
		step := &Step{
			Index: idx,
			With:  make(map[string]interface{}),
			Env:   make(map[string]string),
		}

		entries := flattenMapping(itemNode)
		for k, vNode := range entries {
			vNode = derefNode(vNode)
			switch k {
			case "id":
				step.ID = strings.TrimSpace(vNode.Value)
			case "name":
				step.Name = strings.TrimSpace(vNode.Value)
			case "uses":
				step.Uses = strings.TrimSpace(vNode.Value)
				step.ActionOwner, step.ActionName, step.ActionRef = parseUses(step.Uses)
			case "run":
				step.Run = vNode.Value
			case "if":
				step.If = strings.TrimSpace(vNode.Value)
			case "continue-on-error":
				step.ContinueOnError = parseBool(vNode)
			case "with":
				if vNode.Kind == yaml.MappingNode {
					withEntries := flattenMapping(vNode)
					for wk, wv := range withEntries {
						step.With[wk] = nodeToInterface(wv)
					}
				}
			case "env":
				if vNode.Kind == yaml.MappingNode {
					envEntries := flattenMapping(vNode)
					for ek, ev := range envEntries {
						step.Env[ek] = strings.TrimSpace(derefNode(ev).Value)
					}
				}
			}
		}
		steps = append(steps, step)
	}
	return steps, nil
}

// parseUses decomposes a GitHub Actions "uses:" value into owner, name, ref.
// Examples:
//
//	"actions/upload-artifact@v4" -> ("actions", "upload-artifact", "v4")
//	"docker/build-push-action@v5.1.0" -> ("docker", "build-push-action", "v5.1.0")
//	"./.github/workflows/local.yml" -> (".", ".github/workflows/local.yml", "")
func parseUses(uses string) (owner, name, ref string) {
	uses = strings.TrimSpace(uses)
	if strings.HasPrefix(uses, "./") {
		return ".", strings.TrimPrefix(uses, "./"), ""
	}
	parts := strings.SplitN(uses, "@", 2)
	path := parts[0]
	if len(parts) > 1 {
		ref = parts[1]
	}
	pathParts := strings.Split(path, "/")
	if len(pathParts) >= 2 {
		owner = pathParts[0]
		name = strings.Join(pathParts[1:], "/")
	} else {
		name = path
	}
	return owner, name, ref
}

func parseBool(node *yaml.Node) bool {
	node = derefNode(node)
	val := strings.ToLower(strings.TrimSpace(node.Value))
	b, err := strconv.ParseBool(val)
	if err == nil {
		return b
	}
	return val == "true" || val == "1" || val == "yes"
}

func derefNode(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.AliasNode && node.Alias != nil {
		return derefNode(node.Alias)
	}
	return node
}

// flattenMapping handles normal mapping entries as well as YAML 1.1 merge keys (<<).
func flattenMapping(node *yaml.Node) map[string]*yaml.Node {
	entries := make(map[string]*yaml.Node)
	if node == nil || node.Kind != yaml.MappingNode {
		return entries
	}

	for i := 0; i < len(node.Content); i += 2 {
		kNode := derefNode(node.Content[i])
		vNode := derefNode(node.Content[i+1])
		key := strings.TrimSpace(kNode.Value)

		if key == "<<" {
			// Merge key: value can be a mapping or sequence of mappings
			if vNode.Kind == yaml.MappingNode {
				merged := flattenMapping(vNode)
				for mk, mv := range merged {
					if _, exists := entries[mk]; !exists {
						entries[mk] = mv
					}
				}
			} else if vNode.Kind == yaml.SequenceNode {
				for _, item := range vNode.Content {
					item = derefNode(item)
					if item.Kind == yaml.MappingNode {
						merged := flattenMapping(item)
						for mk, mv := range merged {
							if _, exists := entries[mk]; !exists {
								entries[mk] = mv
							}
						}
					}
				}
			}
			continue
		}

		entries[key] = vNode
	}

	return entries
}

func nodeToInterface(node *yaml.Node) interface{} {
	node = derefNode(node)
	switch node.Kind {
	case yaml.ScalarNode:
		return strings.TrimSpace(node.Value)
	case yaml.SequenceNode:
		var slice []interface{}
		for _, item := range node.Content {
			slice = append(slice, nodeToInterface(item))
		}
		return slice
	case yaml.MappingNode:
		m := make(map[string]interface{})
		for k, v := range flattenMapping(node) {
			m[k] = nodeToInterface(v)
		}
		return m
	default:
		return strings.TrimSpace(node.Value)
	}
}
