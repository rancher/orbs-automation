# Phases and Milestones

How we turn [VISION](VISION.md) into reality, using the design in [ARCHITECTURE_DESIGN](ARCHITECTURE_DESIGN.md) and the decisions in [IMPLEMENTATION](IMPLEMENTATION.md). The open work, in pieces someone can pick up, is the [TODO](TODO.md). **Draft: milestones and order are proposals.**

Milestones are major versions of `gero`, and each has an **audience** and a **message**. We tag v1.0.0 only when we are confident of the foundation under it, not on a date. There are no dates here by design.

## Principles

* **Each milestone stands on its own** and is useful even if later ones slip.
* **Safest and most reviewable first.** Read-only and PR-based work comes before anything that writes without a human reviewing it.
* **Smallest real slice, then widen.** Start with one real flow end to end.
* **Open questions are answered by the code that needs them.**
* **Workstation first, then CI.** A command runs on an engineer's machine, as them and with their own credentials, before it runs in a workflow as the bot, over the same backends ([EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)).
* **Run alongside, then switch.** A new `gero` path runs next to the script it replaces until we trust it, and the script is then removed from the repo that held it.
* **Replace tooling that lives in other repos.** The measure of progress is a script that no longer has to be copied, fixed and kept in step in each repository.
* **Shareable work.** Where pieces do not depend on each other they can be built in parallel by different people.

## The plan at a glance

| Tag | Name | Audience | What we say | Gate |
|---|---|---|---|---|
| **v0.x** | Foundation and the first flows | The team | Nothing outside it | The foundation under v1.0.0 is built |
| **v1.0.0** | A tool the team can rely on | The team, and early users | "`gero` ships bump PRs from a workstation and from CI, in place of scripts in our repos, and traces our release chain" | The gate below |
| **v1.x** | Tracking and standardization | The wider organization, to a limited degree | Progress updates | Releases are tracked and more of the work is shared |
| **v2.0.0** | Complete | The wider organization and leadership | "Most of the release work we did by hand is now automated" | A real release cycle run through `gero` |
| **v3.0.0** | QA bridge | Engineering and QA leadership | "Engineering and QA are in sync through shared tooling" | QA results appear on release tracking |
| Moonshot | Dashboard | Managers and leadership | "Anyone can see the state of releases" | Whenever it is ready; unversioned |

From v1.0.0 only stable tags of `gero` are released. Config documents use `apiVersion: gero/v1alpha2` until v1.0.0, so the team can agree on the shapes before they become `gero/v1`.

---

## v0.x: foundation and the first flows

What the team can use today, by theme. The status of any part is in its spec, and `gero --help` is the list of commands.

* **The tool.** The CLI skeleton and its layers, GitHub access, settings from flags, environment and a user file, `gero doctor`, `gero config`, the REPL and styled output, and tagged, signed binary releases with `gero doctor self-update`.
* **The config model.** `gero/v1alpha2` documents of small kinds, the engine that decodes, defaults and validates them, the graph every command reads, JSON Schemas for editors, and the real configs of the projects of the chain ([CONFIG_UNITS](design/CONFIG_UNITS.md), [CHAIN_MODEL](design/CHAIN_MODEL.md)).
* **Read-only insight.** The line map (`gero lines resolve`), a project's releases and who holds them (`gero repo status`), and the release-line rules in one place.
* **The update engine and the workstation bump.** Routes, targets, holdings written in place in `yaml`, `gomod` and `regex` formats, edits with steps and strategies, and `gero ship`: commits, PRs, superseding, `--refresh` and fan-out with per-target status. `gero repo branch` and `gero repo tag` do the pieces by hand ([UPDATE_ENGINE](design/UPDATE_ENGINE.md)).
* **Onboarding.** `gero init`, `gero repo clone`, `gero shell install`, and the config bundle each release carries, so a fresh workstation needs no checkout ([CENTRAL_BUNDLE](specs/CENTRAL_BUNDLE.md)).

The first real flow is `scc-operator` into `rancher/rancher` (and `shell` by the same route, with a series for each line): one release fanning out to every active Rancher branch, run by an engineer with `gero ship`.

---

## v1.0.0: a tool the team can rely on

v1.0.0 is about **automation we trust**: `gero` does real work in place of scripts that live in other repos, and the foundation under it is one we are confident to build on. Three streams of work lead to it, and they can run side by side.

### Ship from CI: replace the push scripts

`gero ship` already opens bump PRs for an engineer. A CI variant, `gero ci:ship`, runs the same bump as the bot from a workflow, so the `push-to-rancher` scripts of `scc-operator` and `shell` can go ([CI_SHIP](specs/CI_SHIP.md)).

* `gero ci:ship`: the checkouts the workflow provides, the bot identity from `GERO_BOT_USER`, a push with a token that starts workflows, a step summary and the workflow run URL in the PR body.
* A workflow that installs a pinned, signed release of `gero`, run beside the bash script, then in place of it, and the script removed from `scc-operator` and `shell`.

### The chain: a whole release chain from a workstation

The chain `kuberlr-kubectl` to BRO to `rancher/charts` to `rancher/rancher`, one `gero ship` for each hop, with the tag step between hops by hand ([CHAIN_TRACING](specs/CHAIN_TRACING.md)).

* Reading what a commit or branch holding holds, the charts hop through the Assets image, the walk through several hops in `lines resolve` and `repo status`, and the check of a held tag's stage.
* `chartsPackageBump` and BRO's tag into a charts bump, run in dry-run beside today's scripts first.

### Confidence in the foundation

* The config kinds and field names settled, so `gero/v1alpha2` becomes `gero/v1`.
* The exit-code helper, and a release that is signed, installed and self-updated by the team in practice.
* A contribution guide and a backlog of well-sized issues someone else can pick up without the author ([TODO](TODO.md)).
* A review with the team: show the CI bump and the chain from a workstation, review the config shapes and the direction, and fold feedback in.

**Gate for v1.0.0**
* CI passes and a tagged release publishes signed, installable binaries.
* `gero ship` bumps from a workstation and `gero ci:ship` from a workflow, and `scc-operator` and `shell` use it in place of their scripts.
* The chain flow runs for real from a workstation.
* The config shapes are reviewed by the team and settled as `gero/v1`, and the real configs validate in CI.
* A contribution guide and a backlog of issues exist.

**What it means:** the team can collaborate on it, and we can say we use it, to a limited degree. Scripts that were copied between repositories are gone from the first two.

### Decides before v1.0.0

Final kind and field names; how the path through `rancher/charts` into `rancher/rancher` is walked; how producers align on consumer PR conventions.

---

## v1.x: tracking and standardization

Themes in rough dependency order, not promises. Only stable tags are released.

* **More repos off their scripts.** The same `ci:ship` for other producers, and `gero repo tag` as a workflow step behind a sign-off.
* **Checks over the chain** ([TOOLING_GOALS](design/TOOLING_GOALS.md)): stable readiness and line consistency, read-only, on the walk.
* **Tracking issues:** a versioned state block, a read-only rendered view and labels including stale, with fixture-driven tests ([METADATA](specs/METADATA.md)).
* **The reconciler:** `gero ci:reconcile` on a cron and on `repository_dispatch`, per-operation locks and backoff, observing human-pushed tags first.
* **The chain in CI**, tracked end to end, beside and then replacing today's scripts.
* **Shared workflows and rendered ones:** reusable workflows pinned by `gero` tag or SHA, and rendered workflows from `WorkflowsConfig`, the release workflow last because signing makes it the most sensitive.
* **`VERSION.md`** generated from `branches.lines` and `RancherLines`, and `gero lines sync` for the lifecycle dates.
* **Release trains beyond tracking:** tag creation from the tracker, `repository_dispatch` where cron latency hurts, a stream's `requiresPrerelease` enforced for tags made outside `gero repo tag`.
* **"Branch for X"** automation, and hardening ahead of v2.

**Parallelism.** CI and tracking, standardization, and release trains can run side by side. They meet at "branch for X".

**Decides along the way:** whether the tracker builds a chain up front or reads it hop by hop; whether the reconciler reads repo config on every run or only central config; which "branch for X" steps are automated and in what order.

## v2.0.0: complete

**What it means:** `gero` automates as much of the release work we do by hand today as we intend to.

**Gate:**
* At least one real Rancher line cut ("branch for X") has run through `gero`, in addition to the chains, tracked end to end without manual issue edits.
* The flows it covers have retired their old scripts.
* The team uses it for real releases across a full release cycle.

---

## v3.0.0: QA bridge

* Dispatch a thin QA workflow with a correlation ID, milestone and Rancher version; observe its conclusion; record the result on the tracking issue as evidence. The stable gate stays a human decision.
* **Prerequisite, agreed with the QA owners:** what artifact a QA run proves it tested (Rancher version, chart version or image).

## Moonshot: dashboard

* `gero dashboard`: a local web view over the project graph and tracking issues, for managers and leadership as well as engineers. It reads the same state as the tracking issues, so it can be delivered at any point once that state exists. Unversioned.

---

## Cross-cutting

* **Docs:** each milestone writes or trims the specs it touches, following the [docs rules](README.md). Facts about specific repos go in `projects/`.
* **Testing:** fixture-driven tests for resolution and the reconciler; dry-run and `verify` modes for anything that writes.
* **Dependencies:** stdlib first; a new module needs an entry in [DEPENDENCY_POLICY](specs/DEPENDENCY_POLICY.md) and the depguard allow-list.
* **Interactive and styled output:** `gero repl`, the line editor and color are done ([REPL](specs/REPL.md), [CLI_OUTPUT](specs/CLI_OUTPUT.md)). A full-screen TUI is not planned, and the `internal/ui` seam keeps one possible without touching commands.
* **Migration:** old scripts and `automation-core` tooling are retired per capability, not all at once.
* **Other teams:** another team that wants `gero` can use our releases with their own configs, or fork it. We do not plan special support.
