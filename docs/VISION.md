# Engineering Automation & Tooling Vision

## Overview
This repository serves as the central command center for engineering tooling, developer workflows, and release orchestration across our software ecosystem. The goal is to move away from flimsy, sprawling shell scripts toward a robust, composable Go-based CLI and observational GitOps control plane built on top of native GitHub primitives.

## Core Pillars

### 1. Unified Go CLI (`gh` Companion)
* **Philosophy:** Retain the modular, "unix-style" composability of our past scripts, but implement them cleanly in Go.
* **Integration:** A standalone CLI (`gero`) that works well beside the GitHub CLI (`gh`), reusing existing authentication (`GH_TOKEN` or the local `gh` session) and command idioms (cobra noun-verb commands, `--json` output, `gh`-style flags), without requiring `gh` to run. See [CLI_FOUNDATIONS](CLI_FOUNDATIONS.md).
* **Scope:** Handles repo-local workflow generation, GHA-only release operations (such as binary and image signing, which always run on GitHub runners using keyless signing), and developer productivity tasks.
* **Execution Contexts:** Commands are either CI-only (require GHA-provided tokens and fail fast outside GHA) or work in either context (developer workstation or CI).

### 2. Hybrid Workflow Architecture
* **Remote Reusable Workflows:** Stateless, standard CI pipelines (linting, testing, container builds) live in this central repo and are referenced directly by target repositories.
* **Rendered Stub Workflows:** For tasks that must be defined inside the target repo itself (e.g., release, binary signing, and image signing workflows, which run on GitHub runners with keyless signing), a CLI sync command renders stub workflows and repo-local configuration into target repos.

### 3. Observational Reconciler & GitHub Issue State Machines
* **Human-Auditable State:** Releases are tracked via structured GitHub Issues in the automation repo. Machine state is stored in the issue, and a readable view (read-only markdown task lists and labels, including a stale marker) is rendered from it for engineers, QA and managers.
* **Observational Cron:** A scheduled GHA workflow running a Go reconciler checks the actual state of GitHub (tags, PR merges, check runs) and updates the master tracking issues accordingly, avoiding brittle micro-management while maintaining full auditability.

### 4. Cross-Boundary QA Integration (v3)
* **Ecosystem Bridging:** Once core engineering release trains are stable, the central control plane connects with the dedicated QA automation repository, recording automated test results as evidence alongside manual sign-off for stable promotions. The exact handshake is undecided.

### 5. Moonshot: Local Web Dashboards
* **Local Insight:** A lightweight, embedded Go web server mode to provide real-time visual dashboards for project topology, dependency trees, and active release trains.