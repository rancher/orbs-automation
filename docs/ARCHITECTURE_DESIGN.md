# Architectural Design Specification

## 1. System Architecture Overview

The central engineering tooling and automation ecosystem is designed as a hybrid GitOps control plane and CLI engine. It consolidates workflow rendering, multi-repo release orchestration, and developer tooling into a single Go-based binary while maintaining standard GitHub Actions (GHA) features.

```
+-----------------------------------------------------------------------------------+
|                            Central Tooling Repository                             |
|                                                                                   |
|   +-------------------+    +------------------------+    +--------------------+   |
|   |  main.go, cmd/,   |    |  workflows/ (planned)  |    |       config/      |   |
|   |  (Go CLI Binary,  |    |  (Reusable GHA Workf)  |    | (project configs)  |   |
|   |   internal/...)   |    |                        |    |                    |   |
|   +-------------------+    +------------------------+    +--------------------+   |
+------------------------------------------+----------------------------------------+
                                           |
                                           |
                  +------------------------+------------------------+
                  |                                                 |
                  | 1. CLI Pull / Sync                              | 2. Observational Loop
                  v                                                 v
+-----------------------------------+             +-----------------------------------+
|           Target Repo A           |             |           Target Repo B           |
|  - Rendered Stub Workflows        |             |  - RC Builds & Signings (GHA)     |
|  - Standard CI Workflows          |--(Triggers)-|  - Multi-repo Release Pipelines   |
+-----------------------------------+             +-----------------------------------+
```

---

## 2. Core Pillars & Design Mechanics

### A. Distribution & Execution Model
* **Binary:** `gero`, a standalone CLI. It does not require being a `gh` extension.
* **Developer Machines:** Distributed as precompiled binaries attached to GitHub Releases in the central repository. Installed locally or compiled via `go install`. Self-update logic is explicitly out of scope for v1.0.0.
* **GitHub Access:** Uses the Go API libraries `go-gh` (`pkg/auth` only: token and host resolution) and `go-github` (typed REST) directly. Authentication reuses `GH_TOKEN` / `GITHUB_TOKEN` or the local `gh` session where available. `gh` itself is only shelled out to for interactive or passthrough steps. See [GITHUB_ACCESS](specs/GITHUB_ACCESS.md).
* **Execution Contexts:** Commands are either CI-only (the reconciler and anything needing GHA-provided tokens, failing fast elsewhere) or work in either context.
* **CI Runners:** Workflows in CI fetch a pinned, tagged release version of the `gero` binary during initial setup steps. Running `go run` dynamically in CI is prohibited to avoid module download latency and compilation overhead.

### B. Workflow Rendering & Sync Engine
The system uses a **Hybrid Workflow Model** combining central reusable workflows with rendered stub workflows that live in the target repo:
* **Remote Reusable Workflows:** Standard, stateless CI tasks (linting, static analysis, generic container builds) live in the central repo and are invoked via `uses: rancher/orbs-automation/.github/workflows/<workflow>.yml@vX.Y.Z`, pinned to a CLI release tag or SHA, never a floating branch.
* **Rendered Stub Workflows:** For workflows that must be defined in the target repo itself (e.g., release, binary signing, and container image signing, which run on GitHub runners using keyless signing and need the workflow on the tagged branch), a per-repo config defines the subscriptions. The tool reads it from the repo's `automation-core` branch if one exists, otherwise from the default branch. Rendered stubs carry a generated header (template and CLI version, content hash) so drift can be detected.
* **CLI Sync (`gero workflows sync`):** Reads the target repo's config, pulls down canonical templates from the central repository, renders `.github/workflows/` YAML files, shows a dry-run diff, and opens a PR or commits changes. A `verify` command checks rendered files for drift.

### C. CLI Layer
The CLI is built on **cobra** with a small custom loader for CLI config (no viper). `cmd/` is the only place that imports cobra: it holds the command tree, flags and help text, and a short `RunE` per command. Each command's logic lives in its own framework-free `internal/cli/<cmd>/` package, which calls into domain packages under `internal/`, and for the actions that compose (create a branch, bump and commit, push, open a PR) into single-purpose task packages under `internal/task/`, which a command runs alone or chains. *CLI config* (how an invocation behaves) is kept separate from *application config* (the YAML in section 3):

```
 flags  >  GERO_* env  >  user file (~/.config/gero/config.yaml)  >  defaults
                              |
                        internal/platform/clicfg  ->  Settings  ->  command
```

Data goes to stdout and diagnostics to stderr, with `--json` on data commands and `log/slog` for logs. Output for a person is styled by `internal/ui/style` (lipgloss v2) and laid out by `ui.Align`, and `gero repl` reads its lines through `internal/ui/editor` (ergochat/readline). Each outside module sits behind one package, dependencies are stdlib-first, and the allow-list is in [DEPENDENCY_POLICY](specs/DEPENDENCY_POLICY.md). A full-screen TUI is not planned. All decisions are in the [specs](specs/README.md).

---

## 3. Topology & Config

The central automation repo is the source of truth for the project list, how projects relate (what each takes updates from), the Rancher release lines, and the configs of projects we do not own. Config is a set of YAML documents with `apiVersion`, `kind`, `name` and `spec`, inspired by Kubernetes resources but not compatible with them.

Kinds so far (names are not final):
* `Project`: a repo we model. Its `spec` has `repo`, `release` (branch policy, tag pattern, line-to-branch mapping), `updateMethods` (how it takes an update: a strategy configured by the consumer) and `downstream` (the projects it ships to, and with which of their methods). Edges are declared once, on the producing side, and the reverse view is derived. A project can be a producer on one edge and a consumer on another.
* `RancherLines`: the set of Rancher lines with their lifecycle dates (GA, EOM, EOL) and Kubernetes range. The phase is derived from the dates and the current UTC time. It has no branches; each project maps its own branches to line IDs.
* `External`: a third-party project we track (`trackOnly`, `react`) or notify.
* `WorkflowsConfig`: how workflows are rendered into a repo's branches. This lives in the target repo's `gero.yaml`, not centrally, and is the only kind allowed there for now.

Real configs and their README are in [`../config/`](../config/README.md); illustrative mock files are in [`../config-mock/`](../config-mock/) and [`../mock-repos/`](../mock-repos/). Shape, placement, defaults and validation are in [APP_CONFIG](specs/APP_CONFIG.md).

---

## 4. Observational Reconciler & Issue State Machine

Instead of a monolithic control plane that directly manages every build execution, the release engine operates as an **Observational Custom Controller**.

```
+--------------------------------+
| Target Repo Workflow Finishes  |
| (Builds, Signs, Cuts Tags/PRs) |
+---------------+----------------+
                |
                v
+--------------------------------+
|  GitHub API State Refreshed    |
| (Tags, PR Merges, Check Runs)  |
+---------------+----------------+
                |
                v
+--------------------------------+
|    Go Reconciler Runs Cron     |
|    (`gero ci:reconcile`)       |
+---------------+----------------+
                |
                v
+--------------------------------+
| Master Tracking Issue Updated  |
|  - State block updated         |
|  - Rendered view regenerated   |
|  - Phase/stale labels updated  |
+--------------------------------+
```

1. **Decentralized Execution:** Native target repository GHA workflows complete their own builds. Tags can be pushed by a human in the target repo, or created by a workflow run from the tracker; the reconciler observes either path. Downstream bump PRs are opened by shared `gero` logic, not per-repo scripts.
2. **Reconciler Observation:** A cron workflow in the central repo runs `gero ci:reconcile`. It inspects the actual state of GitHub via API calls (checking for tags, open/closed PRs, and workflow check runs). `repository_dispatch` from target repos can be added as a faster path where needed.
3. **Audit State in Issues:** Master release issues in the central repo are the persistent, human-auditable state. Machine state lives in a versioned hidden block in the issue body, and the visible markdown (read-only task lists, per-target status and a stale marker) is rendered from it. Humans do not edit these issues; interaction happens through commands, labels or workflow dispatch. The reconciler adds stage and stale labels and posts progress comments. Audience: engineers, QA and managers (and the future GUI).

---

## 5. QA Automation Handshake (v3 Boundary)

To interface cleanly with QA without coupling engineering builds directly to QA test runner logic. The design is **undecided** and is v3.0.0 scope (see `PHASES.md`); the current leaning is:
1. **Target Repo Event:** When an Engineering RC tag passes all build gates, the central tool dispatches a thin workflow in the **dedicated QA Automation Repository** with a correlation ID, milestone and Rancher version.
2. **QA Pipeline Execution:** The QA repo executes its automated test suites.
3. **Observed Result:** The reconciler observes the QA run's conclusion by correlation ID and records it on the tracking issue as evidence. The final stable promotion remains a human decision.

What a QA run proves it tested (Rancher version, chart version or image) must be agreed with the QA owners first. See the [QA handshake notes](IMPLEMENTATION.md#qa-handshake-v3-undecided) and [observability-e2e](projects/observability-e2e.md).

---

## 6. Moonshot: Local Web Panel Architecture

An optional `gero dashboard` or `gero triage` subcommand runs a lightweight embedded HTTP server (`net/http` with embedded static assets) on the developer's machine:
* Displays real-time status of multi-repo release trains by querying the central tracking issues.
* visualizes the project graph defined by the `downstream` edges in the central config.
* Inspired by Google's `triage-party`, allowing local custom views into multi-repo issue and release states.