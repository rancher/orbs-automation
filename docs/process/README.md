# Release Process

**Status:** staging. This set is being written to restate how a Rancher release is made, and how each project takes part, before the config shapes and the tooling that read them are redesigned. When it settles, the other docs are rewritten around it. Nothing in `../projects/`, `../specs/` or `../IMPLEMENTATION.md` is replaced yet.

## What these docs are for

They describe the **process**: what happens, in which order, who does it, and why. They are not about the scripts or the tooling that carry it out today, and they are not about how `gero` models it. That comes after: the configs are designed from these docs, and the tooling is designed from the configs.

An existing project note in [`../projects/`](../projects/README.md) records how the repo works today and is evidence for a process doc. A process doc is the cleaned-up account of what is meant to happen, and says so where today's practice differs.

## Order of work

1. [`RELEASE_PROCESS.md`](RELEASE_PROCESS.md): the whole process at the high level (draft).
2. One doc for each project, from [`TEMPLATE.md`](TEMPLATE.md).
3. A review of the configs against them, then the Go design.

## Writing rules

* **Facts and rules, terse.** Lists, one fact per bullet, fragments fine. A fact says what is; a rule says what must be, then why.
* **Read the scripts, do not cite them.** Whoever fills in a doc may read a repo's workflows, scripts and configs to learn what really happens, and should. The doc says what happens and who does it, and never names, links or quotes a script, workflow file, command or tool as a source. If a step needs one to make sense, name the artifact it handles instead. The doc stays true if every script is rewritten. The one exception is a config or lock file named as the source of truth for a fact ("Recorded in", "Where it stands"): say what fact it holds, not how it is edited or generated.
* **Document the release, not the changes.** Every release starts with changes that are reviewed, then merged. Do not list what the changes are, where they come from, how often they arrive or what proposes them. Describe what happens from the point a release is wanted: when it is cut, what gates it, who decides, what is published, what is handed on.
* **Follow the template's marks.** Headings, bold labels and the table header are kept as written; `<angle brackets>` are replaced; comment blocks are instructions and are deleted. A section that does not apply says "None." ([TEMPLATE](TEMPLATE.md)).
* **Name who does each step** (a person, a team, or automation). A step done by hand is a step.
* **A line is a Rancher line (`v2.16`).** A *stage* is a tier of one project's releases (`dev`, `stable`). A *hop* is one hand-off between two projects. Use these words only for these.
* **Write differences by line as ranges** (`from v2.16`, `until v2.15`).
* **Mark what is not confirmed** with *(unconfirmed)*. A process doc may be wrong where the repo's own docs and workflows disagree, and saying so is better than choosing silently.
* **Tooling goals go elsewhere.** A goal for the tooling that turns up while writing a process doc (for example, a check the tool should make) goes in [`../design/TOOLING_GOALS.md`](../design/TOOLING_GOALS.md), not in the process doc.
* **Config notes stay as questions and facts,** with no field names. Designing the fields is the next stage.

## Index

* [kuberlr-kubectl](kuberlr-kubectl.md)
* [backup-restore-operator](backup-restore-operator.md)
* [rancher-assets](rancher-assets.md)
* [scc-operator](scc-operator.md)
* [shell](shell.md)
* [rancher-charts](rancher-charts.md)
* [rancher](rancher.md): the destination
