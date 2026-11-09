# Vision

`gero` is a command-line tool for the release engineering of Rancher: the work of getting a change in one repository into a release of the products that depend on it, across many repositories and release lines, with every step checked and recorded.

> Automating workflows, orchestrating tasks, and powering team ORBS.

## The problem

A Rancher release is a chain. A change in one project is released, taken into the next project, released again, and so on until it reaches Rancher, for each supported release line. Today that work is done by shell scripts in each repository and by people who know which script to run and which file to edit. The knowledge is spread out, the scripts differ, and nothing can answer "what does this repository hold, and is it the release we expect?"

## The goal

One tool, written in Go, that knows the chain and can walk it:

* **Say what is true.** What each repository holds of the others, for each release line, and where it is behind.
* **Do each step the same way.** Bump a holding, commit, push, open the pull request, tag the release, from a workstation or from CI.
* **Record the result.** Release state lives where people can read it and audit it.

The shape of the chain is data, not code. A set of YAML documents says what each project releases, what it holds of the others, and what each Rancher line is. The same config drives every command.

## Principles

* **Composable, in the manner of unix.** A command is small and does one thing. A higher-order command such as `gero ship` chains single-action tasks in Go, not through a shell ([PROJECT_LAYOUT](specs/PROJECT_LAYOUT.md#tasks-single-actions-that-compose)).
* **A companion to `gh`.** `gero` is a standalone binary that reuses GitHub authentication and the idioms of `gh`: noun-verb commands, `--json` output, familiar flags. It does not need `gh` to run ([CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md)).
* **Native GitHub, not a new control plane.** Builds and signing run in each repository's own GitHub Actions. `gero` renders the workflows those repositories need and observes the result.
* **Works in two places.** A command runs on a workstation, in CI, or both, and a command that needs a CI-only credential fails early elsewhere.
* **Observe, then record.** A scheduled reconciler compares the actual state of GitHub (tags, pull requests, check runs) with what a release expects, and writes it to a tracking issue. The issue is the audit trail for engineers, QA and managers.

## Where it is going

The [phases](PHASES.md) are the order of the work, and the [architecture](ARCHITECTURE_DESIGN.md) is the shape.

* **Workstation flows.** Check, bump and ship through the chain, one release line at a time.
* **CI and the reconciler.** The same steps in GitHub Actions, and tracking issues kept up to date by observation.
* **QA evidence.** Automated test results recorded beside manual sign-off for stable promotion. How QA hands results over is undecided.
* **A local dashboard** (a stretch goal): an embedded web view of the project graph and the release trains in flight.

## The name

The team is **ORBS**, after the Dragon Balls of *Dragon Ball Z*. The tool is `gero`, after Dr. Gero, who built tireless machines that watch, gather data and keep going without being asked. That is what we want from release automation. The interface stays plain: `gero repo status`, `gero ship`, `gero lines resolve`, `gero config validate`, `gero doctor`. `gero --help` is the current command tree.
