# Metadata: labels and hidden markers

**Status:** draft

How `gero` tags the PRs and issues it writes, so people and tools can find and read them. Two layers carry the metadata, with different roles. The **hidden marker** is the record. **Labels** are a projection of it for search, the GitHub web UI and other teams' tooling. Labels are tags, not state.

## The two layers

| Layer | Role | Authority |
|---|---|---|
| Hidden marker | Identity and the full machine state: versioned, additive, in the body of a PR or tracking issue | **The source of truth.** The tool never trusts a label over it |
| Labels | Search, filtering, signals to other teams and their automation | **Derived.** Written by the tool, readable by anyone |

* One marker envelope serves PRs and tracking issues: a versioned, additive block with a `kind` and the identifying fields of that kind. What a PR marker holds is in [PUSH_TO_REPO](PUSH_TO_REPO.md#branches-and-pr-identity) and what a tracking issue holds is in [IMPLEMENTATION](../IMPLEMENTATION.md#tracking-issue-state). This spec does not repeat either.
* A missing or corrupt marker is a loud error. A missing or odd label never is.
* Lookup uses the marker. Labels are never a lookup key for identity.

## Label classes

Two classes of label, by who owns the name:

* **Configured labels** belong to the consumer, such as `dependencies`, `area/charts` or `origin/bot`, which shared repositories use for bot-made PRs and which can be set up in every ORBS repository. They come from the consumer's `pullRequests` block and follow the labels the teams already use. The tool applies them **once, when it creates the PR or issue**, and never reconciles them afterwards. A person may change them freely.
* **Gero-owned labels** live in the reserved `gero/` namespace, such as `gero/stale` or `gero/superseded`. The tool applies and removes them, and they are recomputed from the marker.

## Repo classes

Two classes of repo, by who owns the repository:

* **ORBS-owned repos**, where the team can create labels and own the `gero/` namespace.
* **Shared repos**, shared with the whole org (`rancher/rancher`, `rancher/charts`). Only the org's standard labels may be used.

Which class a repo is comes from the `Project`'s `spec.repo.owned` in central config ([APP_CONFIG](APP_CONFIG.md)), not from a guess at runtime.

## The four cases

| | Configured label | Gero-owned label (`gero/*`) |
|---|---|---|
| **ORBS-owned repo** | Applied at creation if it exists. A missing one is a warning and is **not created** | Applied and reconciled. **Created on demand** if missing |
| **Shared repo** | Applied at creation if it exists. A missing one is a warning and is skipped | **Never used.** Nothing is written, and nothing warns, because it is by design |

* **Only gero-owned labels in an ORBS-owned repo are ever created.** Every other missing label is skipped with a warning.
* A shared repo gets no `gero/` labels, so its PRs and issues rely on the marker and the org's own labels. A signal that other teams need there is a configured label, agreed with them.
* The on-demand label has a fixed color and description from a compiled table, so the labels look the same in every repo.

## Applying labels

* **Labels are a separate step after the PR or issue exists.** A label failure never blocks the PR, the issue or a marker update. It is a warning, and the run's exit code does not change for it.
* **Filter before sending.** The repo's labels are fetched once per target per run (paginated, in memory, no disk cache) and compared case-insensitively. Only labels that exist are sent, so one bad name cannot fail a whole batch. Each skipped label is a warning naming the repo and the label. The same check runs in `--preview` and dry runs, which write nothing.
* **A failed apply is still a warning**, for the race where a label is deleted after the fetch.

## Reconciling gero-owned labels

* The reconciler recomputes the owned labels of an item from its marker on every run and fixes any drift. A `gero/` label added or removed by hand is corrected, so labels cannot become a second source of truth.
* Only `gero/` labels are touched. Everything else on the item is left alone.
* The vocabulary is small and stable. Other teams' automation may key off it, so a rename is a breaking change.

## Control labels

Labels are not an input, with one deliberate exception: a small, named set of **control labels** that the ORBS team applies to steer the automation.

* `gero/hold` means **pause**. It is checked when the reconciler or `gero ci:ship` next looks at the item, and a held item gets no new work and a note in the output saying so. Nothing reacts to the label in real time, and work already running is not stopped. It applies to a tracking issue and to a PR:
  * **On a master tracking issue**, it pauses the train at the next reconcile.
  * **On a PR**, `gero` does no further work on that PR: it does not merge it if `gero` owns the merge. A hold does not block supersession: a newer version still closes a held PR ([PUSH_TO_REPO](PUSH_TO_REPO.md#branches-and-pr-identity)). A merge that `gero` does not own, such as a person or GitHub's auto-merge, is out of its control.
  * **A hold does not propagate.** A PR's hold does not hold the train, and a train's hold does not need a label on each PR. The tracking issue's **state** records that the PR is held, so the rendered view and the labels derived from it show it, but the hold on the train itself stays a decision made on the train.
* A control label is **read, never written or removed by the tool.** It is not reconciled away like the other `gero/` labels, which is how the reconciler tells the two apart.
* Control labels exist only in ORBS-owned repos, because the `gero/` namespace does not exist anywhere else. A shared repo is paused by other means (config, or not dispatching).
* Workstation commands do not obey a hold: an engineer who runs `gero ship` has made the decision. They get a warning that the item is held.
* The set is closed and documented here. Adding a control label is a design decision, not a convenience.

## Vocabulary

Configured labels are the existing Rancher labels, named in config where a consumer needs them. This spec names only the gero-owned and control labels:

| Label | Kind | Meaning |
|---|---|---|
| `gero/stale` | Owned | The item's last reconciled time is past the threshold ([IMPLEMENTATION](../IMPLEMENTATION.md#tracking-issue-state)) |
| `gero/superseded` | Owned | A newer version replaced this PR ([PUSH_TO_REPO](PUSH_TO_REPO.md#branches-and-pr-identity)) |
| `gero/hold` | Control | Pause CI-run automation on this item |

## Not built yet

* All of it. `gero ship` today applies the configured labels only, and warns for a missing one.

## Open questions

* Whether a `pullRequests` block can name labels per class, or a global config injects the existing Rancher labels.
* The full gero-owned vocabulary, such as phase labels on tracking issues.