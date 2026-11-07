# Phases and Milestones

How we turn [VISION](VISION.md) into reality, using the design in [ARCHITECTURE_DESIGN](ARCHITECTURE_DESIGN.md) and the decisions in [IMPLEMENTATION](IMPLEMENTATION.md). **Draft: milestones, numbering and order are proposals.**

Milestones are tied to `gero` version tags, and each headline tag has an **audience** and a **message**. Versions between the headline tags are planning labels; real tags may differ. There are no dates here by design.

## Principles

* **Each milestone stands on its own** and is useful even if later ones slip.
* **Safest and most reviewable first.** Read-only and PR-based work comes before anything that writes without a human reviewing it.
* **Smallest real slice, then widen.** Start with one real flow end to end.
* **Open questions are answered by the code that needs them.** Each milestone lists what it decides.
* **Workstation first, then CI.** A command runs on an engineer's machine, as them and with their own credentials, before it runs in a workflow as the bot, over the same backends ([EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)). The team uses each one by hand while its CI form is built.
* **Run alongside, then switch.** A new `gero` path runs next to the script it replaces until we trust it.
* **Shareable work.** Where milestones do not depend on each other they can be built in parallel by different people.

## The Plan at a Glance

| Tag | Name | Who builds it | Audience | What we say | Gate |
|---|---|---|---|---|---|
| **v0.x** | Solo foundation and POC | The author, with team review | Nobody outside the team | Nothing | POC accepted by the team, then the chain flow runs for real |
| **v1.0.0** | Foundation and a headline automated, tracked chain | The author | The wider organization, to a limited degree | "We have a tool, it already automates and tracks a multi-repo release chain, and the team is building on it" | Peer-ready and the chain flow runs for real |
| **v1.x** | Team build-out | The team | The team, and early users | Progress updates | Real flows run through `gero`; stable tags only |
| **v2.0.0** | Complete | The team | The wider organization and leadership | "Most of the release work we did by hand is now automated" | A real release cycle run through `gero` |
| **v3.0.0** | QA bridge | The team with QA | Engineering and QA leadership | "Engineering and QA are now in sync through shared tooling" | QA results appear on release tracking |
| Moonshot | Dashboard | Anyone | Managers and leadership | "Anyone can see the state of releases" | Delivered whenever it is ready; unversioned |

Until v1.0.0 the tool is not announced. From v1.0.0 only stable tags are released (no prereleases of `gero` itself). Config documents use `apiVersion: gero/v1alpha1` during v0.x, so the team can review and agree on the shapes before they become `gero/v1` with v1.0.0.

---

## v0.x: Solo foundation and POC

**Owner:** the author, building a foundation others can build on, plus two real flows end to end. Not announced outside the team. v1.0.0 is about **automation and tracking**, not file generation.

Two flows, in order:
* **Flow A, `scc-operator` into `rancher/rancher` (the POC).** One release line fanning out to every active Rancher branch. It exercises the update dispatcher, `yamlBump`, per-target status, the tracker and the reconciler with the fewest moving parts. `shell` takes the same route with a series for each line. An engineer runs the bump with `gero ship` from their own clone; in CI it is still a bash script triggered by a stable tag, until `gero ci:ship` replaces it. Its job is to prove to the team that the approach is good.
* **Flow B, `kuberlr-kubectl` and everything it touches (the v1.0.0 highlight).** The full chain `kuberlr-kubectl` to BRO to `rancher/charts` to `rancher/rancher`: `yamlBump`, `chartsPackageBump` (the most complex strategy), multi-hop tracking and the tag step between hops.

| Label | Work | Depends on |
|---|---|---|
| v0.1 | **Foundations:** Go module, CLI skeleton per the [specs](specs/README.md) (cobra root, global flags, `internal/clicfg` loader with precedence tests, `log/slog`, exit-code helper, `internal/ui` seam, depguard allow-list), CI for this repo, tagged binary releases, config loader and `gero config validate` (runs without credentials), `gero config show --resolved`, the GitHub access layer (including a spike confirming the **(unverified)** items in [GITHUB_ACCESS](specs/GITHUB_ACCESS.md) and the YAML library choice), CI-only versus either-context gating. Real configs for **all** repos, written and corrected up front even before each is needed | none |
| v0.2 | **Read-only insight:** tag classification, the line map (`gero lines resolve`), a project's releases and who has picked them up (`gero repo status`, which also names what each line can have next), and the release-line rules in one place (`internal/git/releasetag`) for every command to share | v0.1 |
| v0.3 | **Update dispatcher, `yamlBump` and the workstation bump:** target, branch resolution from the line map, named strategy, steps with a commit for each, per-consumer `pullRequests` conventions, deterministic branches, reuse and supersede of open PRs, `--refresh`, preconditions, a release for each line, fan-out with per-target status. The commands that make the pieces by hand: `repo branch`, `repo tag`. Used by the team beside the existing script | v0.2 |
| v0.4 | **`gero ci:ship`:** the same bump as the bot from a workflow ([CI_SHIP](specs/CI_SHIP.md)): a temporary clone, the bot identity, a push with a token that starts workflows, a step summary, steps from the producer and the central bundle. Run beside `push-to-rancher.yml`, then in place of it | v0.3 |
| v0.5 | **Tracking issue format:** versioned state block, read-only rendered view, labels including stale; fixture-driven tests | v0.2 |
| v0.6 | **Reconciler:** `gero ci:reconcile` on cron, per-operation locks, backoff; observe human-pushed tags first | v0.4, v0.5 |
| v0.7 | **Flow A runs for real** through `gero` in CI and tracked end to end, beside and then replacing today's script | v0.6 |
| | **POC review with the team:** show Flow A, review the config shapes and the direction, and fold feedback in before going further | v0.7 |
| v0.8 | **More strategies:** `chartsPackageBump`, run in dry-run beside today's scripts | POC review |
| v0.9 | **Multi-hop chains:** chain views and the tag step between hops, tracked in one issue; the line map follows a chain through another project to its end (`gero lines resolve` follows one hop today) | v0.8 |
| v0.10 | **Flow B runs for real** through `gero`, plus hardening ahead of v1.0.0 | v0.9 |

### Decides in v0.x

Final kind and field names; whether chains are built up front or hop by hop; how the reconciler is triggered; who makes the clone for `gero ci:ship` (the tool or the workflow's checkout, [CI_SHIP](specs/CI_SHIP.md#open-questions)); whether the path through `rancher/charts` into `rancher/rancher` is modeled so the line map connects it; how producers align on consumer PR conventions.

---

## v1.0.0: Foundation and a headline automated, tracked chain

**Gate:**
* Repo layout, contribution guide, and how to run tests and a dry-run locally.
* CI passes and a tagged release publishes installable binaries.
* Config validates; the schema and defaults are documented in code; the team has reviewed the config shapes (at the POC review), and they move from `gero/v1alpha1` to `gero/v1`.
* The GitHub access layer exists with narrow, fakeable interfaces.
* Flow A (`scc-operator`) and Flow B (the `kuberlr-kubectl` chain) both run for real: `gero` opens the bump PRs, the reconciler tracks them (including human-pushed tags), and the old scripts are retired or demonstrably replaceable.
* Real configs exist for all repos and validate in CI.
* A backlog of well-sized issues that someone else can pick up without the author.

**What it means:** the team can start collaborating, and we can start telling people we use it, to a limited degree. The chain is the story.

---

## v1.x: Team build-out

Planning labels, not promises, roughly in dependency order. Only stable tags are released.

| Label | Work | Depends on |
|---|---|---|
| v1.1 | `VERSION.md` generator and verify: generate from `release.lines` and `RancherLines` with a generated header, dry-run diff, PR on the default branch | v1.0.0 |
| v1.2 | Shared reusable workflows (CI, head builds, FOSSA), pinned by `gero` tag or SHA | v1.0.0 |
| v1.3 | Rendered workflows from `WorkflowsConfig`; the release workflow last (signing and attestation make it the most sensitive) | v1.2 |
| v1.4 | Release trains beyond tracking: tag creation from the tracker through a workflow in the target repo (the CI form of `gero repo tag`, behind the tracker's sign-off), `repository_dispatch` where cron latency hurts, `release.requiresPrerelease` enforced for tags made outside `gero repo tag` too, with a per-release override | v1.0.0 |
| v1.5 | "Branch for X" automation and hardening ahead of v2 | v1.1, v1.3, v1.4 |

**Parallelism.** Two streams can run side by side after v1.0.0: standardization (v1.1 to v1.3) and release trains (v1.4). They meet at v1.5.

**Decides along the way:** the standard `VERSION.md` layout and whether older readers need the old formats; how stubs pin workflow versions; whether the reconciler reads repo config on every run or only central config; the override mechanism; which "branch for X" steps are automated and in what order.

## v2.0.0: Complete

**What it means:** `gero` automates as much of the release work we do by hand today as we intend to.

**Gate:**
* At least one real Rancher line cut ("branch for X") has run through `gero`, in addition to the chained release from v1.0.0, tracked end to end without manual issue edits.
* The flows it covers have retired their old scripts.
* The team uses it for real releases across a full release cycle.

---

## v3.0.0: QA bridge

* Dispatch a thin QA workflow with a correlation ID, milestone and Rancher version; observe its conclusion; record the result on the tracking issue as evidence. The stable gate stays a human decision.
* **Prerequisite, agreed with the QA owners:** what artifact a QA run proves it tested (Rancher version, chart version or image).

## Moonshot: Dashboard

* `gero dashboard`: a local web view over the project graph and tracking issues, for managers and leadership as well as engineers. It reads the same state as the tracking issues, so it can be delivered at any point once that state exists. Unversioned.

---

## Cross-cutting

* **Docs:** each milestone writes or trims the specs it touches, following the [docs rules](README.md). Facts about specific repos go in `projects/`.
* **Testing:** fixture-driven tests for resolution and the reconciler; dry-run and `verify` modes for anything that writes.
* **Dependencies:** stdlib first; a new module needs an entry in [DEPENDENCY_POLICY](specs/DEPENDENCY_POLICY.md) and the depguard allow-list.
* **TUI:** not scheduled. The `internal/ui` seam lands in v0.1 so a TUI can be added after v1.0.0 without touching commands.
* **Migration:** old scripts and `automation-core` tooling are retired per capability, not all at once.
* **Other teams:** another team that wants `gero` can use our releases with their own configs, or fork it. We do not plan special support.

## Open Questions for Phasing

None at the moment.
