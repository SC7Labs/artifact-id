package analysis

import (
	"fmt"
	"sort"
	"strings"

	"ci-artifact-identity/internal/workflow"
)

// WorkflowReport contains the analysis findings and artifact flows for a single workflow.
type WorkflowReport struct {
	Path          string          `json:"path"`
	Name          string          `json:"name"`
	Status        string          `json:"status"` // PASS, WARNING, FAIL, UNVERIFIED, NOT_APPLICABLE
	Jobs          []string        `json:"jobs"`
	ArtifactFlows []*ArtifactFlow `json:"artifactFlows"`
	Findings      []*Finding      `json:"findings"`
}

// AnalysisResult contains the overall analysis results across all workflows.
type AnalysisResult struct {
	SchemaVersion string            `json:"schemaVersion"`
	ToolVersion   string            `json:"toolVersion"`
	Repository    string            `json:"repository"`
	WorkflowCount int               `json:"workflowCount"`
	Workflows     []*WorkflowReport `json:"workflows"`
	ArtifactFlows []*ArtifactFlow   `json:"artifactFlows"`
	Findings      []*Finding        `json:"findings"`
	Summary       Summary           `json:"summary"`
}

// Summary aggregates counts of findings and flows.
type Summary struct {
	WorkflowsAnalyzed int `json:"workflowsAnalyzed"`
	ArtifactChains    int `json:"artifactChains"`
	Failures          int `json:"failures"`
	Warnings          int `json:"warnings"`
	Unverified        int `json:"unverified"`
}

// JobAnalysisData holds extracted information for a single job.
type JobAnalysisData struct {
	Job               *workflow.Job
	BuildEvents       []BuildEvent
	ValidationEvents  []ValidationEvent
	DeploymentEvents  []DeploymentEvent
	ArtifactProducers []*ArtifactProducer
	ArtifactConsumers []*ArtifactConsumer
}

// AnalyzeWorkflow analyzes a single workflow for artifact identity and flow consistency.
func AnalyzeWorkflow(wf *workflow.Workflow) (*WorkflowReport, error) {
	dag, err := workflow.BuildDAG(wf.Jobs)
	if err != nil {
		return nil, err
	}

	jobData := make(map[string]*JobAnalysisData)
	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		data := &JobAnalysisData{
			Job:               job,
			BuildEvents:       make([]BuildEvent, 0),
			ValidationEvents:  make([]ValidationEvent, 0),
			DeploymentEvents:  make([]DeploymentEvent, 0),
			ArtifactProducers: ExtractArtifactProducers(wf.Path, job),
			ArtifactConsumers: ExtractArtifactConsumers(wf.Path, job),
		}

		for _, step := range job.Steps {
			data.BuildEvents = append(data.BuildEvents, ClassifyBuildStep(jobID, step)...)
			data.ValidationEvents = append(data.ValidationEvents, ClassifyValidationStep(jobID, step)...)
			data.DeploymentEvents = append(data.DeploymentEvents, ClassifyDeploymentStep(jobID, step)...)
		}
		jobData[jobID] = data
	}

	findings := make([]*Finding, 0)
	flows := make([]*ArtifactFlow, 0)

	// Check for deployment events across all jobs
	var deployJobIDs []string
	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		hasDeployEvents := len(jobData[jobID].DeploymentEvents) > 0
		hasEnv := job.Environment != ""
		hasDeployName := isDeploymentJobName(jobID, job.Name)
		hasConsumers := len(jobData[jobID].ArtifactConsumers) > 0

		if hasDeployEvents || hasEnv || (hasDeployName && (hasConsumers || len(dag.GetTransitiveUpstream(jobID)) > 0)) {
			deployJobIDs = append(deployJobIDs, jobID)
		}
	}

	if len(deployJobIDs) == 0 {
		report := &WorkflowReport{
			Path:          wf.Path,
			Name:          wf.Name,
			Status:        "NOT_APPLICABLE",
			Jobs:          wf.JobOrder,
			ArtifactFlows: []*ArtifactFlow{},
			Findings:      []*Finding{},
		}
		return report, nil
	}

	// Gather all producers in the entire workflow to detect global name collisions
	producersByName := make(map[string][]*ArtifactProducer)
	for _, jobID := range wf.JobOrder {
		for _, prod := range jobData[jobID].ArtifactProducers {
			producersByName[prod.ArtifactName] = append(producersByName[prod.ArtifactName], prod)
		}
	}

	for _, deployJobID := range deployJobIDs {
		deployData := jobData[deployJobID]
		upstreamJobIDs := dag.GetTransitiveUpstream(deployJobID)

		// Check for opaque action in deployment job
		for _, step := range deployData.Job.Steps {
			if step.Uses != "" && !isBenignAction(step.Uses) {
				findings = append(findings, &Finding{
					RuleID:       RuleArtifactIdentityUnverified,
					Status:       StatusUnverified,
					WorkflowFile: wf.Path,
					JobID:        deployJobID,
					StepIndex:    &step.Index,
					StepName:     step.Name,
					Reason:       ReasonOpaqueAction,
					Message:      "Deployment relies on an opaque action.",
					Evidence:     fmt.Sprintf("deployment job %q executes opaque action %q", deployJobID, step.Uses),
					WhyItMatters: "Artifacts deployed via external or local custom actions cannot be statically verified.",
					Remediation:  "Use standard deployment commands or verified deployment actions.",
				})
			}
		}

		// Check for opaque reusable workflow in deploy or upstream
		for upID := range upstreamJobIDs {
			upJob := wf.Jobs[upID]
			if upJob.Uses != "" {
				findings = append(findings, &Finding{
					RuleID:       RuleArtifactIdentityUnverified,
					Status:       StatusUnverified,
					WorkflowFile: wf.Path,
					JobID:        deployJobID,
					Reason:       ReasonOpaqueReusableWorkflow,
					Message:      "Upstream job calls an opaque reusable workflow.",
					Evidence:     fmt.Sprintf("upstream job %q calls reusable workflow %q", upID, upJob.Uses),
					WhyItMatters: "Artifact transfer inside or across reusable workflows cannot be statically verified.",
					Remediation:  "Establish explicit artifact upload/download steps in the calling workflow.",
				})
			}
		}
		if deployData.Job.Uses != "" {
			findings = append(findings, &Finding{
				RuleID:       RuleArtifactIdentityUnverified,
				Status:       StatusUnverified,
				WorkflowFile: wf.Path,
				JobID:        deployJobID,
				Reason:       ReasonOpaqueReusableWorkflow,
				Message:      "Deployment job calls an opaque reusable workflow.",
				Evidence:     fmt.Sprintf("job %q calls reusable workflow %q", deployJobID, deployData.Job.Uses),
				WhyItMatters: "Artifacts consumed inside external reusable workflows cannot be statically analyzed without resolving the external repository.",
				Remediation:  "Inline the workflow or reference local reusable workflows with static analysis.",
			})
		}

		// Collect upstream validation events and producers
		var upstreamValJobs []string
		var upstreamProducers []*ArtifactProducer
		var upstreamValidators []*ArtifactConsumer
		hasUnblockedValidation := false

		for _, upID := range wf.JobOrder {
			if !upstreamJobIDs[upID] {
				continue
			}
			upData := jobData[upID]

			hasPrimaryVal := false
			for _, v := range upData.ValidationEvents {
				if !v.IsSecondaryLint {
					hasPrimaryVal = true
					break
				}
			}
			if hasPrimaryVal {
				upstreamValJobs = append(upstreamValJobs, upID)
				if upData.Job.ContinueOnError {
					hasUnblockedValidation = true
				}
				for _, step := range upData.Job.Steps {
					if step.ContinueOnError {
						for _, v := range upData.ValidationEvents {
							if v.StepIndex == step.Index && !v.IsSecondaryLint {
								hasUnblockedValidation = true
							}
						}
					}
				}
				// Consumers in this verified validation job are genuine validators
				upstreamValidators = append(upstreamValidators, upData.ArtifactConsumers...)
			} else {
				// If job has docker-digest consumer, preserve it for docker digest analysis
				for _, c := range upData.ArtifactConsumers {
					if c.Kind == "docker-digest" {
						upstreamValidators = append(upstreamValidators, c)
					}
				}
				// For file artifact consumers without primary validation, check if it executed an opaque action
				hasFileConsumers := false
				for _, c := range upData.ArtifactConsumers {
					if c.Kind != "docker-digest" {
						hasFileConsumers = true
						break
					}
				}
				if hasFileConsumers {
					hasOpaque := false
					var opaqueStep *workflow.Step
					for _, step := range upData.Job.Steps {
						if step.Uses != "" && !isBenignAction(step.Uses) {
							hasOpaque = true
							opaqueStep = step
							break
						}
					}
					if hasOpaque {
						findings = append(findings, &Finding{
							RuleID:       RuleArtifactIdentityUnverified,
							Status:       StatusUnverified,
							WorkflowFile: wf.Path,
							JobID:        upID,
							StepIndex:    &opaqueStep.Index,
							StepName:     opaqueStep.Name,
							Reason:       ReasonOpaqueAction,
							Message:      "Artifact validation relies on an opaque action.",
							Evidence:     fmt.Sprintf("upstream job %q downloads artifact but validation relies on opaque action %q", upID, opaqueStep.Uses),
							WhyItMatters: "Static analysis cannot verify artifact handling inside local or external custom actions.",
							Remediation:  "Use standard validation commands or verified runner actions.",
						})
					}
				}
			}

			upstreamProducers = append(upstreamProducers, upData.ArtifactProducers...)
		}

		// Check continue-on-error in validation
		if hasUnblockedValidation {
			findings = append(findings, &Finding{
				RuleID:       RuleArtifactIdentityUnverified,
				Status:       StatusUnverified,
				WorkflowFile: wf.Path,
				JobID:        deployJobID,
				Reason:       ReasonConditionalPathUnresolved,
				Message:      "Upstream validation has continue-on-error enabled.",
				Evidence:     "validation step or job has continue-on-error: true, allowing failed validation to proceed to deployment",
				WhyItMatters: "Artifact identity requires proof that the deployed artifact passed mandatory validation.",
				Remediation:  "Remove continue-on-error from validation jobs and steps.",
			})
		}

		// Validation in the same job prior to deployment
		if len(deployData.ValidationEvents) > 0 {
			for _, v := range deployData.ValidationEvents {
				if !v.IsSecondaryLint {
					upstreamValJobs = append(upstreamValJobs, deployJobID)
					break
				}
			}
		}

		// Rule AI001: REBUILD_AFTER_VALIDATION
		// Check whether deploy job contains a non-auxiliary build command
		if len(upstreamValJobs) > 0 {
			var nonAuxBuilds []BuildEvent
			for _, b := range deployData.BuildEvents {
				if !b.IsAuxiliary {
					nonAuxBuilds = append(nonAuxBuilds, b)
				}
			}

			if len(nonAuxBuilds) > 0 {
				firstBuild := nonAuxBuilds[0]
				stepIdx := firstBuild.StepIndex
				stepName := firstBuild.StepName
				if stepName == "" {
					stepName = fmt.Sprintf("Step %d", stepIdx+1)
				}

				valJobSummary := strings.Join(upstreamValJobs, ", ")
				findings = append(findings, &Finding{
					RuleID:       RuleRebuildAfterValidation,
					Status:       StatusFail,
					WorkflowFile: wf.Path,
					JobID:        deployJobID,
					StepIndex:    &stepIdx,
					StepName:     stepName,
					Message:      "Deployment rebuilds after validation.",
					Evidence:     fmt.Sprintf("validation job: %s\ndownstream build: %s\ndeployment job: %s", valJobSummary, firstBuild.Command, deployJobID),
					WhyItMatters: "Deployment rebuilds after validation, so the tool cannot establish that the deployed bytes are the bytes that passed validation.",
					Remediation:  "Download and deploy the pre-built artifact validated in upstream jobs instead of rebuilding.",
				})

				flow := &ArtifactFlow{
					Workflow:     wf.Path,
					ArtifactName: "unnamed-rebuilt-artifact",
					Status:       FlowRebuild,
					Notes:        fmt.Sprintf("Job %s rebuilds with %q after validation in %s", deployJobID, firstBuild.Command, valJobSummary),
				}
				flows = append(flows, flow)
				continue
			}
		}

		// Check for Docker digest flows
		isDockerFlow := false
		for _, upProd := range upstreamProducers {
			if upProd.Kind == "docker/build-push-action" && upProd.DigestSymbol != "" {
				isDockerFlow = true
				digestValidated := false
				var valJobName string
				var valDigestRef string

				for _, valCons := range upstreamValidators {
					if valCons.Kind == "docker-digest" {
						digestValidated = true
						valJobName = valCons.JobID
						valDigestRef = valCons.DigestSymbol
						break
					}
				}

				// Find deployer docker references
				var deployDigestCons *ArtifactConsumer
				for _, depCons := range deployData.ArtifactConsumers {
					if depCons.Kind == "docker-digest" {
						deployDigestCons = depCons
						break
					}
				}

				if digestValidated && deployDigestCons != nil {
					if IsDynamicDigestRef(valDigestRef) || IsDynamicDigestRef(deployDigestCons.DigestSymbol) {
						findings = append(findings, &Finding{
							RuleID:       RuleArtifactIdentityUnverified,
							Status:       StatusUnverified,
							WorkflowFile: wf.Path,
							JobID:        deployJobID,
							StepIndex:    &deployDigestCons.StepIndex,
							StepName:     deployDigestCons.StepName,
							Reason:       ReasonDynamicDigest,
							Message:      "Docker digest reference is dynamic and cannot be verified statically.",
							Evidence:     fmt.Sprintf("validated digest: %s\ndeployed digest: %s", valDigestRef, deployDigestCons.DigestSymbol),
							WhyItMatters: "Dynamic expressions for container digests prevent static verification of image identity.",
							Remediation:  "Reference a deterministic step or job output: ${{ steps.<step_id>.outputs.digest }}.",
						})

						flows = append(flows, &ArtifactFlow{
							Workflow:     wf.Path,
							ArtifactName: upProd.DigestSymbol,
							Producer:     upProd,
							Deployers:    []*ArtifactConsumer{deployDigestCons},
							Status:       FlowUnverified,
							Reason:       ReasonDynamicDigest,
							Notes:        "Dynamic digest reference in deployment or validation",
						})
					} else if valDigestRef == deployDigestCons.DigestSymbol {
						flow := &ArtifactFlow{
							Workflow:     wf.Path,
							ArtifactName: upProd.DigestSymbol,
							Producer:     upProd,
							Deployers:    []*ArtifactConsumer{deployDigestCons},
							Status:       FlowEstablished,
							Notes:        "Docker digest reference verified from build to deployment",
						}
						flows = append(flows, flow)

						findings = append(findings, &Finding{
							RuleID:       RuleArtifactChainEstablished,
							Status:       StatusPass,
							WorkflowFile: wf.Path,
							JobID:        deployJobID,
							StepIndex:    &deployDigestCons.StepIndex,
							StepName:     deployDigestCons.StepName,
							Message:      "Artifact chain established via immutable Docker digest.",
							Evidence:     fmt.Sprintf("Docker digest '%s' validated in '%s' and consumed in deployment job '%s'", upProd.DigestSymbol, valJobName, deployJobID),
							WhyItMatters: "Statically confirms that the immutable digest validated is referenced in deployment.",
						})
					} else {
						// Statically mismatched digests: AI002 FAIL
						findings = append(findings, &Finding{
							RuleID:       RuleTestedAndDeployedArtifactMismatch,
							Status:       StatusFail,
							WorkflowFile: wf.Path,
							JobID:        deployJobID,
							StepIndex:    &deployDigestCons.StepIndex,
							StepName:     deployDigestCons.StepName,
							Message:      "Tested and deployed container digests mismatch.",
							Evidence:     fmt.Sprintf("validated digest: %s\ndeployed digest: %s", valDigestRef, deployDigestCons.DigestSymbol),
							WhyItMatters: "The container image digest validated is not the digest deployed.",
							Remediation:  "Deploy the same container digest validated during testing.",
						})
					}
				} else if digestValidated && deployDigestCons == nil {
					// Check if deployment references mutable tag: AI005 WARNING
					findings = append(findings, &Finding{
						RuleID:       RuleMutableImageReference,
						Status:       StatusWarning,
						WorkflowFile: wf.Path,
						JobID:        deployJobID,
						Message:      "Mutable image tag used for deployment after digest validation.",
						Evidence:     fmt.Sprintf("validated digest '%s' in job '%s', but deployment in job '%s' uses mutable tag '%s'", upProd.DigestSymbol, valJobName, deployJobID, upProd.Tags),
						WhyItMatters: "Mutable tags can be overwritten or drift between validation and deployment, whereas image digests provide immutable identity.",
						Remediation:  fmt.Sprintf("Reference the immutable digest output ${{ %s }} in deployment step.", upProd.DigestSymbol),
					})

					flows = append(flows, &ArtifactFlow{
						Workflow:     wf.Path,
						ArtifactName: upProd.Tags,
						Producer:     upProd,
						Status:       FlowUnverified,
						Reason:       ReasonUnsupportedArtifactSemantics,
						Notes:        "Mutable tag used instead of validated digest",
					})
				}
			}
		}

		if isDockerFlow {
			continue
		}

		// Check standard file artifact downloads in deploy job
		deployConsumers := deployData.ArtifactConsumers

		if len(deployConsumers) == 0 {
			if len(upstreamValJobs) > 0 && len(upstreamProducers) > 0 {
				findings = append(findings, &Finding{
					RuleID:       RuleArtifactIdentityUnverified,
					Status:       StatusUnverified,
					WorkflowFile: wf.Path,
					JobID:        deployJobID,
					Reason:       ReasonUnsupportedArtifactSemantics,
					Message:      "Deployment does not consume validated upstream artifact.",
					Evidence:     fmt.Sprintf("upstream artifacts uploaded but deployment job '%s' downloads none", deployJobID),
					WhyItMatters: "Without downloading the verified artifact, deployment cannot be proven to use tested bytes.",
					Remediation:  "Add actions/download-artifact in deployment job to use the tested build artifact.",
				})
			}
			continue
		}

		// Distinct upstream artifact names count
		distinctUpstreamNames := make(map[string]bool)
		for _, p := range upstreamProducers {
			distinctUpstreamNames[p.ArtifactName] = true
		}

		for _, depConsumer := range deployConsumers {
			// Check for unsupported pattern / merge-multiple semantics
			if depConsumer.Pattern != "" {
				findings = append(findings, &Finding{
					RuleID:       RuleArtifactIdentityUnverified,
					Status:       StatusUnverified,
					WorkflowFile: wf.Path,
					JobID:        deployJobID,
					StepIndex:    &depConsumer.StepIndex,
					Reason:       ReasonUnsupportedArtifactSemantics,
					Message:      "download-artifact uses dynamic pattern matching.",
					Evidence:     fmt.Sprintf("pattern: %q in step %s", depConsumer.Pattern, depConsumer.StepName),
					WhyItMatters: "Pattern matching downloads arbitrary matching artifacts that cannot be uniquely verified statically.",
					Remediation:  "Specify explicit artifact names without wildcard patterns.",
				})
				continue
			}

			if depConsumer.MergeMultiple {
				findings = append(findings, &Finding{
					RuleID:       RuleArtifactIdentityUnverified,
					Status:       StatusUnverified,
					WorkflowFile: wf.Path,
					JobID:        deployJobID,
					StepIndex:    &depConsumer.StepIndex,
					Reason:       ReasonUnsupportedArtifactSemantics,
					Message:      "download-artifact has merge-multiple enabled.",
					Evidence:     "merge-multiple: true in download-artifact",
					WhyItMatters: "Merging multiple artifacts into one destination destroys isolated artifact identity.",
					Remediation:  "Download and deploy discrete artifacts individually without merging.",
				})
				continue
			}

			// Handle wildcard / download all
			if depConsumer.DownloadsAll {
				if len(distinctUpstreamNames) > 1 {
					findings = append(findings, &Finding{
						RuleID:       RuleArtifactIdentityUnverified,
						Status:       StatusUnverified,
						WorkflowFile: wf.Path,
						JobID:        deployJobID,
						StepIndex:    &depConsumer.StepIndex,
						StepName:     depConsumer.StepName,
						Reason:       ReasonDownloadAllMultipleArtifacts,
						Message:      "download-artifact without name is ambiguous when multiple artifacts exist.",
						Evidence:     fmt.Sprintf("deploy downloads all artifacts (*), but upstream produces %d distinct artifacts", len(distinctUpstreamNames)),
						WhyItMatters: "Wildcard download cannot prove which specific artifact is validated or deployed.",
						Remediation:  "Specify the exact artifact name in actions/download-artifact with: name: <artifact>.",
					})
					continue
				}
			}

			// Find matching upstream producers
			var matchingProducers []*ArtifactProducer
			for _, prod := range upstreamProducers {
				matched, _ := CompareArtifactNames(prod.ArtifactName, depConsumer.ArtifactName, nil)
				if matched {
					matchingProducers = append(matchingProducers, prod)
				}
			}

			// Check for ambiguous producers (multiple producers with same artifact name)
			if len(matchingProducers) > 1 {
				var prodJobNames []string
				for _, p := range matchingProducers {
					prodJobNames = append(prodJobNames, p.JobID)
				}
				findings = append(findings, &Finding{
					RuleID:       RuleArtifactIdentityUnverified,
					Status:       StatusUnverified,
					WorkflowFile: wf.Path,
					JobID:        deployJobID,
					StepIndex:    &depConsumer.StepIndex,
					StepName:     depConsumer.StepName,
					Reason:       ReasonAmbiguousProducer,
					Message:      fmt.Sprintf("Artifact %q is produced by multiple upstream jobs.", depConsumer.ArtifactName),
					Evidence:     fmt.Sprintf("producers: %s", strings.Join(prodJobNames, ", ")),
					WhyItMatters: "Artifact-ID cannot determine which upstream job produced the artifact being deployed.",
					Remediation:  "Ensure each build artifact has a distinct name or unique producer path.",
				})
				continue
			}

			var matchingProducer *ArtifactProducer
			if len(matchingProducers) == 1 {
				matchingProducer = matchingProducers[0]
			}

			// Check if producer uploaded with static name inside a matrix job
			if matchingProducer != nil && matchingProducer.Job.HasMatrix && !IsGitHubExpression(matchingProducer.ArtifactName) {
				findings = append(findings, &Finding{
					RuleID:       RuleArtifactIdentityUnverified,
					Status:       StatusUnverified,
					WorkflowFile: wf.Path,
					JobID:        deployJobID,
					Reason:       ReasonAmbiguousProducer,
					Message:      fmt.Sprintf("Matrix job %q produces artifact %q with static name across multiple matrix runs.", matchingProducer.JobID, matchingProducer.ArtifactName),
					Evidence:     fmt.Sprintf("static artifact name %q inside matrix job %q", matchingProducer.ArtifactName, matchingProducer.JobID),
					WhyItMatters: "Concurrent matrix executions overwrite or produce ambiguous artifacts when static names are used.",
					Remediation:  "Include matrix parameters in artifact name: name: artifact-${{ matrix.os }}.",
				})
				continue
			}

			// Find upstream validation consumers
			var matchingValidators []*ArtifactConsumer
			var mismatchValidators []*ArtifactConsumer
			isAmbiguous := false
			unverifiedReason := ""

			for _, valConsumer := range upstreamValidators {
				// Check if validator downloaded wildcard when multiple artifacts exist
				if valConsumer.DownloadsAll && len(distinctUpstreamNames) > 1 {
					isAmbiguous = true
					unverifiedReason = ReasonDownloadAllMultipleArtifacts
					break
				}

				matched, ambig := CompareArtifactNames(valConsumer.ArtifactName, depConsumer.ArtifactName, nil)
				if ambig {
					isAmbiguous = true
					unverifiedReason = ReasonDynamicArtifactName
				} else if matched {
					matchingValidators = append(matchingValidators, valConsumer)
				} else {
					mismatchValidators = append(mismatchValidators, valConsumer)
				}
			}

			// Check matrix parameter consistency between producer, validator, and deployer
			if matchingProducer != nil && matchingProducer.IsExpression {
				comp, amb := CompareMatrixContexts(matchingProducer.MatrixParams, depConsumer.MatrixParams, matchingProducer.ArtifactName, depConsumer.ArtifactName)
				if amb || !comp {
					isAmbiguous = true
					unverifiedReason = ReasonMatrixContextUnresolved
				}
			}

			// Check condition consistency
			if matchingProducer != nil {
				comp, amb := CompareConditions(matchingProducer.IfCondition, depConsumer.IfCondition)
				if amb || !comp {
					isAmbiguous = true
					unverifiedReason = ReasonConditionalPathUnresolved
				}
			}

			if isAmbiguous {
				if unverifiedReason == "" {
					unverifiedReason = ReasonDynamicArtifactName
				}
				stepIdx := depConsumer.StepIndex
				valName := "upstream expression"
				if len(upstreamValidators) > 0 {
					valName = upstreamValidators[0].ArtifactName
				}
				findings = append(findings, &Finding{
					RuleID:       RuleArtifactIdentityUnverified,
					Status:       StatusUnverified,
					WorkflowFile: wf.Path,
					JobID:        deployJobID,
					StepIndex:    &stepIdx,
					StepName:     depConsumer.StepName,
					Reason:       unverifiedReason,
					Message:      "Artifact identity cannot be statically verified.",
					Evidence:     fmt.Sprintf("validation artifact: %s\ndeployment artifact: %s\nreason: %s", valName, depConsumer.ArtifactName, unverifiedReason),
					WhyItMatters: "Dynamic expressions or matrix discrepancies prevent static proof of equivalence.",
					Remediation:  "Use deterministic matching names and compatible execution contexts across jobs.",
				})

				flows = append(flows, &ArtifactFlow{
					Workflow:     wf.Path,
					ArtifactName: depConsumer.ArtifactName,
					Producer:     matchingProducer,
					Deployers:    []*ArtifactConsumer{depConsumer},
					Status:       FlowUnverified,
					Reason:       unverifiedReason,
					Notes:        fmt.Sprintf("Unresolved identity: %s", unverifiedReason),
				})
			} else if len(mismatchValidators) > 0 && len(matchingValidators) == 0 {
				stepIdx := depConsumer.StepIndex
				mismatchVal := mismatchValidators[0]
				findings = append(findings, &Finding{
					RuleID:       RuleTestedAndDeployedArtifactMismatch,
					Status:       StatusFail,
					WorkflowFile: wf.Path,
					JobID:        deployJobID,
					StepIndex:    &stepIdx,
					StepName:     depConsumer.StepName,
					Message:      "Tested and deployed artifact identities mismatch.",
					Evidence:     fmt.Sprintf("validation job: %s (artifact %q)\ndeployment job: %s (artifact %q)", mismatchVal.JobID, mismatchVal.ArtifactName, deployJobID, depConsumer.ArtifactName),
					WhyItMatters: "The artifact verified during testing is not the artifact being deployed to production.",
					Remediation:  fmt.Sprintf("Ensure deployment job %q downloads artifact %q rather than %q.", deployJobID, mismatchVal.ArtifactName, depConsumer.ArtifactName),
				})

				flows = append(flows, &ArtifactFlow{
					Workflow:     wf.Path,
					ArtifactName: depConsumer.ArtifactName,
					Producer:     matchingProducer,
					Validators:   mismatchValidators,
					Deployers:    []*ArtifactConsumer{depConsumer},
					Status:       FlowMismatch,
					Notes:        fmt.Sprintf("Tested %q but deployed %q", mismatchVal.ArtifactName, depConsumer.ArtifactName),
				})
			} else {
				producerHasValidation := false
				if matchingProducer != nil {
					for _, v := range jobData[matchingProducer.JobID].ValidationEvents {
						if !v.IsSecondaryLint {
							producerHasValidation = true
							break
						}
					}
				}

				if len(matchingValidators) > 0 || (matchingProducer != nil && producerHasValidation) {
					stepIdx := depConsumer.StepIndex
					artName := depConsumer.ArtifactName
					if artName == "*" && matchingProducer != nil {
						artName = matchingProducer.ArtifactName
					}

					flow := &ArtifactFlow{
						Workflow:     wf.Path,
						ArtifactName: artName,
						Producer:     matchingProducer,
						Validators:   matchingValidators,
						Deployers:    []*ArtifactConsumer{depConsumer},
						Status:       FlowEstablished,
						Notes:        "Artifact chain verified from build through validation to deploy",
					}
					flows = append(flows, flow)

					prodJob := "upstream"
					if matchingProducer != nil {
						prodJob = matchingProducer.JobID
					}
					valJob := prodJob
					if len(matchingValidators) > 0 {
						valJob = matchingValidators[0].JobID
					}

					findings = append(findings, &Finding{
						RuleID:       RuleArtifactChainEstablished,
						Status:       StatusPass,
						WorkflowFile: wf.Path,
						JobID:        deployJobID,
						StepIndex:    &stepIdx,
						StepName:     depConsumer.StepName,
						Message:      "Artifact chain established.",
						Evidence:     fmt.Sprintf("producer: %s\nvalidator: %s\ndeployer: %s\nartifact: %s", prodJob, valJob, deployJobID, artName),
						WhyItMatters: "Statically confirms that the artifact validated during testing is the exact artifact deployed.",
					})
				} else {
					stepIdx := depConsumer.StepIndex
					prodJob := "upstream"
					if matchingProducer != nil {
						prodJob = matchingProducer.JobID
					}
					findings = append(findings, &Finding{
						RuleID:       RuleArtifactIdentityUnverified,
						Status:       StatusUnverified,
						WorkflowFile: wf.Path,
						JobID:        deployJobID,
						StepIndex:    &stepIdx,
						StepName:     depConsumer.StepName,
						Reason:       ReasonUnsupportedArtifactSemantics,
						Message:      "Deployed artifact was not validated in an upstream test job.",
						Evidence:     fmt.Sprintf("artifact %q produced by %q is deployed by %q without being validated by any intermediate job", depConsumer.ArtifactName, prodJob, deployJobID),
						WhyItMatters: "Artifact identity requires proof that the deployed artifact was validated prior to deployment.",
						Remediation:  "Add an upstream job that downloads and tests the artifact before deployment.",
					})

					flows = append(flows, &ArtifactFlow{
						Workflow:     wf.Path,
						ArtifactName: depConsumer.ArtifactName,
						Producer:     matchingProducer,
						Deployers:    []*ArtifactConsumer{depConsumer},
						Status:       FlowUnverified,
						Reason:       ReasonUnsupportedArtifactSemantics,
						Notes:        "Artifact deployed without upstream validation",
					})
				}
			}
		}
	}

	sortFindings(findings)

	sort.Slice(flows, func(i, j int) bool {
		if flows[i].Workflow != flows[j].Workflow {
			return flows[i].Workflow < flows[j].Workflow
		}
		return flows[i].ArtifactName < flows[j].ArtifactName
	})

	overallStatus := "PASS"
	hasFail := false
	hasWarn := false
	hasUnverified := false
	hasPass := false

	for _, f := range findings {
		switch f.Status {
		case StatusFail:
			hasFail = true
		case StatusWarning:
			hasWarn = true
		case StatusUnverified:
			hasUnverified = true
		case StatusPass:
			hasPass = true
		}
	}

	if hasFail {
		overallStatus = "FAIL"
	} else if hasWarn {
		overallStatus = "WARNING"
	} else if hasUnverified {
		overallStatus = "UNVERIFIED"
	} else if hasPass {
		overallStatus = "PASS"
	} else {
		overallStatus = "NOT_APPLICABLE"
	}

	report := &WorkflowReport{
		Path:          wf.Path,
		Name:          wf.Name,
		Status:        overallStatus,
		Jobs:          wf.JobOrder,
		ArtifactFlows: flows,
		Findings:      findings,
	}

	return report, nil
}

// AnalyzeWorkflows analyzes a collection of workflows and aggregates results.
func AnalyzeWorkflows(repositoryPath string, workflows []*workflow.Workflow) (*AnalysisResult, error) {
	result := &AnalysisResult{
		SchemaVersion: "1",
		ToolVersion:   "0.1.0",
		Repository:    repositoryPath,
		WorkflowCount: len(workflows),
		Workflows:     make([]*WorkflowReport, 0, len(workflows)),
		ArtifactFlows: make([]*ArtifactFlow, 0),
		Findings:      make([]*Finding, 0),
	}

	for _, wf := range workflows {
		report, err := AnalyzeWorkflow(wf)
		if err != nil {
			return nil, fmt.Errorf("error analyzing workflow %s: %w", wf.Path, err)
		}
		result.Workflows = append(result.Workflows, report)
		result.ArtifactFlows = append(result.ArtifactFlows, report.ArtifactFlows...)
		result.Findings = append(result.Findings, report.Findings...)
	}

	sort.Slice(result.Workflows, func(i, j int) bool {
		return result.Workflows[i].Path < result.Workflows[j].Path
	})

	sortFindings(result.Findings)
	sort.Slice(result.ArtifactFlows, func(i, j int) bool {
		if result.ArtifactFlows[i].Workflow != result.ArtifactFlows[j].Workflow {
			return result.ArtifactFlows[i].Workflow < result.ArtifactFlows[j].Workflow
		}
		return result.ArtifactFlows[i].ArtifactName < result.ArtifactFlows[j].ArtifactName
	})

	for _, f := range result.Findings {
		switch f.Status {
		case StatusFail:
			result.Summary.Failures++
		case StatusWarning:
			result.Summary.Warnings++
		case StatusUnverified:
			result.Summary.Unverified++
		}
	}
	for _, flow := range result.ArtifactFlows {
		if flow.Status == FlowEstablished {
			result.Summary.ArtifactChains++
		}
	}
	result.Summary.WorkflowsAnalyzed = len(workflows)

	return result, nil
}

func sortFindings(findings []*Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].WorkflowFile != findings[j].WorkflowFile {
			return findings[i].WorkflowFile < findings[j].WorkflowFile
		}
		if findings[i].JobID != findings[j].JobID {
			return findings[i].JobID < findings[j].JobID
		}
		if findings[i].RuleID != findings[j].RuleID {
			return findings[i].RuleID < findings[j].RuleID
		}
		stepI := -1
		if findings[i].StepIndex != nil {
			stepI = *findings[i].StepIndex
		}
		stepJ := -1
		if findings[j].StepIndex != nil {
			stepJ = *findings[j].StepIndex
		}
		if stepI != stepJ {
			return stepI < stepJ
		}
		return findings[i].Message < findings[j].Message
	})
}

// isDeploymentJobName checks if a job ID or name indicates a deployment, release, or publication step.
func isDeploymentJobName(jobID, jobName string) bool {
	idLower := strings.ToLower(jobID)
	nameLower := strings.ToLower(jobName)
	keywords := []string{"deploy", "release", "publish"}
	for _, kw := range keywords {
		if strings.Contains(idLower, kw) || strings.Contains(nameLower, kw) {
			return true
		}
	}
	return false
}

// isBenignAction checks if an action is a known standard runner, setup, or artifact utility.
// Actions that are local (./...) or unknown third-party actions in the critical path
// are considered opaque actions because static analysis cannot determine their internal behavior.
func isBenignAction(uses string) bool {
	u := strings.TrimSpace(uses)
	if u == "" {
		return true
	}
	// Local actions are always opaque
	if strings.HasPrefix(u, "./") || strings.HasPrefix(u, ".\\") {
		return false
	}

	uLower := strings.ToLower(u)
	if strings.HasPrefix(uLower, "actions/") || strings.HasPrefix(uLower, "github/") {
		return true
	}

	benignPrefixes := []string{
		"docker/setup-buildx-action",
		"docker/setup-qemu-action",
		"docker/login-action",
		"docker/metadata-action",
		"docker/build-push-action",
		"codecov/codecov-action",
		"github/codeql-action",
		"dtolnay/rust-toolchain",
		"astral-sh/setup-uv",
		"golangci/golangci-lint-action",
		"ruby/setup-ruby",
		// Known deployment actions
		"actions/deploy-pages",
		"jamesives/github-pages-deploy-action",
		"cloudflare/wrangler-action",
		"google-github-actions/",
		"aws-actions/",
		"azure/",
		"softprops/action-gh-release",
		"pypa/gh-action-pypi-publish",
	}

	for _, prefix := range benignPrefixes {
		if strings.HasPrefix(uLower, prefix) {
			return true
		}
	}
	return false
}
