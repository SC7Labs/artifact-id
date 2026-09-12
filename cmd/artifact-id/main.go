package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ci-artifact-identity/internal/analysis"
	"ci-artifact-identity/internal/report"
	"ci-artifact-identity/internal/workflow"
)

const (
	Version = "0.1.0"

	ExitSuccess    = 0
	ExitIdentIssue = 1
	ExitInvocation = 2
)

func main() {
	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

func run(args []string) int {
	fs := flag.NewFlagSet("artifact-id", flag.ContinueOnError)

	var (
		formatFlag   string
		workflowFlag string
		versionFlag  bool
		helpFlag     bool
	)

	fs.StringVar(&formatFlag, "format", "text", "Output format: 'text' or 'json'")
	fs.StringVar(&workflowFlag, "workflow", "", "Path to a specific workflow file to analyze")
	fs.BoolVar(&versionFlag, "version", false, "Print version and exit")
	fs.BoolVar(&helpFlag, "help", false, "Print help and exit")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Artifact-ID %s — Static CI Artifact Identity Analyzer\n\n", Version)
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  artifact-id [flags] [repository-path]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return ExitInvocation
	}

	if helpFlag {
		fs.Usage()
		return ExitSuccess
	}

	if versionFlag {
		fmt.Printf("Artifact-ID %s\n", Version)
		return ExitSuccess
	}

	format := strings.ToLower(strings.TrimSpace(formatFlag))
	if format != "text" && format != "json" {
		fmt.Fprintf(os.Stderr, "Error: invalid format %q. Supported formats: 'text', 'json'\n", formatFlag)
		return ExitInvocation
	}

	repoPath := "."
	posArgs := fs.Args()
	if len(posArgs) > 0 {
		repoPath = posArgs[0]
	} else if workflowFlag != "" {
		wfAbs, err := filepath.Abs(workflowFlag)
		if err == nil {
			dir := filepath.Dir(wfAbs)
			if filepath.Base(dir) == "workflows" && filepath.Base(filepath.Dir(dir)) == ".github" {
				repoPath = filepath.Dir(filepath.Dir(dir))
			} else {
				repoPath = filepath.Dir(wfAbs)
			}
		}
	}

	repoAbs, err := filepath.Abs(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot resolve repository path %q: %v\n", repoPath, err)
		return ExitInvocation
	}

	repoInfo, err := os.Stat(repoAbs)
	if err != nil || !repoInfo.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: repository path %q does not exist or is not a directory\n", repoPath)
		return ExitInvocation
	}

	// Resolve real repository path to prevent symlink traversal escapes
	realRepoAbs, err := filepath.EvalSymlinks(repoAbs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot resolve symlinks for repository path %q: %v\n", repoPath, err)
		return ExitInvocation
	}

	var workflowFiles []string

	if workflowFlag != "" {
		wfPath, err := filepath.Abs(workflowFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot resolve workflow path %q: %v\n", workflowFlag, err)
			return ExitInvocation
		}
		wfInfo, err := os.Stat(wfPath)

		if err != nil || wfInfo.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: requested workflow %q does not exist\n", workflowFlag)
			return ExitInvocation
		}

		evalWfPath, err := filepath.EvalSymlinks(wfPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot evaluate symlink for workflow %q: %v\n", wfPath, err)
			return ExitInvocation
		}
		rel, err := filepath.Rel(realRepoAbs, evalWfPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			fmt.Fprintf(os.Stderr, "Error: workflow %q links outside repository\n", wfPath)
			return ExitInvocation
		}

		workflowFiles = append(workflowFiles, wfPath)
	} else {
		wfDir := filepath.Join(repoAbs, ".github", "workflows")
		wfDirInfo, err := os.Stat(wfDir)
		if err == nil && wfDirInfo.IsDir() {
			evalWfDir, err := filepath.EvalSymlinks(wfDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot evaluate symlink for workflows directory %q: %v\n", wfDir, err)
				return ExitInvocation
			}
			relDir, err := filepath.Rel(realRepoAbs, evalWfDir)
			if err != nil || strings.HasPrefix(relDir, "..") {
				fmt.Fprintf(os.Stderr, "Error: workflows directory %q links outside repository\n", wfDir)
				return ExitInvocation
			}

			entries, err := os.ReadDir(wfDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot read directory %q: %v\n", wfDir, err)
				return ExitInvocation
			}

			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(name))
				if ext == ".yml" || ext == ".yaml" {
					fullPath := filepath.Join(wfDir, name)

					evalPath, err := filepath.EvalSymlinks(fullPath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: cannot evaluate symlink %q: %v\n", fullPath, err)
						return ExitInvocation
					}
					rel, err := filepath.Rel(realRepoAbs, evalPath)
					if err != nil || strings.HasPrefix(rel, "..") {
						fmt.Fprintf(os.Stderr, "Error: workflow %q links outside repository\n", fullPath)
						return ExitInvocation
					}

					workflowFiles = append(workflowFiles, fullPath)
					if len(workflowFiles) > workflow.MaxWorkflowCount {
						fmt.Fprintf(os.Stderr, "Error: repository exceeds maximum workflow limit of %d\n", workflow.MaxWorkflowCount)
						return ExitInvocation
					}
				}
			}
		}
	}

	sort.Strings(workflowFiles)

	var workflows []*workflow.Workflow
	for _, wfPath := range workflowFiles {
		wf, err := workflow.ParseWorkflowFile(wfPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return ExitInvocation
		}
		rel, err := filepath.Rel(repoAbs, wfPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			wf.Path = rel
		}
		workflows = append(workflows, wf)
	}

	result, err := analysis.AnalyzeWorkflows(repoPath, workflows)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitInvocation
	}

	if format == "json" {
		if err := report.RenderJSON(os.Stdout, result); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to render JSON: %v\n", err)
			return ExitInvocation
		}
	} else {
		if err := report.RenderText(os.Stdout, result); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to render text: %v\n", err)
			return ExitInvocation
		}
	}

	if result.Summary.Failures > 0 || result.Summary.Warnings > 0 || result.Summary.Unverified > 0 {
		return ExitIdentIssue
	}

	return ExitSuccess
}
