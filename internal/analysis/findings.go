package analysis

// FindingStatus defines the status of an analysis finding.
type FindingStatus string

const (
	StatusPass       FindingStatus = "PASS"
	StatusWarning    FindingStatus = "WARNING"
	StatusFail       FindingStatus = "FAIL"
	StatusUnverified FindingStatus = "UNVERIFIED"
)

// Rule ID constants
const (
	RuleRebuildAfterValidation            = "AI001"
	RuleTestedAndDeployedArtifactMismatch = "AI002"
	RuleArtifactIdentityUnverified        = "AI003"
	RuleArtifactChainEstablished          = "AI004"
	RuleMutableImageReference             = "AI005"
)

// Standard UNVERIFIED reason codes for AI003
const (
	ReasonDynamicArtifactName          = "dynamic-artifact-name"
	ReasonAmbiguousProducer            = "ambiguous-producer"
	ReasonDownloadAllMultipleArtifacts = "download-all-multiple-artifacts"
	ReasonMatrixContextUnresolved      = "matrix-context-unresolved"
	ReasonConditionalPathUnresolved    = "conditional-path-unresolved"
	ReasonOpaqueReusableWorkflow       = "opaque-reusable-workflow"
	ReasonOpaqueAction                 = "opaque-action"
	ReasonUnsupportedArtifactSemantics = "unsupported-artifact-semantics"
	ReasonDynamicDigest                = "dynamic-digest"
	ReasonMultipleCandidateArtifacts   = "multiple-candidate-artifacts"
)

// FlowStatus represents the verified state of an artifact flow chain.
type FlowStatus string

const (
	FlowEstablished   FlowStatus = "ESTABLISHED"
	FlowMismatch      FlowStatus = "MISMATCH"
	FlowRebuild       FlowStatus = "REBUILD"
	FlowUnverified    FlowStatus = "UNVERIFIED"
	FlowNotApplicable FlowStatus = "NOT_APPLICABLE"
)

// Finding represents a single rule evaluation finding.
type Finding struct {
	RuleID       string        `json:"ruleId"`
	Status       FindingStatus `json:"status"`
	WorkflowFile string        `json:"workflowFile"`
	JobID        string        `json:"jobId,omitempty"`
	StepIndex    *int          `json:"stepIndex,omitempty"`
	StepName     string        `json:"stepName,omitempty"`
	Message      string        `json:"message"`
	Reason       string        `json:"reason,omitempty"`
	Evidence     string        `json:"evidence"`
	WhyItMatters string        `json:"whyItMatters"`
	Remediation  string        `json:"remediation,omitempty"`
}

// ArtifactFlow represents a traced chain of an artifact across jobs.
type ArtifactFlow struct {
	Workflow     string              `json:"workflow"`
	ArtifactName string              `json:"artifactName"`
	Producer     *ArtifactProducer   `json:"producer,omitempty"`
	Validators   []*ArtifactConsumer `json:"validators,omitempty"`
	Deployers    []*ArtifactConsumer `json:"deployers,omitempty"`
	Status       FlowStatus          `json:"status"`
	Reason       string              `json:"reason,omitempty"`
	Notes        string              `json:"notes,omitempty"`
}
