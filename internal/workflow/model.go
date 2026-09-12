package workflow

// Resource bounds for static analysis
const (
	MaxWorkflowFileSize      = 5 * 1024 * 1024 // 5 MB
	MaxWorkflowCount         = 500
	MaxJobsPerWorkflow       = 200
	MaxStepsPerJob           = 500
	MaxReusableWorkflowDepth = 5
)

// Workflow represents a parsed GitHub Actions workflow file.
type Workflow struct {
	Path     string          `json:"path"`
	Name     string          `json:"name"`
	Jobs     map[string]*Job `json:"jobs"`
	JobOrder []string        `json:"jobOrder"` // Deterministic topologically sorted job IDs
}

// Job represents a single job in a workflow.
type Job struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name,omitempty"`
	Needs           []string               `json:"needs,omitempty"`
	Steps           []*Step                `json:"steps"`
	Outputs         map[string]string      `json:"outputs,omitempty"`
	HasMatrix       bool                   `json:"hasMatrix"`
	MatrixParams    map[string][]string    `json:"matrixParams,omitempty"`
	MatrixInclude   []map[string]string    `json:"matrixInclude,omitempty"`
	MatrixExclude   []map[string]string    `json:"matrixExclude,omitempty"`
	RunsOn          string                 `json:"runsOn,omitempty"`
	Environment     string                 `json:"environment,omitempty"`
	If              string                 `json:"if,omitempty"`
	ContinueOnError bool                   `json:"continueOnError,omitempty"`
	Uses            string                 `json:"uses,omitempty"` // Reusable workflow call
	With            map[string]interface{} `json:"with,omitempty"`
}

// Step represents an individual step in a job.
type Step struct {
	Index           int                    `json:"index"`
	ID              string                 `json:"id,omitempty"`
	Name            string                 `json:"name,omitempty"`
	Uses            string                 `json:"uses,omitempty"`
	ActionOwner     string                 `json:"actionOwner,omitempty"`
	ActionName      string                 `json:"actionName,omitempty"`
	ActionRef       string                 `json:"actionRef,omitempty"`
	Run             string                 `json:"run,omitempty"`
	With            map[string]interface{} `json:"with,omitempty"`
	Env             map[string]string      `json:"env,omitempty"`
	If              string                 `json:"if,omitempty"`
	ContinueOnError bool                   `json:"continueOnError,omitempty"`
}

// JobDependency represents a directed dependency edge from one job to another.
type JobDependency struct {
	FromJobID string `json:"fromJobId"`
	ToJobID   string `json:"toJobId"`
}
