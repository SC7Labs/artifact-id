# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-09-11

### Added
- Static CI artifact identity analyzer for GitHub Actions workflows.
- Core rules:
  - `AI001`: `REBUILD_AFTER_VALIDATION` (FAIL) — detects downstream rebuilding after validation.
  - `AI002`: `TESTED_AND_DEPLOYED_ARTIFACT_MISMATCH` (FAIL) — detects mismatched artifact names or container digests between validation and deployment.
  - `AI003`: `ARTIFACT_IDENTITY_UNVERIFIED` (UNVERIFIED) — detects ambiguous artifact flows, dynamic expressions, matrix parameter discrepancies, continue-on-error validation, and unresolvable reusable workflows with fine-grained reason codes.
  - `AI004`: `ARTIFACT_CHAIN_ESTABLISHED` (PASS) — verifies unbroken artifact flows from build to validation to deployment.
  - `AI005`: `MUTABLE_IMAGE_REFERENCE_AFTER_DIGEST_VALIDATION` (WARNING) — warns when mutable container tags are deployed after immutable digest validation.
- Graph-based workflow DAG resolution with cycle detection, topological sorting, and deduplication.
- YAML parser supporting YAML 1.1 merge keys (`<<: *anchor`), alias dereferencing, and bounded file/job/step limits.
- Command tokenization supporting multi-line commands, chaining (`&&`, `||`, `;`), environment prefixes, and subshells, distinguishing auxiliary tool/documentation builds from application rebuilds.
- First-class support for `actions/upload-pages-artifact` and `actions/deploy-pages`.
- Deterministic JSON report output (`schemaVersion: "1"`) and human-readable terminal output.
- Suite of 10 representative synthetic workflow topologies for regression testing.
- Opaque action detection (`ReasonOpaqueAction`) for local or external custom actions in critical validation and deployment paths.
- Pipeline delimiter splitting (`|`) and GitHub CLI release command (`gh release create/upload`) classification.
- Docker dynamic digest detection (`ReasonDynamicDigest`) and expanded mutable tag coverage.
- JSON serialization hardening ensuring non-nil array representations (`[]` rather than `null`).
- External validation against 15 mature public open-source repositories.

