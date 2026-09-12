package analysis

import (
	"regexp"
	"strings"

	"ci-artifact-identity/internal/workflow"
)

// BuildEvent represents a recognized build execution within a job.
type BuildEvent struct {
	JobID       string `json:"jobId"`
	StepIndex   int    `json:"stepIndex"`
	StepName    string `json:"stepName,omitempty"`
	Command     string `json:"command"`
	BuildFamily string `json:"buildFamily"`
	IsAuxiliary bool   `json:"isAuxiliary"` // True if building an auxiliary tool/uploader/docs rather than app
	Position    int    `json:"position"`
}

// ValidationEvent represents a recognized test or validation execution within a job.
type ValidationEvent struct {
	JobID            string `json:"jobId"`
	StepIndex        int    `json:"stepIndex"`
	StepName         string `json:"stepName,omitempty"`
	Command          string `json:"command"`
	ValidationFamily string `json:"validationFamily"`
	IsSecondaryLint  bool   `json:"isSecondaryLint"`
	Position         int    `json:"position"`
}

// DeploymentEvent represents a recognized deployment command or action within a job.
type DeploymentEvent struct {
	JobID            string `json:"jobId"`
	StepIndex        int    `json:"stepIndex"`
	StepName         string `json:"stepName,omitempty"`
	Command          string `json:"command,omitempty"`
	Action           string `json:"action,omitempty"`
	DeploymentFamily string `json:"deploymentFamily"`
	Position         int    `json:"position"`
}

var (
	// Env prefix pattern: FOO=bar, NODE_ENV=production, etc.
	reEnvPrefix = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*=\S*\s+`)

	// Build command patterns (anchored to command start)
	reGoBuild     = regexp.MustCompile(`^go\s+build(\s|$)`)
	reCargoBuild  = regexp.MustCompile(`^cargo\s+build(\s|$)`)
	reNpmBuild    = regexp.MustCompile(`^npm\s+run\s+(build|compile|bundle|dist)(\S*)(\s|$)`)
	rePnpmBuild   = regexp.MustCompile(`^pnpm(\s+run)?\s+(build|compile)(\S*)(\s|$)`)
	reYarnBuild   = regexp.MustCompile(`^yarn(\s+run)?\s+(build|compile|bundle)(\S*)(\s|$)`)
	reBunBuild    = regexp.MustCompile(`^bun(\s+run)?\s+(build|compile)(\S*)(\s|$)`)
	reDotnetBuild = regexp.MustCompile(`^dotnet\s+(build|publish)(\s|$)`)
	reMvnBuild    = regexp.MustCompile(`^mvn\s+.*(package|compile)(\s|$)`)
	reGradleBuild = regexp.MustCompile(`^(\./)?gradlew?\s+.*(build|assemble|bootJar|shadowJar)(\s|$)`)
	rePythonBuild = regexp.MustCompile(`^(python[0-9.]*|py)\s+-m\s+build(\s|$)`)
	rePoetryBuild = regexp.MustCompile(`^(poetry|flit|hatch)\s+build(\s|$)`)
	reDockerBuild = regexp.MustCompile(`^(docker(\s+buildx)?|podman)\s+build(\s|$)`)

	// Auxiliary build patterns (building docs or separate tools/uploader)
	reAuxBuild = regexp.MustCompile(`(build-docs|build:docs|docs|tools/|cmd/uploader|uploader)`)

	// Validation command patterns
	reNpmTest    = regexp.MustCompile(`^npm(\s+run)?\s+(test|t)(\s|$)`)
	rePnpmTest   = regexp.MustCompile(`^pnpm(\s+run)?\s+(test|t)(\s|$)`)
	reYarnTest   = regexp.MustCompile(`^yarn(\s+run)?\s+test(\s|$)`)
	reBunTest    = regexp.MustCompile(`^bun(\s+run)?\s+test(\s|$)`)
	reGoTest     = regexp.MustCompile(`^go\s+test(\s|$)`)
	reCargoTest  = regexp.MustCompile(`^cargo\s+test(\s|$)`)
	rePytest     = regexp.MustCompile(`^(pytest|(python[0-9.]*|py)\s+-m\s+pytest)(\s|$)`)
	reDotnetTest = regexp.MustCompile(`^dotnet\s+test(\s|$)`)
	reMvnTest    = regexp.MustCompile(`^mvn\s+.*(test|verify)(\s|$)`)
	reGradleTest = regexp.MustCompile(`^(\./)?gradlew?\s+.*(test|check)(\s|$)`)

	// Secondary lint patterns
	reLint = regexp.MustCompile(`^(npm\s+run\s+lint|eslint|golangci-lint|flake8|ruff|cargo\s+(clippy|check))(\s|$)`)

	// Deployment command patterns (strictly matched against actual deployment invocations)
	reNpmDeploy    = regexp.MustCompile(`^npm\s+run\s+deploy(-prod|-production|:prod|:production)?(\s|$)`)
	rePnpmDeploy   = regexp.MustCompile(`^pnpm(\s+run)?\s+deploy(-prod|-production|:prod|:production)?(\s|$)`)
	reYarnDeploy   = regexp.MustCompile(`^yarn(\s+run)?\s+deploy(-prod|-production|:prod|:production)?(\s|$)`)
	reBunDeploy    = regexp.MustCompile(`^bun(\s+run)?\s+deploy(-prod|-production|:prod|:production)?(\s|$)`)
	reFirebase     = regexp.MustCompile(`^firebase\s+deploy(\s|$)`)
	reVercel       = regexp.MustCompile(`^vercel(\s+--prod|\s+deploy)?(\s|$)`)
	reNetlify      = regexp.MustCompile(`^netlify\s+deploy(\s|$)`)
	reKubectl      = regexp.MustCompile(`^kubectl\s+(apply|rollout|set)(\s|$)`)
	reHelm         = regexp.MustCompile(`^helm\s+(upgrade|install)(\s|$)`)
	reAwsDeploy    = regexp.MustCompile(`^aws\s+(deploy|s3\s+sync|ecs\s+update-service|cloudformation\s+deploy)(\s|$)`)
	reGcloudDeploy = regexp.MustCompile(`^gcloud\s+(app\s+deploy|run\s+deploy|compute\s+instances\s+update)(\s|$)`)
	reDockerPush   = regexp.MustCompile(`^(docker|podman)\s+push(\s|$)`)
	rePublishPkg   = regexp.MustCompile(`^(npm\s+publish|cargo\s+publish|twine\s+upload|poetry\s+publish)(\s|$)`)
	reGhRelease    = regexp.MustCompile(`^(xargs\s+)?gh\s+release\s+(create|upload)(\s|$)`)
)

// ExtractCommands splits a script into individual executable command tokens,
// stripping comments, echo/printf invocations, subshell wrappers, and environment prefixes.
func ExtractCommands(script string) []string {
	var cmds []string
	lines := strings.Split(script, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Strip inline comments if outside quotes
		if commentIdx := strings.Index(trimmed, " #"); commentIdx != -1 {
			trimmed = strings.TrimSpace(trimmed[:commentIdx])
		}

		// Split on command delimiters: &&, ||, ;, |
		// Basic shell tokenization: replace delimiters with newline
		replacer := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n")
		subTokens := strings.Split(replacer.Replace(trimmed), "\n")

		for _, sub := range subTokens {
			cmd := strings.TrimSpace(sub)
			if cmd == "" {
				continue
			}

			// Strip subshell parentheses: (cd foo && make)
			cmd = strings.TrimPrefix(cmd, "(")
			cmd = strings.TrimSuffix(cmd, ")")
			cmd = strings.TrimSpace(cmd)

			// Strip leading environment variable assignments
			for reEnvPrefix.MatchString(cmd) {
				cmd = strings.TrimSpace(reEnvPrefix.ReplaceAllString(cmd, ""))
			}

			// Ignore echo and printf commands
			if strings.HasPrefix(cmd, "echo ") || strings.HasPrefix(cmd, "printf ") || cmd == "echo" || cmd == "printf" {
				continue
			}

			cmds = append(cmds, cmd)
		}
	}

	return cmds
}

// ClassifyBuildStep checks if a step executes a recognized build command.
func ClassifyBuildStep(jobID string, step *workflow.Step) []BuildEvent {
	var events []BuildEvent
	if step.Run == "" {
		return events
	}

	commands := ExtractCommands(step.Run)
	for lineIdx, cmd := range commands {
		family := ""
		if reGoBuild.MatchString(cmd) {
			family = "go"
		} else if reCargoBuild.MatchString(cmd) {
			family = "cargo"
		} else if reNpmBuild.MatchString(cmd) {
			family = "npm"
		} else if rePnpmBuild.MatchString(cmd) {
			family = "pnpm"
		} else if reYarnBuild.MatchString(cmd) {
			family = "yarn"
		} else if reBunBuild.MatchString(cmd) {
			family = "bun"
		} else if reDotnetBuild.MatchString(cmd) {
			family = "dotnet"
		} else if reMvnBuild.MatchString(cmd) {
			family = "maven"
		} else if reGradleBuild.MatchString(cmd) {
			family = "gradle"
		} else if rePythonBuild.MatchString(cmd) || rePoetryBuild.MatchString(cmd) {
			family = "python"
		} else if reDockerBuild.MatchString(cmd) {
			family = "docker"
		}

		if family != "" {
			isAux := reAuxBuild.MatchString(cmd)
			events = append(events, BuildEvent{
				JobID:       jobID,
				StepIndex:   step.Index,
				StepName:    step.Name,
				Command:     cmd,
				BuildFamily: family,
				IsAuxiliary: isAux,
				Position:    lineIdx + 1,
			})
		}
	}

	return events
}

// ClassifyValidationStep checks if a step executes a recognized validation/test command.
func ClassifyValidationStep(jobID string, step *workflow.Step) []ValidationEvent {
	var events []ValidationEvent
	if step.Run == "" {
		return events
	}

	commands := ExtractCommands(step.Run)
	for lineIdx, cmd := range commands {
		family := ""
		isSecondary := false

		if reGoTest.MatchString(cmd) {
			family = "go"
		} else if reCargoTest.MatchString(cmd) {
			family = "cargo"
		} else if reNpmTest.MatchString(cmd) {
			family = "npm"
		} else if rePnpmTest.MatchString(cmd) {
			family = "pnpm"
		} else if reYarnTest.MatchString(cmd) {
			family = "yarn"
		} else if reBunTest.MatchString(cmd) {
			family = "bun"
		} else if rePytest.MatchString(cmd) {
			family = "pytest"
		} else if reDotnetTest.MatchString(cmd) {
			family = "dotnet"
		} else if reMvnTest.MatchString(cmd) {
			family = "maven"
		} else if reGradleTest.MatchString(cmd) {
			family = "gradle"
		} else if reLint.MatchString(cmd) {
			family = "lint"
			isSecondary = true
		}

		if family != "" {
			events = append(events, ValidationEvent{
				JobID:            jobID,
				StepIndex:        step.Index,
				StepName:         step.Name,
				Command:          cmd,
				ValidationFamily: family,
				IsSecondaryLint:  isSecondary,
				Position:         lineIdx + 1,
			})
		}
	}

	return events
}

// ClassifyDeploymentStep checks if a step executes a recognized deployment command or action.
func ClassifyDeploymentStep(jobID string, step *workflow.Step) []DeploymentEvent {
	var events []DeploymentEvent

	// 1. Check uses: actions
	if step.Uses != "" {
		usesLower := strings.ToLower(step.Uses)
		family := ""
		if strings.HasPrefix(usesLower, "actions/deploy-pages") {
			family = "github-pages"
		} else if strings.HasPrefix(usesLower, "jamesives/github-pages-deploy-action") {
			family = "github-pages"
		} else if strings.HasPrefix(usesLower, "cloudflare/wrangler-action") {
			family = "cloudflare"
		} else if strings.HasPrefix(usesLower, "google-github-actions/deploy-") {
			family = "gcloud"
		} else if strings.HasPrefix(usesLower, "aws-actions/amazon-ecs-deploy") {
			family = "aws"
		} else if strings.HasPrefix(usesLower, "azure/webapps-deploy") {
			family = "azure"
		} else if strings.HasPrefix(usesLower, "softprops/action-gh-release") {
			family = "gh-release"
		} else if strings.HasPrefix(usesLower, "pypa/gh-action-pypi-publish") {
			family = "pypi"
		} else if strings.HasPrefix(usesLower, "docker/build-push-action") {
			if pushVal, ok := step.With["push"]; ok {
				pushStr := strings.TrimSpace(strings.ToLower(getStringValue(pushVal)))
				if pushStr == "true" {
					family = "docker-push"
				}
			}
		}

		if family != "" {
			events = append(events, DeploymentEvent{
				JobID:            jobID,
				StepIndex:        step.Index,
				StepName:         step.Name,
				Action:           step.Uses,
				DeploymentFamily: family,
				Position:         1,
			})
		}
	}

	// 2. Check run: commands
	if step.Run != "" {
		commands := ExtractCommands(step.Run)
		for lineIdx, cmd := range commands {
			family := ""
			if reNpmDeploy.MatchString(cmd) {
				family = "npm"
			} else if rePnpmDeploy.MatchString(cmd) {
				family = "pnpm"
			} else if reYarnDeploy.MatchString(cmd) {
				family = "yarn"
			} else if reBunDeploy.MatchString(cmd) {
				family = "bun"
			} else if reFirebase.MatchString(cmd) {
				family = "firebase"
			} else if reVercel.MatchString(cmd) {
				family = "vercel"
			} else if reNetlify.MatchString(cmd) {
				family = "netlify"
			} else if reKubectl.MatchString(cmd) {
				family = "kubectl"
			} else if reHelm.MatchString(cmd) {
				family = "helm"
			} else if reAwsDeploy.MatchString(cmd) {
				family = "aws"
			} else if reGcloudDeploy.MatchString(cmd) {
				family = "gcloud"
			} else if reDockerPush.MatchString(cmd) {
				family = "docker"
			} else if rePublishPkg.MatchString(cmd) {
				family = "package-publish"
			} else if reGhRelease.MatchString(cmd) {
				family = "gh-release"
			}

			if family != "" {
				events = append(events, DeploymentEvent{
					JobID:            jobID,
					StepIndex:        step.Index,
					StepName:         step.Name,
					Command:          cmd,
					DeploymentFamily: family,
					Position:         lineIdx + 1,
				})
			}
		}
	}

	return events
}

func getStringValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
