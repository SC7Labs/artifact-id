# ci-artifact-identity (`artifact-id`)

Static CI Artifact Identity Analyzer for GitHub Actions workflows.

Answers the question:
> **"Did the CI workflow establish that the artifact being deployed is the same artifact that was validated/tested?"**

---

## Why This Project Exists

Modern CI pipelines can be completely green while the deployed artifact was rebuilt after testing:

```
[Traditional CI: Fragile Identity]
source
  ↓
build
  ↓
test
  ↓
rebuild
  ↓
deploy UNKNOWN/DIFFERENT bytes
```

In this common pattern, unit tests and integration tests ran against one build, but the deployment job triggered a separate rebuild (`npm run build`, `docker build`, etc.). The deployed bytes were never directly validated, leaving room for non-deterministic builds, dependency drift, cache pollution, or timing differences to introduce untested bugs into production.

A stronger pipeline establishes an **immutable artifact chain**:

```
[Established Artifact Chain]
source
  ↓
build artifact
  ↓
test THAT artifact
  ↓
deploy THAT SAME artifact
```

`artifact-id` statically inspects GitHub Actions workflows (`.github/workflows/*.yml`, `.yaml`) to determine whether your workflow architecture establishes this artifact identity chain or re-compiles downstream.

---

## What Artifact Identity Means Here

Artifact identity means establishing a traceable, unbroken dataflow link from the job that produces a build artifact, through the job that tests or validates that artifact, to the job that deploys it:

1. **Job Dependency (`needs:`)**: Specifies execution order, but **does not** imply artifact reuse. A deployment job that `needs: test` may still recompile code from scratch.
2. **Artifact Movement (`upload-artifact` / `download-artifact`)**: Defines the physical transfer of bytes between isolated CI runner environments.
3. **Identity Verification**: Verifies that the deployed artifact matches the tested artifact name (or immutable content digest) with no intervening rebuild step.

---

## Core Rules

| Rule ID | Name | Severity | Description |
|---|---|---|---|
| **AI001** | `REBUILD_AFTER_VALIDATION` | `FAIL` | A deployment path rebuilds code after upstream validation without evidence that the already-validated artifact is reused. |
| **AI002** | `TESTED_AND_DEPLOYED_ARTIFACT_MISMATCH` | `FAIL` | Upstream validation tested artifact `A`, but downstream deployment downloaded and deployed artifact `B`. |
| **AI003** | `ARTIFACT_IDENTITY_UNVERIFIED` | `UNVERIFIED` | Dynamic GitHub Actions expressions prevent proving equivalence between validated and deployed artifacts. |
| **AI004** | `ARTIFACT_CHAIN_ESTABLISHED` | `PASS` | An artifact is uploaded by a build job, downloaded and tested by a validation job, and downloaded and deployed without rebuilding. |
| **AI005** | `MUTABLE_IMAGE_REFERENCE_AFTER_DIGEST_VALIDATION` | `WARNING` | An immutable container image digest was validated upstream, but deployment references only a mutable tag (`:latest`). |

---

## Dependencies & Design Principles

- **Minimal Dependencies**: Relies exclusively on the Go standard library and `gopkg.in/yaml.v3` for robust YAML 1.1/1.2 parsing and anchor resolution.
- **Zero Runtime Network Calls**: Operates completely offline with zero external API calls to GitHub, package registries, or remote services.
- **Zero Code Execution**: Analyzes workflow configurations purely statically without executing runner commands, repository scripts, or containers.
- **Deterministic Output**: Produces identical, sorted output across repeated runs, filesystems, and operating systems.
- **False-Pass-Resistant by Design**: Prioritizes conservative static analysis over speculative passes. Ambiguous expressions, opaque actions, or reusable workflow boundaries are reported as `UNVERIFIED` (`AI003`) with machine-readable reason codes.

---

## Installation & CLI Usage

### Build from Source

Requirements: Go 1.22+

```bash
go build -o artifact-id ./cmd/artifact-id
```

### Basic Commands

Analyze workflows in the current repository:
```bash
artifact-id .
```

Analyze a specific repository path:
```bash
artifact-id /path/to/repository
```

Analyze a single workflow file:
```bash
artifact-id --workflow .github/workflows/release.yml .
```

Output machine-readable JSON:
```bash
artifact-id --format json .
```

Check version:
```bash
artifact-id --version
# Artifact-ID 0.1.0
```

---

## Exit Codes

`artifact-id` provides explicit, deterministic exit codes suitable for CI integration:

- **`0`**: No demonstrated artifact-identity problem was found. All analyzed flows were either established (`AI004`) or not applicable (e.g. workflows without deployment).
- **`1`**: At least one warning (`WARNING`), failure (`FAIL`), or unverified (`UNVERIFIED`) condition exists.
- **`2`**: Invocation or input failure:
  - Repository directory does not exist.
  - Workflow YAML is malformed or cannot be parsed.
  - Invalid command-line arguments.
  - Requested workflow file does not exist.

---

## Supported Patterns & Heuristics

`artifact-id` implements conservative, static pattern detection:

- **Artifact Primitives**:
  - `actions/upload-artifact` (v2, v3, v4): Tracks artifact name, output path, producer job, and step index.
  - `actions/download-artifact` (v2, v3, v4): Tracks artifact name, download destination, consumer job, and wildcard downloads.
- **Docker Digest Flows**:
  - `docker/build-push-action`: Captures immutable image digest references (`steps.<id>.outputs.digest`).
- **Build Classifiers**:
  - Node/JS: `npm run build`, `pnpm build`, `yarn build`, `bun run build`
  - Go: `go build`
  - Rust: `cargo build`, `cargo build --release`
  - .NET: `dotnet build`, `dotnet publish`
  - Java: `mvn package`, `mvn verify`, `gradle build`, `./gradlew build`
  - Python: `python -m build`, `poetry build`, `flit build`, `hatch build`
  - Container: `docker build`, `podman build`
- **Validation Classifiers**:
  - `npm test`, `pnpm test`, `yarn test`, `bun test`
  - `go test`
  - `cargo test`
  - `pytest`, `python -m pytest`
  - `dotnet test`
  - `mvn test`, `mvn verify`
  - `gradle test`, `./gradlew test`
- **Deployment Classifiers**:
  - Commands: `npm run deploy`, `pnpm deploy`, `yarn deploy`, `firebase deploy`, `vercel deploy`, `netlify deploy`, `kubectl apply`, `helm upgrade`, `aws ... deploy`, `gcloud ... deploy`, `docker push`
  - Actions: `actions/deploy-pages`, `cloudflare/wrangler-action`, `google-github-actions/deploy-*`, `aws-actions/amazon-ecs-deploy-*`, `azure/webapps-deploy`, `softprops/action-gh-release`, `pypa/gh-action-pypi-publish`

---

## Examples

### 1. Established Chain (Good)

```yaml
name: Release
on:
  push:
    tags: ['v*']
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: npm run build
      - uses: actions/upload-artifact@v4
        with:
          name: web-dist
          path: dist/
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: web-dist
      - run: npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: web-dist
      - run: npm run deploy
```

**Output:**
```
Artifact-ID 0.1.0
Repository: .

Workflow: release.yml

Artifact flow
  build
    └─ web-dist
        ↓
  test
    └─ web-dist
        ↓
  deploy
    └─ web-dist

✓ Artifact chain established

Summary
  workflows analyzed: 1
  artifact chains: 1
  failures: 0
  warnings: 0
  unverified: 0
```

### 2. Rebuild After Validation (AI001 FAIL)

```yaml
jobs:
  build:
    steps:
      - run: npm run build
  test:
    needs: build
    steps:
      - run: npm test
  deploy:
    needs: test
    steps:
      - run: npm run build
      - run: npm run deploy
```

**Output:**
```
AI001  FAIL
deploy / Build production

Deployment rebuilds after validation.

Evidence:
  validation job: test
  downstream build: npm run build
  deployment job: deploy

Why it matters:
  Deployment rebuilds after validation, so the tool cannot establish that the deployed bytes are the bytes that passed validation.

Remediation:
  Download and deploy the pre-built artifact validated in upstream jobs instead of rebuilding.

Artifact identity: NOT ESTABLISHED
```

---

## Non-Goals & Boundaries

`artifact-id` is intentionally focused:
- **Not** a general YAML syntax linter.
- **Not** a permission or secret scanner.
- **Not** an SBOM generator or SLSA attestation engine.
- **Not** a workflow runner or GitHub API client.
- **Not** an arbitrary shell script execution engine.

---

## Explicit Limitations

> **This tool performs static workflow analysis.**
> It does not execute CI jobs and cannot prove runtime behavior that is not represented in workflow configuration.

> **A green result means the analyzed artifact flow was structurally consistent according to the supported patterns.**
>
> It is **NOT**:
> - a security certification
> - proof the artifact itself is correct
> - proof GitHub Actions actually completed successfully
> - proof external deployment systems behaved correctly

### Additional Technical Limitations

- **Matrix Workflows**: Dynamic matrix expressions (e.g. `artifact-${{ matrix.os }}`) are tracked symbolically. Identical symbolic references match; unresolvable or mismatched combinations are reported as `UNVERIFIED` (`AI003`).
- **Dynamic Scripts**: Arbitrary shell scripts that construct artifact names dynamically or fetch artifacts via custom curl/API calls cannot be resolved statically and remain `UNVERIFIED`.
- **Non-GitHub Actions CI**: v0.1 only targets GitHub Actions workflows.

---

## License

MIT License — Copyright (c) 2026 SC7Labs Contributors.
