# Artifact-ID v0.1.0 External Validation Report

This report documents the external-reality validation pass for `artifact-id` v0.1.0 conducted against 15 mature public open-source GitHub repositories.

---

## 1. Methodology & Corpus

All repositories were shallow-cloned (`--depth 1`) from public GitHub remotes into an isolated validation environment. Workflows were analyzed purely statically using `artifact-id --format json`. Every reported `PASS`, `FAIL`, `WARNING`, `UNVERIFIED`, and `NOT_APPLICABLE` result was manually reviewed against the raw workflow YAML definitions to verify correctness.

### Target Repositories

| Repository | Remote Origin | Inspected Commit SHA | Primary Ecosystem | Workflows |
|---|---|---|---|---|
| **bat** | `https://github.com/sharkdp/bat.git` | `7323a7514f7601737640e7172be115127d6db08c` | Rust | 2 |
| **black** | `https://github.com/psf/black.git` | `20622e1259c29bda81831962ace1348ba1921c84` | Python | 13 |
| **buildx** | `https://github.com/docker/buildx.git` | `bf5ccd4f3dc37238fea64cca3262bfbee88e9ad7` | Docker / Go | 9 |
| **checkout** | `https://github.com/actions/checkout.git` | `f548e57e544e1ff5a4c46bf1e1b8685f8e4a348a` | Actions / Node | 5 |
| **cli** | `https://github.com/cli/cli.git` | `ba51bb47799e308109a980fca846a90d164f5811` | Go | 14 |
| **express** | `https://github.com/expressjs/express.git` | `53d4a0d606c0388f764f192b306ce0e90200e7e8` | Node | 4 |
| **fastapi** | `https://github.com/fastapi/fastapi.git` | `50113da16fec53b66b80d75e80a89296de4fa5a5` | Python | 20 |
| **gin** | `https://github.com/gin-gonic/gin.git` | `dcaa4296d111981ffb31ac3eba90bb63e1eb5ab9` | Go | 4 |
| **httpx** | `https://github.com/encode/httpx.git` | `b5addb64f0161ff6bfe94c124ef76f6a1fba5254` | Python | 2 |
| **hugo** | `https://github.com/gohugoio/hugo.git` | `881c0ec70d746d3a86b29c9104aa56ac51040ee9` | Go | 3 |
| **ora** | `https://github.com/sindresorhus/ora.git` | `86403dcc176ad26a94f3005774f8c7fbebad2714` | Node | 1 |
| **ripgrep** | `https://github.com/BurntSushi/ripgrep.git` | `3fce3b5bb0236da2df6d99672afb8a719642eca7` | Rust | 2 |
| **serde** | `https://github.com/serde-rs/serde.git` | `a874a1b1bb1cc16cf5ee3b1b7b527af5705742bb` | Rust | 1 |
| **upload-artifact** | `https://github.com/actions/upload-artifact.git` | `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` | Actions / Node | 7 |
| **zap** | `https://github.com/uber-go/zap.git` | `bb1a55dd13257cf7cbd06b4146674c67ca614dea` | Go | 1 |

Total Workflows Analyzed: **88**

---

## 2. Summary of Findings

Across the 88 analyzed workflows, Artifact-ID identified the following distribution:

| Status | Count | Notes |
|---|---|---|
| **UNVERIFIED** | 5 workflows | Ambiguities, opaque action boundaries, pattern-based downloads, or external reusable workflows detected. |
| **NOT_APPLICABLE** | 83 workflows | Continuous integration test suites, linters, issue triage, or tag-only release scripts without cross-job artifact handoffs. |
| **PASS** | 0 workflows (OSS repos) | None of the 15 open-source repositories implemented an unbroken static artifact transfer chain without external opaque actions or unresolvable patterns. |
| **FAIL** | 0 workflows | No direct static mismatches (`AI002`) or unproven rebuild-after-validation (`AI001`) with explicit upload/download pairs were present. |

---

## 3. Detailed Per-Repository Audit & Manual Inspection

### 1. sharkdp/bat (Rust)
- **Inspected Commit**: `7323a7514f7601737640e7172be115127d6db08c`
- **Workflows**:
  - `.github/workflows/CICD.yml`: `UNVERIFIED`
    - Finding: `AI003 UNVERIFIED` (`reason: opaque-action`) in job `winget`.
    - Evidence: Step 0 executes `uses: vedantmgoyal9/winget-releaser@19e706d4c9121098010096f9c495a70a7518b30f`.
    - Manual Verification: The `winget` job is a release step that delegates release package publication to a third-party custom action. Because the action implementation is opaque to static analysis, `artifact-id` conservative design correctly marks this unverified.
  - `.github/workflows/require-changelog-for-PRs.yml`: `NOT_APPLICABLE` (PR check).
- **Verdict**: Correct.

### 2. psf/black (Python)
- **Inspected Commit**: `20622e1259c29bda81831962ace1348ba1921c84`
- **Workflows**:
  - `.github/workflows/pypi_upload.yml`: `UNVERIFIED`
    - Finding: `AI003 UNVERIFIED` (`reason: unsupported-artifact-semantics`) in job `publish-mypyc`.
    - Evidence: `actions/download-artifact` with `pattern: "*-mypyc-wheels"` and `merge-multiple: true`.
    - Manual Verification: Line 98 of `pypi_upload.yml` downloads wheels using wildcard pattern matching. Dynamic pattern matching downloads an arbitrary collection of files that cannot be statically matched to upstream producers. `artifact-id` correctly refuses to pass this unverified flow.
  - 12 other workflows (`test.yml`, `lint.yml`, `diff_shades.yml`, etc.): `NOT_APPLICABLE` (CI test/lint only).
- **Verdict**: Correct.

### 3. docker/buildx (Docker / Go)
- **Inspected Commit**: `bf5ccd4f3dc37238fea64cca3262bfbee88e9ad7`
- **Workflows**:
  - `.github/workflows/build.yml`: `UNVERIFIED`
    - Finding 1: `AI003 UNVERIFIED` (`reason: opaque-reusable-workflow`) in job `release`.
      - Evidence: Upstream job `binaries` calls external reusable workflow `docker/github-builder/.github/workflows/bake.yml@58cb9f5b71b1836d6f690c1e95effdeb9b98cb8a`.
    - Finding 2: `AI003 UNVERIFIED` (`reason: dynamic-artifact-name`) in job `release`.
      - Evidence: Validation container reference contains dynamic expression `@${{ env.GOTESTSUM_VERSION }}`.
    - Manual Verification: The `buildx` release pipeline invokes an external repository reusable workflow (`docker/github-builder/...`) to build release binaries. Static analysis cannot inspect cross-repository external workflows without external network access. Correctly flagged as `opaque-reusable-workflow`.
  - 8 other workflows (`e2e.yml`, `validate.yml`, `docs-release.yml`, etc.): `NOT_APPLICABLE`.
- **Verdict**: Correct.

### 4. actions/checkout (GitHub Actions / Node)
- **Inspected Commit**: `f548e57e544e1ff5a4c46bf1e1b8685f8e4a348a`
- **Workflows**: 5 workflows (`check-dist.yml`, `licenses.yml`, `lint.yml`, `test.yml`, `test-proxy.yml`).
- **Tool Result**: `NOT_APPLICABLE` for all workflows.
- **Manual Verification**: `actions/checkout` is an action repository. Its workflows only run test suites, linting, and proxy integration tests. There are no deployment commands or deployment actions.
- **Verdict**: Correct.

### 5. cli/cli (Go)
- **Inspected Commit**: `ba51bb47799e308109a980fca846a90d164f5811`
- **Workflows**:
  - `.github/workflows/deployment.yml`: `UNVERIFIED`
    - Finding 1: `AI003 UNVERIFIED` (`reason: download-all-multiple-artifacts`) in job `release`.
      - Evidence: Step `Merge built artifacts` calls `actions/download-artifact` with no `name:`, but upstream jobs produce 3 distinct platform artifacts (`linux`, `macos`, `windows`).
    - Finding 2: `AI003 UNVERIFIED` (`reason: opaque-action`) in jobs `linux`, `macos`, `windows`.
      - Evidence: Execution of custom actions `goreleaser/goreleaser-action`, `nodeselector/setup-apple-codesign`, and `microsoft/setup-msbuild`.
    - Manual Verification: The `release` job runs `script/label-assets dist/gh_* | xargs gh release create ...`. The command pipeline splitting on `|` properly detects the `gh release create` command. However, the download step downloads all artifacts without naming individual packages, creating an ambiguous identity mapping.
  - 13 other workflows (`go.yml`, `lint.yml`, `acceptance.yml`, triage workflows): `NOT_APPLICABLE`.
- **Verdict**: Correct.

### 6. expressjs/express (Node)
- **Inspected Commit**: `53d4a0d606c0388f764f192b306ce0e90200e7e8`
- **Workflows**: 4 workflows (`ci.yml`, `legacy.yml`, `codeql.yml`, `scorecard.yml`).
- **Tool Result**: `NOT_APPLICABLE` for all workflows.
- **Manual Verification**: Express executes testing and coverage reporting across Node.js versions. Deployments to npm are not automated in these GitHub Actions workflows.
- **Verdict**: Correct.

### 7. fastapi/fastapi (Python)
- **Inspected Commit**: `50113da16fec53b66b80d75e80a89296de4fa5a5`
- **Workflows**:
  - `.github/workflows/deploy-docs.yml`: `UNVERIFIED`
    - Finding: `AI003 UNVERIFIED` (`reason: unsupported-artifact-semantics`) in job `deploy-docs`.
    - Evidence: Step downloads with `pattern: "docs-site-*"` and `merge-multiple: true`.
    - Manual Verification: Line 42 of `deploy-docs.yml` downloads multiple documentation translation sites using pattern matching and merges them before deployment to Cloudflare/S3. Correctly recognized as unsupported non-deterministic artifact semantics.
  - 19 other workflows (`test.yml`, `publish.yml`, etc.): `NOT_APPLICABLE`.
- **Verdict**: Correct.

### 8. gin-gonic/gin (Go)
- **Inspected Commit**: `dcaa4296d111981ffb31ac3eba90bb63e1eb5ab9`
- **Workflows**: 4 workflows (`gin.yml`, `goreleaser.yml`, `codeql.yml`, `trivy-scan.yml`).
- **Tool Result**: `NOT_APPLICABLE` for all workflows.
- **Manual Verification**: `goreleaser.yml` executes `goreleaser --snapshot --skip=publish --clean`, which is explicitly a non-publishing snapshot check. There is no multi-job deployment chain.
- **Verdict**: Correct.

### 9. encode/httpx (Python)
- **Inspected Commit**: `b5addb64f0161ff6bfe94c124ef76f6a1fba5254`
- **Workflows**: 2 workflows (`publish.yml`, `test-suite.yml`).
- **Tool Result**: `NOT_APPLICABLE` for all workflows.
- **Manual Verification**: `publish.yml` runs only on published releases and builds/uploads directly to PyPI from git checkout within a single step, without an upstream CI artifact validation chain.
- **Verdict**: Correct.

### 10. gohugoio/hugo (Go)
- **Inspected Commit**: `881c0ec70d746d3a86b29c9104aa56ac51040ee9`
- **Workflows**: 3 workflows (`test.yml`, `image.yml`, `stale.yml`).
- **Tool Result**: `NOT_APPLICABLE` for all workflows.
- **Manual Verification**: Hugo tests against matrices of Go and Ruby versions. Container builds in `image.yml` only trigger on push to master without deployment stages.
- **Verdict**: Correct.

### 11. sindresorhus/ora (Node)
- **Inspected Commit**: `86403dcc176ad26a94f3005774f8c7fbebad2714`
- **Workflows**: 1 workflow (`main.yml`).
- **Tool Result**: `NOT_APPLICABLE`.
- **Manual Verification**: Single-job Node.js unit test matrix with no deployments.
- **Verdict**: Correct.

### 12. BurntSushi/ripgrep (Rust)
- **Inspected Commit**: `3fce3b5bb0236da2df6d99672afb8a719642eca7`
- **Workflows**:
  - `.github/workflows/ci.yml`: `NOT_APPLICABLE` (unit/integration test suite).
  - `.github/workflows/release.yml`: `NOT_APPLICABLE`.
    - Manual Verification: `release.yml` triggers on tag push (`[0-9]+.[0-9]+.[0-9]+`). Job `create-release` initializes a draft GitHub release with `gh release create`. Then `build-release` builds binary deb/tarballs locally and immediately executes `gh release upload`. There is no upstream validation job that tests a binary before handing it off to `build-release`. The binary is built and uploaded in the same job.
- **Verdict**: Correct.

### 13. serde-rs/serde (Rust)
- **Inspected Commit**: `a874a1b1bb1cc16cf5ee3b1b7b527af5705742bb`
- **Workflows**: 1 workflow (`ci.yml`).
- **Tool Result**: `NOT_APPLICABLE`.
- **Manual Verification**: Pure CI matrix testing across Rust stable, beta, and nightly.
- **Verdict**: Correct.

### 14. actions/upload-artifact (GitHub Actions / Node)
- **Inspected Commit**: `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a`
- **Workflows**: 7 workflows (`test.yml`, `test-proxy.yml`, `check-dist.yml`, `licensed.yml`, `codeql-analysis.yml`, `publish-immutable-actions.yml`, `release-new-action-version.yml`).
- **Tool Result**: `NOT_APPLICABLE` for all workflows.
- **Manual Verification**: `release-new-action-version.yml` updates major git tags using `actions/publish-action`. No build artifacts are deployed.
- **Verdict**: Correct.

### 15. uber-go/zap (Go)
- **Inspected Commit**: `bb1a55dd13257cf7cbd06b4146674c67ca614dea`
- **Workflows**: 1 workflow (`go.yml`).
- **Tool Result**: `NOT_APPLICABLE`.
- **Manual Verification**: Unit testing with race detector and code coverage upload to Codecov. No deployments.
- **Verdict**: Correct.

---

## 4. Key Takeaways & Hardening Validations

1. **Conservative Boundary Enforcement (`ReasonOpaqueAction`)**:
   In `sharkdp/bat` (`vedantmgoyal9/winget-releaser`) and `cli/cli` (`goreleaser/goreleaser-action`), deployment jobs execute third-party custom actions. Artifact-ID correctly refuses to grant speculative passes to opaque actions.

2. **Cross-Repository Reusable Workflows (`ReasonOpaqueReusableWorkflow`)**:
   In `docker/buildx`, release binaries are built inside `docker/github-builder/.github/workflows/bake.yml`. Without fetching external repository code, static analysis cannot inspect the build-to-upload chain. Emitting `opaque-reusable-workflow` accurately reports this boundary.

3. **Pattern Matching & Merge Multiple (`ReasonUnsupportedArtifactSemantics`)**:
   In `psf/black` and `fastapi/fastapi`, `actions/download-artifact` steps use dynamic wildcard patterns (`*-mypyc-wheels` and `docs-site-*`). Artifact-ID strictly marks pattern downloads as unverified because wildcard matching allows arbitrary untracked artifacts into the deployment workspace.

4. **Wildcard Downloads with Multiple Upstream Candidates (`ReasonDownloadAllMultipleArtifacts`)**:
   In `cli/cli`, `actions/download-artifact` without a name downloads all artifacts in a workflow where 3 separate platform jobs produce distinct binaries. Artifact-ID flags this ambiguity.

5. **JSON Array Hardening**:
   All workflows with zero findings or zero flows serialize `"findings": []` and `"artifactFlows": []` rather than `null`.

---

## 5. Real-World Positive Controls (AI004 PASS)

To definitively prove that Artifact-ID accepts legitimate CI/CD artifact identity chains without false positives, five mature public open-source GitHub repositories were identified, cloned into an isolated validation environment, and analyzed. Every workflow was manually audited line-by-line against its raw YAML definition.

### Summary Table

| Repository | Remote Origin | Commit SHA | Ecosystem | Workflow Path | Artifact | Producer | Validator | Deployer | Result |
|---|---|---|---|---|---|---|---|---|---|
| **twbs/blog** | `https://github.com/twbs/blog.git` | `0c6dae9d229aed915586dd35b7c5da2132d1b098` | GitHub Pages / Web | `.github/workflows/ci.yml` | `github-pages` | `test` | `test` | `deploy` | **AI004 PASS** |
| **dswah/pyGAM** | `https://github.com/dswah/pyGAM.git` | `b6a48ab5d1ef7df07ce2a34bbe572bca75e83e7d` | Python / PyPI | `.github/workflows/pypi.yml` | `wheels` | `build_wheels` | `test_wheels` | `deploy` | **AI004 PASS** |
| **davatron5000/microlighter** | `https://github.com/davatron5000/microlighter.git` | `0e6956e0a9e9628652af12d2debf5f9bb5c5b614` | GitHub Pages / Node | `.github/workflows/ci.yml` | `github-pages` | `test` | `test` | `deploy` | **AI004 PASS** |
| **luanti-org/luanti-org.github.io** | `https://github.com/luanti-org/luanti-org.github.io.git` | `b82bcfd57a469a3e5511e6c349fb1b06fb795687` | GitHub Pages / Web | `.github/workflows/build.yml` | `github-pages` | `build` | `build` | `deploy` | **AI004 PASS** |
| **usman2x/awesome-pakistani** | `https://github.com/usman2x/awesome-pakistani.git` | `045daa8b327480ffef666028e4dd48ad06daa0f2` | GitHub Pages / Node | `.github/workflows/pages.yml` | `github-pages` | `build` | `build` | `deploy` | **AI004 PASS** |

---

### Positive Control 1: twbs/blog (GitHub Pages)

- **Repository**: `twbs/blog`
- **Origin URL**: `https://github.com/twbs/blog.git`
- **Inspected Commit SHA**: `0c6dae9d229aed915586dd35b7c5da2132d1b098`
- **Workflow File**: `.github/workflows/ci.yml`
- **Artifact Name**: `github-pages`
- **Producer Job**: `test`
- **Validator Job**: `test`
- **Deployer Job**: `deploy`

#### CLI Execution & Output
```bash
$ artifact-id --format json /path/to/twbs_blog
```
```json
{
  "schemaVersion": "1",
  "toolVersion": "0.1.0",
  "workflowCount": 3,
  "workflows": [
    {
      "path": ".github/workflows/ci.yml",
      "name": "CI",
      "status": "PASS",
      "jobs": ["test", "deploy"],
      "artifactFlows": [
        {
          "workflow": ".github/workflows/ci.yml",
          "artifactName": "github-pages",
          "producer": {
            "workflowFile": ".github/workflows/ci.yml",
            "jobId": "test",
            "stepIndex": 5,
            "stepName": "Upload docs",
            "artifactName": "github-pages",
            "kind": "actions/upload-pages-artifact"
          },
          "deployers": [
            {
              "workflowFile": ".github/workflows/ci.yml",
              "jobId": "deploy",
              "stepIndex": 0,
              "stepName": "Deploy to GitHub Pages",
              "artifactName": "github-pages",
              "kind": "actions/deploy-pages"
            }
          ],
          "status": "ESTABLISHED",
          "notes": "Artifact chain verified from build through validation to deploy"
        }
      ],
      "findings": [
        {
          "ruleId": "AI004",
          "status": "PASS",
          "workflowFile": ".github/workflows/ci.yml",
          "jobId": "deploy",
          "stepIndex": 0,
          "stepName": "Deploy to GitHub Pages",
          "message": "Artifact chain established.",
          "evidence": "producer: test\nvalidator: test\ndeployer: deploy\nartifact: github-pages"
        }
      ]
    }
  ],
  "summary": {
    "workflowsAnalyzed": 3,
    "artifactChains": 1,
    "failures": 0,
    "warnings": 0,
    "unverified": 0
  }
}
```

#### Line-by-Line YAML Audit
- **Validation**: Step 4 runs `npm test` within job `test`, successfully establishing primary validation.
- **Upload**: Step 5 uses `actions/upload-pages-artifact@fc324d3547104276b827a68afc52ff2a11cc49c9` (v5.0.0) with condition `if: github.repository == 'twbs/blog' && github.ref == 'refs/heads/main'`. This produces artifact `github-pages`.
- **Deploy**: Job `deploy` declares `needs: test` and identical condition `if: github.repository == 'twbs/blog' && github.ref == 'refs/heads/main'`. Step 0 executes `actions/deploy-pages@cd2ce8fcbc39b97be8ca5fce6e763baed58fa128` (v5.0.0), consuming artifact `github-pages`.
- **Integrity**: No rebuild steps exist in `deploy`. No opaque actions or unresolvable dynamic expressions exist in the critical path.
- **Verdict**: Unambiguous, fully verified `AI004 PASS`.

---

### Positive Control 2: dswah/pyGAM (Python / PyPI 3-Job DAG)

- **Repository**: `dswah/pyGAM`
- **Origin URL**: `https://github.com/dswah/pyGAM.git`
- **Inspected Commit SHA**: `b6a48ab5d1ef7df07ce2a34bbe572bca75e83e7d`
- **Workflow File**: `.github/workflows/pypi.yml`
- **Artifact Name**: `wheels`
- **Producer Job**: `build_wheels`
- **Validator Job**: `test_wheels`
- **Deployer Job**: `deploy`

#### CLI Execution & Output
```bash
$ artifact-id --format json /path/to/dswah_pygam
```
```json
{
  "schemaVersion": "1",
  "toolVersion": "0.1.0",
  "workflowCount": 1,
  "workflows": [
    {
      "path": ".github/workflows/pypi.yml",
      "name": "Publish to PyPI",
      "status": "PASS",
      "jobs": ["build_wheels", "test_wheels", "deploy"],
      "artifactFlows": [
        {
          "workflow": ".github/workflows/pypi.yml",
          "artifactName": "wheels",
          "producer": {
            "workflowFile": ".github/workflows/pypi.yml",
            "jobId": "build_wheels",
            "stepIndex": 3,
            "stepName": "Store wheels",
            "artifactName": "wheels",
            "kind": "actions/upload-artifact"
          },
          "validators": [
            {
              "workflowFile": ".github/workflows/pypi.yml",
              "jobId": "test_wheels",
              "stepIndex": 3,
              "artifactName": "wheels",
              "destination": "wheelhouse",
              "kind": "actions/download-artifact"
            }
          ],
          "deployers": [
            {
              "workflowFile": ".github/workflows/pypi.yml",
              "jobId": "deploy",
              "stepIndex": 0,
              "artifactName": "wheels",
              "destination": "wheelhouse",
              "kind": "actions/download-artifact"
            }
          ],
          "status": "ESTABLISHED",
          "notes": "Artifact chain verified from build through validation to deploy"
        }
      ],
      "findings": [
        {
          "ruleId": "AI004",
          "status": "PASS",
          "workflowFile": ".github/workflows/pypi.yml",
          "jobId": "deploy",
          "stepIndex": 0,
          "message": "Artifact chain established.",
          "evidence": "producer: build_wheels\nvalidator: test_wheels\ndeployer: deploy\nartifact: wheels"
        }
      ]
    }
  ],
  "summary": {
    "workflowsAnalyzed": 1,
    "artifactChains": 1,
    "failures": 0,
    "warnings": 0,
    "unverified": 0
  }
}
```

#### Line-by-Line YAML Audit
- **Producer**: Job `build_wheels` executes `python -m build --wheel --sdist --outdir wheelhouse` and uploads via `actions/upload-artifact@v4` with explicit name `wheels`.
- **Validation**: Job `test_wheels` declares `needs: build_wheels`, downloads `name: wheels` via `actions/download-artifact@v7`, installs the downloaded wheel (`uv pip install "${{ env.WHEELNAME }}[dev]"`), and executes `python -m pytest`. This proves the actual built artifact is tested.
- **Deployment**: Job `deploy` declares `needs: test_wheels`, downloads exact `name: wheels` via `actions/download-artifact@v4`, and publishes to PyPI using official `pypa/gh-action-pypi-publish@release/v1`.
- **Integrity**: Full 3-job diamond/pipeline DAG. No rebuild commands exist in `deploy`. Artifact identity `wheels` is completely preserved across all three jobs.
- **Verdict**: Unambiguous, fully verified `AI004 PASS`.

---

### Positive Control 3: davatron5000/microlighter (GitHub Pages / Node)

- **Repository**: `davatron5000/microlighter`
- **Origin URL**: `https://github.com/davatron5000/microlighter.git`
- **Inspected Commit SHA**: `0e6956e0a9e9628652af12d2debf5f9bb5c5b614`
- **Workflow File**: `.github/workflows/ci.yml`
- **Artifact Name**: `github-pages`
- **Producer Job**: `test`
- **Validator Job**: `test`
- **Deployer Job**: `deploy`

#### CLI Execution & Output
```bash
$ artifact-id --format json /path/to/davatron5000_microlighter
```
```json
{
  "schemaVersion": "1",
  "toolVersion": "0.1.0",
  "workflowCount": 1,
  "workflows": [
    {
      "path": ".github/workflows/ci.yml",
      "name": "CI & Pages",
      "status": "PASS",
      "jobs": ["test", "deploy"],
      "findings": [
        {
          "ruleId": "AI004",
          "status": "PASS",
          "workflowFile": ".github/workflows/ci.yml",
          "jobId": "deploy",
          "stepIndex": 0,
          "stepName": "Deploy to GitHub Pages",
          "message": "Artifact chain established.",
          "evidence": "producer: test\nvalidator: test\ndeployer: deploy\nartifact: github-pages"
        }
      ]
    }
  ],
  "summary": {
    "workflowsAnalyzed": 1,
    "artifactChains": 1,
    "failures": 0,
    "warnings": 0,
    "unverified": 0
  }
}
```

#### Line-by-Line YAML Audit
- **Validation**: Step 4 runs `npm test` ("Build, test, and check size budget") within job `test`.
- **Upload**: Step 6 executes `actions/upload-pages-artifact@v3` with `path: docs` and condition `if: github.event_name == 'push'`.
- **Deploy**: Job `deploy` has `needs: test` and matching condition `if: github.event_name == 'push'`. Step 0 executes `actions/deploy-pages@v4`.
- **Integrity**: No rebuild in deploy. Official runner actions used throughout.
- **Verdict**: Unambiguous, fully verified `AI004 PASS`.

---

### Positive Control 4: luanti-org/luanti-org.github.io (GitHub Pages)

- **Repository**: `luanti-org/luanti-org.github.io`
- **Origin URL**: `https://github.com/luanti-org/luanti-org.github.io.git`
- **Inspected Commit SHA**: `b82bcfd57a469a3e5511e6c349fb1b06fb795687`
- **Workflow File**: `.github/workflows/build.yml`
- **Artifact Name**: `github-pages`
- **Producer Job**: `build`
- **Validator Job**: `build`
- **Deployer Job**: `deploy`

#### CLI Execution & Output
```bash
$ artifact-id --format json /path/to/luanti_org
```
```json
{
  "schemaVersion": "1",
  "toolVersion": "0.1.0",
  "workflowCount": 1,
  "workflows": [
    {
      "path": ".github/workflows/build.yml",
      "name": "build",
      "status": "PASS",
      "jobs": ["build", "deploy"],
      "findings": [
        {
          "ruleId": "AI004",
          "status": "PASS",
          "workflowFile": ".github/workflows/build.yml",
          "jobId": "deploy",
          "stepIndex": 0,
          "stepName": "Deploy to GitHub Pages",
          "message": "Artifact chain established.",
          "evidence": "producer: build\nvalidator: build\ndeployer: deploy\nartifact: github-pages"
        }
      ]
    }
  ],
  "summary": {
    "workflowsAnalyzed": 1,
    "artifactChains": 1,
    "failures": 0,
    "warnings": 0,
    "unverified": 0
  }
}
```

#### Line-by-Line YAML Audit
- **Validation**: Step 5 executes `npm test` ("Test site") after `npm run build` in job `build`.
- **Upload**: Step 6 executes `actions/upload-pages-artifact@v3` with `path: _site`.
- **Deploy**: Job `deploy` declares `needs: build` and runs `actions/deploy-pages@v4`.
- **Integrity**: Unconditional deterministic handoff between producer and deployer. No rebuild in deployer.
- **Verdict**: Unambiguous, fully verified `AI004 PASS`.

---

### Positive Control 5: usman2x/awesome-pakistani (GitHub Pages / Node)

- **Repository**: `usman2x/awesome-pakistani`
- **Origin URL**: `https://github.com/usman2x/awesome-pakistani.git`
- **Inspected Commit SHA**: `045daa8b327480ffef666028e4dd48ad06daa0f2`
- **Workflow File**: `.github/workflows/pages.yml`
- **Artifact Name**: `github-pages`
- **Producer Job**: `build`
- **Validator Job**: `build`
- **Deployer Job**: `deploy`

#### CLI Execution & Output
```bash
$ artifact-id --format json /path/to/usman2x_pakistani
```
```json
{
  "schemaVersion": "1",
  "toolVersion": "0.1.0",
  "workflowCount": 1,
  "workflows": [
    {
      "path": ".github/workflows/pages.yml",
      "name": "Publish GitHub Pages",
      "status": "PASS",
      "jobs": ["build", "deploy"],
      "findings": [
        {
          "ruleId": "AI004",
          "status": "PASS",
          "workflowFile": ".github/workflows/pages.yml",
          "jobId": "deploy",
          "stepIndex": 0,
          "stepName": "Deploy to GitHub Pages",
          "message": "Artifact chain established.",
          "evidence": "producer: build\nvalidator: build\ndeployer: deploy\nartifact: github-pages"
        }
      ]
    }
  ],
  "summary": {
    "workflowsAnalyzed": 1,
    "artifactChains": 1,
    "failures": 0,
    "warnings": 0,
    "unverified": 0
  }
}
```

#### Line-by-Line YAML Audit
- **Validation**: Step 3 executes `npm run build && npm test` in job `build`.
- **Upload**: Step 5 executes `actions/upload-pages-artifact@v4` with `path: docs`.
- **Deploy**: Job `deploy` declares `needs: build` and runs `actions/deploy-pages@v4`.
- **Integrity**: Deterministic handoff, zero opaque actions, zero rebuild commands.
- **Verdict**: Unambiguous, fully verified `AI004 PASS`.

---

### Additional Real-World Negative & Unverified Cases Uncovered

During the search for positive controls, several public repositories exposed genuine failure or unverified patterns:

1. **FirefoxCSS-Store/FirefoxCSS-Store.github.io** (`AI001 FAIL`):
   - `.github/workflows/build.yml` executes `npm test` (Step 3), followed by `npm run build` (Step 4), followed by `actions/deploy-pages@v5` (Step 6) in the same job. Because the build step occurs after validation, the deployed bytes are rebuilt and unverified prior to deployment.
2. **executablebooks/MyST-Parser** (`AI001 FAIL`):
   - Publish jobs run `flit build` after upstream test jobs have completed, resulting in unverified rebuild after validation.
3. **ciembor/4bit** (`AI003 UNVERIFIED`):
   - Producer uploads on `github.event_name == 'push'`, while deployer runs on `github.event_name == 'push' && github.ref == 'refs/heads/master'`. Artifact-ID correctly flags `conditional-path-unresolved`.
4. **omnilib/aiosqlite** (`AI003 UNVERIFIED`):
   - Job `test` tests git checkout in parallel. Job `build` runs `python -m build` and uploads `sdist`. Job `publish` downloads `sdist` and deploys to PyPI without intermediate validation. Artifact-ID correctly flags `unsupported-artifact-semantics` (unvalidated artifact).

---

## 6. Minimum Go Version Toolchain Verification

`go.mod` specifies `go 1.22.0`. To ensure that Artifact-ID compiles, passes all unit tests, passes the race detector, and passes `go vet` on the claimed minimum toolchain, official Go 1.22 was installed in an isolated scratch environment and executed directly.

### Verified Toolchain Version
```bash
$ /path/to/go1.22/bin/go version
go version go1.22.12 linux/amd64
```

### 1. `go 1.22 test -count=1 ./...`
```
ok  	ci-artifact-identity/cmd/artifact-id	0.020s
ok  	ci-artifact-identity/internal/analysis	0.067s
ok  	ci-artifact-identity/internal/report	0.003s
ok  	ci-artifact-identity/internal/workflow	0.005s
```

### 2. `go 1.22 test -count=1 -race ./...`
```
ok  	ci-artifact-identity/cmd/artifact-id	1.138s
ok  	ci-artifact-identity/internal/analysis	1.668s
ok  	ci-artifact-identity/internal/report	1.018s
ok  	ci-artifact-identity/internal/workflow	1.016s
```

### 3. `go 1.22 vet ./...`
```
(Clean - 0 warnings / 0 errors)
```

### Minimum Go Version Consistency Check
- `go.mod`: `go 1.22.0`
- `README.md`: `Requirements: Go 1.22+`
- `CONTRIBUTING.md`: `- Go 1.22 or higher`
- `.github/workflows/ci.yml`: `go-version: ['1.22', '1.27']`

All files consistently declare Go 1.22 compatibility.

---

## 7. Current System Toolchain Quality Gates (Go 1.27.1)

- `go test -count=1 ./...`: **PASS** (all packages pass)
- `go test -count=1 -race ./...`: **PASS** (zero data races)
- `go vet ./...`: **PASS** (zero vet issues)
- `test -z "$(gofmt -l .)"`: **PASS** (`GOFMT_CLEAN`)
- Privacy scan (developer machine paths, personal usernames, and emails in source and binary): **CLEAN** (0 occurrences)

