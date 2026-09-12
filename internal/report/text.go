package report

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"ci-artifact-identity/internal/analysis"
)

// RenderText writes a clean, compact terminal report of the analysis results.
func RenderText(w io.Writer, res *analysis.AnalysisResult) error {
	fmt.Fprintf(w, "Artifact-ID %s\n", res.ToolVersion)
	fmt.Fprintf(w, "Repository: %s\n", res.Repository)
	fmt.Fprintf(w, "Workflows analyzed: %d\n\n", res.WorkflowCount)

	for _, wf := range res.Workflows {
		relName := filepath.Base(wf.Path)
		fmt.Fprintf(w, "%s\n\n", relName)

		var nonPassFindings []*analysis.Finding
		for _, f := range wf.Findings {
			if f.Status != analysis.StatusPass {
				nonPassFindings = append(nonPassFindings, f)
			}
		}

		if len(nonPassFindings) > 0 {
			for _, f := range nonPassFindings {
				fmt.Fprintf(w, "%-6s %s\n", f.RuleID, f.Status)
				fmt.Fprintf(w, "Workflow: %s\n", relName)
				if f.JobID != "" {
					fmt.Fprintf(w, "Deploy job: %s\n", f.JobID)
				}
				fmt.Fprintln(w)

				if f.Status == analysis.StatusUnverified {
					fmt.Fprintln(w, "Artifact identity could not be established.")
					if f.Reason != "" {
						fmt.Fprintf(w, "\nReason:\n  %s\n", f.Reason)
					}
				} else {
					fmt.Fprintf(w, "%s\n", f.Message)
				}
				fmt.Fprintln(w)

				if f.Evidence != "" {
					fmt.Fprintln(w, "Evidence:")
					lines := strings.Split(f.Evidence, "\n")
					for _, line := range lines {
						fmt.Fprintf(w, "  %s\n", line)
					}
					fmt.Fprintln(w)
				}

				if f.WhyItMatters != "" {
					fmt.Fprintln(w, "Why it matters:")
					fmt.Fprintf(w, "  %s\n\n", f.WhyItMatters)
				}

				if f.Remediation != "" {
					fmt.Fprintln(w, "Suggested action:")
					fmt.Fprintf(w, "  %s\n\n", f.Remediation)
				}
			}

			if wf.Status == "FAIL" {
				fmt.Fprintln(w, "Artifact identity: NOT ESTABLISHED")
				fmt.Fprintln(w)
			} else if wf.Status == "UNVERIFIED" {
				fmt.Fprintln(w, "Artifact identity: UNVERIFIED")
				fmt.Fprintln(w)
			}
		} else if len(wf.ArtifactFlows) > 0 {
			for _, flow := range wf.ArtifactFlows {
				if flow.Status == analysis.FlowEstablished {
					fmt.Fprintln(w, "Artifact chain")
					prodJob := "build"
					if flow.Producer != nil {
						prodJob = flow.Producer.JobID
					}
					fmt.Fprintf(w, "  %s\n", prodJob)
					fmt.Fprintf(w, "    └─ %s\n", flow.ArtifactName)
					fmt.Fprintln(w, "       ↓")

					valJob := "test"
					if len(flow.Validators) > 0 {
						valJob = flow.Validators[0].JobID
					}
					fmt.Fprintf(w, "  %s\n", valJob)
					fmt.Fprintf(w, "    └─ %s\n", flow.ArtifactName)
					fmt.Fprintln(w, "       ↓")

					depJob := "deploy"
					if len(flow.Deployers) > 0 {
						depJob = flow.Deployers[0].JobID
					}
					fmt.Fprintf(w, "  %s\n", depJob)
					fmt.Fprintf(w, "    └─ %s\n\n", flow.ArtifactName)

					fmt.Fprintln(w, "PASS  Artifact identity established")
					fmt.Fprintln(w)
				}
			}
		} else if wf.Status == "NOT_APPLICABLE" {
			fmt.Fprintln(w, "(No deployment flow detected - not applicable)")
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintln(w, "Summary")
	fmt.Fprintf(w, "  workflows analyzed: %d\n", res.Summary.WorkflowsAnalyzed)
	fmt.Fprintf(w, "  artifact chains:    %d\n", res.Summary.ArtifactChains)
	fmt.Fprintf(w, "  failures:           %d\n", res.Summary.Failures)
	fmt.Fprintf(w, "  warnings:           %d\n", res.Summary.Warnings)
	fmt.Fprintf(w, "  unverified:         %d\n", res.Summary.Unverified)

	return nil
}
