package analysis

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"ci-artifact-identity/internal/workflow"
)

// ArtifactProducer tracks an artifact uploaded or generated for downstream consumption.
type ArtifactProducer struct {
	WorkflowFile  string              `json:"workflowFile"`
	JobID         string              `json:"jobId"`
	Job           *workflow.Job       `json:"-"`
	StepIndex     int                 `json:"stepIndex"`
	StepName      string              `json:"stepName,omitempty"`
	StepID        string              `json:"stepId,omitempty"`
	ArtifactName  string              `json:"artifactName"`
	Path          string              `json:"path,omitempty"`
	IsExpression  bool                `json:"isExpression"`
	Kind          string              `json:"kind"`
	ActionVersion string              `json:"actionVersion,omitempty"`
	DigestSymbol  string              `json:"digestSymbol,omitempty"`
	Tags          string              `json:"tags,omitempty"`
	Overwrite     bool                `json:"overwrite,omitempty"`
	IfCondition   string              `json:"ifCondition,omitempty"`
	MatrixParams  map[string][]string `json:"matrixParams,omitempty"`
}

// ArtifactConsumer tracks an artifact downloaded or consumed for validation or deployment.
type ArtifactConsumer struct {
	WorkflowFile    string              `json:"workflowFile"`
	JobID           string              `json:"jobId"`
	Job             *workflow.Job       `json:"-"`
	StepIndex       int                 `json:"stepIndex"`
	StepName        string              `json:"stepName,omitempty"`
	StepID          string              `json:"stepId,omitempty"`
	ArtifactName    string              `json:"artifactName"`
	Destination     string              `json:"destination,omitempty"`
	DownloadsAll    bool                `json:"downloadsAll"`
	Pattern         string              `json:"pattern,omitempty"`
	MergeMultiple   bool                `json:"mergeMultiple,omitempty"`
	IsExpression    bool                `json:"isExpression"`
	Kind            string              `json:"kind"`
	ActionVersion   string              `json:"actionVersion,omitempty"`
	DigestSymbol    string              `json:"digestSymbol,omitempty"`
	IfCondition     string              `json:"ifCondition,omitempty"`
	ContinueOnError bool                `json:"continueOnError,omitempty"`
	MatrixParams    map[string][]string `json:"matrixParams,omitempty"`
}

var (
	reExpression    = regexp.MustCompile(`\$\{\{\s*([^}]+?)\s*\}\}`)
	reDigestRef     = regexp.MustCompile(`steps\.([a-zA-Z0-9_-]+)\.outputs\.digest`)
	reNeedsDigest   = regexp.MustCompile(`needs\.([a-zA-Z0-9_-]+)\.outputs\.digest`)
	reSha256Digest  = regexp.MustCompile(`sha256:[a-fA-F0-9]{64}`)
	reMatrixParam   = regexp.MustCompile(`matrix\.([a-zA-Z0-9_-]+)`)
	reDynamicDigest = regexp.MustCompile(`@\$\{\{\s*([^}]+?)\s*\}\}`)
)

// IsDynamicDigestRef checks if a digest symbol or reference is a dynamic expression
// rather than a deterministic step/needs output or fixed sha256.
func IsDynamicDigestRef(ref string) bool {
	if ref == "" {
		return false
	}
	if reSha256Digest.MatchString(ref) {
		return false
	}
	if reDigestRef.MatchString(ref) || reNeedsDigest.MatchString(ref) {
		return false
	}
	return strings.Contains(ref, "${{") && strings.Contains(ref, "}}") || strings.Contains(ref, "@${{")
}

// NormalizeExpression normalizes GitHub Actions expressions by trimming whitespace.
func NormalizeExpression(expr string) string {
	trimmed := strings.TrimSpace(expr)
	return reExpression.ReplaceAllStringFunc(trimmed, func(match string) string {
		sub := reExpression.FindStringSubmatch(match)
		if len(sub) > 1 {
			inner := strings.Join(strings.Fields(sub[1]), " ")
			return fmt.Sprintf("${{ %s }}", inner)
		}
		return match
	})
}

// IsGitHubExpression checks whether the string contains a GitHub Actions expression.
func IsGitHubExpression(s string) bool {
	return strings.Contains(s, "${{") && strings.Contains(s, "}}")
}

// CompareArtifactNames compares two artifact names.
// Returns:
//   - matched: true if they are structurally identical or match wildcard
//   - ambiguous: true if static resolution cannot verify equivalence (e.g. dynamic expressions)
func CompareArtifactNames(producerName, consumerName string, knownOutputs map[string]string) (matched bool, ambiguous bool) {
	prodNorm := NormalizeExpression(producerName)
	consNorm := NormalizeExpression(consumerName)

	if knownOutputs != nil {
		for k, v := range knownOutputs {
			placeholder := fmt.Sprintf("${{ needs.%s }}", k)
			prodNorm = strings.ReplaceAll(prodNorm, placeholder, v)
			consNorm = strings.ReplaceAll(consNorm, placeholder, v)
		}
	}

	if consNorm == "*" || consNorm == "" {
		return true, false
	}

	if prodNorm == consNorm {
		return true, false
	}

	if IsGitHubExpression(prodNorm) || IsGitHubExpression(consNorm) {
		return false, true
	}

	return false, false
}

// CompareMatrixContexts verifies that any matrix parameters referenced in the artifact name
// have identical discrete domains between the producer and consumer.
func CompareMatrixContexts(prodMatrix, consMatrix map[string][]string, prodName, consName string) (compatible bool, ambiguous bool) {
	prodMatches := reMatrixParam.FindAllStringSubmatch(prodName, -1)
	consMatches := reMatrixParam.FindAllStringSubmatch(consName, -1)

	if len(prodMatches) == 0 && len(consMatches) == 0 {
		return true, false
	}

	if len(prodMatches) != len(consMatches) {
		return false, true
	}

	for _, m := range prodMatches {
		param := m[1]
		pVals, pOk := prodMatrix[param]
		cVals, cOk := consMatrix[param]

		if !pOk || !cOk {
			return false, true
		}

		if len(pVals) != len(cVals) {
			return false, true
		}

		sort.Strings(pVals)
		sort.Strings(cVals)
		for i := range pVals {
			if pVals[i] != cVals[i] {
				return false, true
			}
		}
	}

	return true, false
}

// CompareConditions checks if the execution conditions between jobs/steps are statically compatible.
func CompareConditions(prodCond, consCond string) (compatible bool, ambiguous bool) {
	p := strings.TrimSpace(prodCond)
	c := strings.TrimSpace(consCond)

	if p == "" || c == "" {
		return true, false
	}

	if NormalizeExpression(p) == NormalizeExpression(c) {
		return true, false
	}

	// Non-empty conditions that differ cannot be proven compatible without full solver
	return false, true
}

// ExtractArtifactProducers extracts artifact producers from a job's steps.
func ExtractArtifactProducers(workflowFile string, job *workflow.Job) []*ArtifactProducer {
	var producers []*ArtifactProducer

	for _, step := range job.Steps {
		if step.Uses == "" {
			continue
		}
		owner, actionName, ref := step.ActionOwner, step.ActionName, step.ActionRef

		if (owner == "actions" && strings.HasPrefix(actionName, "upload-artifact")) ||
			strings.HasPrefix(strings.ToLower(step.Uses), "actions/upload-artifact") {

			name := "artifact"
			if n, ok := step.With["name"]; ok && n != nil {
				nameStr := strings.TrimSpace(fmt.Sprintf("%v", n))
				if nameStr != "" {
					name = nameStr
				}
			}
			pathStr := ""
			if p, ok := step.With["path"]; ok && p != nil {
				pathStr = strings.TrimSpace(fmt.Sprintf("%v", p))
			}

			overwrite := false
			if ow, ok := step.With["overwrite"]; ok && ow != nil {
				owStr := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", ow)))
				overwrite = owStr == "true" || owStr == "1"
			}

			normName := NormalizeExpression(name)
			effCondition := job.If
			if step.If != "" {
				if effCondition != "" {
					effCondition = fmt.Sprintf("(%s) && (%s)", effCondition, step.If)
				} else {
					effCondition = step.If
				}
			}

			producers = append(producers, &ArtifactProducer{
				WorkflowFile:  workflowFile,
				JobID:         job.ID,
				Job:           job,
				StepIndex:     step.Index,
				StepName:      step.Name,
				StepID:        step.ID,
				ArtifactName:  normName,
				Path:          pathStr,
				IsExpression:  IsGitHubExpression(normName),
				Kind:          "actions/upload-artifact",
				ActionVersion: ref,
				Overwrite:     overwrite,
				IfCondition:   effCondition,
				MatrixParams:  job.MatrixParams,
			})
		} else if strings.HasPrefix(strings.ToLower(step.Uses), "docker/build-push-action") {
			tagsStr := ""
			if t, ok := step.With["tags"]; ok && t != nil {
				tagsStr = strings.TrimSpace(fmt.Sprintf("%v", t))
			}
			digestSym := ""
			if step.ID != "" {
				digestSym = fmt.Sprintf("steps.%s.outputs.digest", step.ID)
			}

			producers = append(producers, &ArtifactProducer{
				WorkflowFile:  workflowFile,
				JobID:         job.ID,
				Job:           job,
				StepIndex:     step.Index,
				StepName:      step.Name,
				StepID:        step.ID,
				ArtifactName:  digestSym,
				IsExpression:  false,
				Kind:          "docker/build-push-action",
				ActionVersion: ref,
				DigestSymbol:  digestSym,
				Tags:          tagsStr,
				IfCondition:   job.If,
				MatrixParams:  job.MatrixParams,
			})
		} else if (owner == "actions" && strings.HasPrefix(actionName, "upload-pages-artifact")) ||
			strings.HasPrefix(strings.ToLower(step.Uses), "actions/upload-pages-artifact") {

			name := "github-pages"
			if n, ok := step.With["name"]; ok && n != nil {
				nameStr := strings.TrimSpace(fmt.Sprintf("%v", n))
				if nameStr != "" {
					name = nameStr
				}
			}
			pathStr := ""
			if p, ok := step.With["path"]; ok && p != nil {
				pathStr = strings.TrimSpace(fmt.Sprintf("%v", p))
			}

			normName := NormalizeExpression(name)
			effCondition := job.If
			if step.If != "" {
				if effCondition != "" {
					effCondition = fmt.Sprintf("(%s) && (%s)", effCondition, step.If)
				} else {
					effCondition = step.If
				}
			}

			producers = append(producers, &ArtifactProducer{
				WorkflowFile:  workflowFile,
				JobID:         job.ID,
				Job:           job,
				StepIndex:     step.Index,
				StepName:      step.Name,
				StepID:        step.ID,
				ArtifactName:  normName,
				Path:          pathStr,
				IsExpression:  IsGitHubExpression(normName),
				Kind:          "actions/upload-pages-artifact",
				ActionVersion: ref,
				IfCondition:   effCondition,
				MatrixParams:  job.MatrixParams,
			})
		}
	}

	return producers
}

// ExtractArtifactConsumers extracts artifact consumers from a job's steps.
func ExtractArtifactConsumers(workflowFile string, job *workflow.Job) []*ArtifactConsumer {
	var consumers []*ArtifactConsumer

	for _, step := range job.Steps {
		owner, actionName, ref := step.ActionOwner, step.ActionName, step.ActionRef

		// 1. Check actions/download-artifact
		if (owner == "actions" && strings.HasPrefix(actionName, "download-artifact")) ||
			strings.HasPrefix(strings.ToLower(step.Uses), "actions/download-artifact") {

			downloadsAll := true
			name := "*"
			if n, ok := step.With["name"]; ok && n != nil {
				nameStr := strings.TrimSpace(fmt.Sprintf("%v", n))
				if nameStr != "" {
					name = nameStr
					downloadsAll = false
				}
			}
			destStr := ""
			if p, ok := step.With["path"]; ok && p != nil {
				destStr = strings.TrimSpace(fmt.Sprintf("%v", p))
			}

			patternStr := ""
			if pat, ok := step.With["pattern"]; ok && pat != nil {
				patternStr = strings.TrimSpace(fmt.Sprintf("%v", pat))
			}

			mergeMultiple := false
			if mm, ok := step.With["merge-multiple"]; ok && mm != nil {
				mmStr := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", mm)))
				mergeMultiple = mmStr == "true" || mmStr == "1"
			}

			effCondition := job.If
			if step.If != "" {
				if effCondition != "" {
					effCondition = fmt.Sprintf("(%s) && (%s)", effCondition, step.If)
				} else {
					effCondition = step.If
				}
			}

			normName := NormalizeExpression(name)
			consumers = append(consumers, &ArtifactConsumer{
				WorkflowFile:    workflowFile,
				JobID:           job.ID,
				Job:             job,
				StepIndex:       step.Index,
				StepName:        step.Name,
				StepID:          step.ID,
				ArtifactName:    normName,
				Destination:     destStr,
				DownloadsAll:    downloadsAll,
				Pattern:         patternStr,
				MergeMultiple:   mergeMultiple,
				IsExpression:    IsGitHubExpression(normName),
				Kind:            "actions/download-artifact",
				ActionVersion:   ref,
				IfCondition:     effCondition,
				ContinueOnError: job.ContinueOnError || step.ContinueOnError,
				MatrixParams:    job.MatrixParams,
			})
			continue
		}

		// 2. Check actions/deploy-pages
		if (owner == "actions" && strings.HasPrefix(actionName, "deploy-pages")) ||
			strings.HasPrefix(strings.ToLower(step.Uses), "actions/deploy-pages") {

			name := "github-pages"
			if n, ok := step.With["artifact_name"]; ok && n != nil {
				nameStr := strings.TrimSpace(fmt.Sprintf("%v", n))
				if nameStr != "" {
					name = nameStr
				}
			}

			effCondition := job.If
			if step.If != "" {
				if effCondition != "" {
					effCondition = fmt.Sprintf("(%s) && (%s)", effCondition, step.If)
				} else {
					effCondition = step.If
				}
			}

			normName := NormalizeExpression(name)
			consumers = append(consumers, &ArtifactConsumer{
				WorkflowFile:    workflowFile,
				JobID:           job.ID,
				Job:             job,
				StepIndex:       step.Index,
				StepName:        step.Name,
				StepID:          step.ID,
				ArtifactName:    normName,
				DownloadsAll:    false,
				IsExpression:    IsGitHubExpression(normName),
				Kind:            "actions/deploy-pages",
				ActionVersion:   ref,
				IfCondition:     effCondition,
				ContinueOnError: job.ContinueOnError || step.ContinueOnError,
				MatrixParams:    job.MatrixParams,
			})
			continue
		}

		// 2. Check for docker digest references
		contentToScan := step.Run
		for _, v := range step.With {
			contentToScan += " " + fmt.Sprintf("%v", v)
		}
		for _, v := range step.Env {
			contentToScan += " " + v
		}

		digestMatch := ""
		if match := reDigestRef.FindString(contentToScan); match != "" {
			digestMatch = match
		} else if match := reNeedsDigest.FindString(contentToScan); match != "" {
			digestMatch = match
		} else if match := reSha256Digest.FindString(contentToScan); match != "" {
			digestMatch = match
		} else if match := reDynamicDigest.FindString(contentToScan); match != "" {
			digestMatch = match
		}

		if digestMatch != "" {
			consumers = append(consumers, &ArtifactConsumer{
				WorkflowFile:    workflowFile,
				JobID:           job.ID,
				Job:             job,
				StepIndex:       step.Index,
				StepName:        step.Name,
				StepID:          step.ID,
				ArtifactName:    digestMatch,
				DigestSymbol:    digestMatch,
				Kind:            "docker-digest",
				IfCondition:     job.If,
				ContinueOnError: job.ContinueOnError || step.ContinueOnError,
				MatrixParams:    job.MatrixParams,
			})
		}
	}

	return consumers
}
