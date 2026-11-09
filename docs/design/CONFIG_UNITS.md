# Config Units

**Status:** decisions, 2026-11-07, built in v1alpha2, with the Go module additions of 2026-11-08: the `gomod` Artifact kind and format, `at` as a location or a list with `lines`, `{prefix}` and `{bareTag}`, `{version}` without a stream's prefix everywhere it is used, reading and writing every location of a Holding in `yaml` and `gomod`, `goBump` and its commit, the check of a `gomod` Artifact on the Go proxy, and the `locations-differ` finding that `repo status` raises. The decisions so far on breaking the config into smaller units, kept in one place. [CONFIG_REVIEW](CONFIG_REVIEW.md) has the gaps and questions this answers; the [fixture](fixture/README.md) writes the seven projects in this shape; [CONFIG_SCHEMA](CONFIG_SCHEMA.md) is the field reference. Breaking changes are acceptable while the version is `gero/v1alpha2`; the real config is written in it ([CUTOVER](CUTOVER.md)).

## What the tooling works from

* **Declared:** the config says which repo holds which release of what, where, and how to read it
* **Observed:** the state of the repos on GitHub at a ref, and public facts such as lifecycle dates
* **Out of scope:** internal release dates ([CONSTRAINTS](CONSTRAINTS.md))

Every question the tools answer compares declared with observed: readiness, line consistency, status, `lines resolve`, shipping.

## One lookup, four steps

To say what a repo holds, and whether that is stable, the tool does four things.

1. **Locate:** open a file at a ref and find a place in it (a Holding's `at`)
2. **Parse:** turn the text found there into a version, a commit or a branch name (a Holding's `holds.form`)
3. **Interpret:** know which artifact, and so which stream, it is from, and so which scheme classifies it (a Holding's `refs`)
4. **Classify:** stable or prerelease, which line, newer or older (the Stream's scheme)

Writing is the same path in reverse: render the form from a release and put it at the location.

## Decided

* A Project is too large a unit; it is split into smaller kinds that can still share one file
* Every document has a `name`, unique within its kind; a name identifies it in logs and errors and can be used to pick it
* References are explicit names; nothing depends on which file a document is in or its place in the file
* A top-level `refs` key holds every reference to another document, and only names (a name or a list of names); anything that qualifies a reference goes in `spec` or becomes its own kind
* What a release produces is its own kind, `Artifact`, a separate document with `spec.kind` of `image`, `file` or `gomod`
* A `gomod` Artifact is a Go module. Its `name` is the module path, and a release exists when its tag is on the repo and the version is on the public Go module proxy. The proxy is not configurable yet; a later field can name another. The kind is named for the module type, not `module`, so another language's modules are a new kind
* A Holding with `format: gomod` writes the `require` line for a module path; what else a Go bump needs (`go mod tidy`) is an Edit with the named strategy `goBump`
* A tag may carry a path prefix (`helmtools/v0.1.2`, as Go requires for a module in a subdirectory). `{tag}` is the tag as published; `{prefix}`, `{bareTag}` and `{version}` split it (see Templates)
* A module's major version (`/v3`) is part of its path and not of its tag, so each major is its own Stream (`tagPattern: "v3.*"`) and its own `gomod` Artifact. Moving a consumer to the next major is a new path, so a new Holding, not a bump
* A line of a consumer stays on one minor of a library and takes only its patches, so each minor is its own Stream (`tagPattern: "v3.8.*"`, `universal`) and its own Artifact, even when the module path is the same. Which line holds which minor is a fact about the consumer (BRO and Rancher hold different minors for one line), so it is written on the consumer's Holdings with `lines`, not on the Stream. The library's own Kubernetes ranges are not in the config ([`wrangler.yaml`](fixture/wrangler.yaml))
* **A Holding's locations agree.** The locations of one Holding should hold the same version for a line; a difference is a warning (Rancher main: `pkg/client` and `pkg/plan` on `rc.2` while the root and `pkg/apis` are on `rc.3`). `replace` directives in a `go.mod` are not modelled
* **Line consistency is derived from the layout.** A Project with `branches.layout: perLine` is compared with the `finalDestination` (Rancher): for a module or image both hold, the stream it holds for a line should be the stream Rancher holds for that line, and a difference is a warning. A holder with `layout: single` (SCC operator) has no lines of its own and is not compared. It needs no field: it follows from the layouts and the Holdings
* **Commits are the engine's, one for each step.** Shipping commits after the Holding writes and after each Edit step or strategy phase, and skips a commit when a step changed nothing. There is no setting for it; a Holding, Edit or strategy does not decide where a commit falls
* The second stage is `stable`; a scheme supplies a project's default stages, and a project writes `stages:` only to override (review Q1)
* A Holding is one document with two blocks: `at` is where to look, and `holds` is what it holds of a producer and how the text is written. The fixture showed every location has exactly one such tie. Revisit and split the kind if a case needs one location tied to two producers. The reverse, one producer held at several locations, is a list: `at` is a location or a list of locations, each with its own `file`, `key`, `format` and optional `lines`, all tied to the same artifact and `holds` (Rancher's nested `go.mod` files)
* The `form` is a template; the variables it may use are listed under Templates
* A key is rendered from its template first, then read as a path in the file's own style: for `yaml` that is the path syntax `yq` uses (`.global.kubectl.tag`, `.chart-versions."2.16"`, `.additionalCharts[0].url`). The schema documents the subset it supports (keys, quoted keys, list items); a leading dot is optional. A key in a YAML flow mapping needs quoting when it contains `[`
* The schema lists only the formats a project needs now (`yaml`, `dockerfile`, `gomod`) and documents the others, so adding one is a new row, not a new idea
* Templates take modifiers (`{line|trimprefix:v}`, `{stage|replace:stable=prod}`) from a small closed set, so a respelled value needs no map of its own

## Units (proposed)

*The table and examples are the proposal as it stands. Two points are leans, not decisions: branches on the Project and series on the Stream (O5), and a commit or branch held from a Project (O10).*

| Kind | Does | `refs` | `spec` |
|---|---|---|---|
| `Project` | the repo as a place, and its branch layout | none | `repo` (name, `owned`, `external`, `managedBy`, default branch), `finalDestination`, `pullRequests` (title, branch prefix, labels, push target), `branches` (layout, stages, and the branch for each line) |
| `Stream` | what a project releases: the versions and the lines | `project` | `scheme`, `stages` (override only), `lines` (each with its series), `universal` |
| `Artifact` | one thing each release of a stream produces | `stream` | `kind` (`image`, `file` or `gomod`), `name` (an image's repository path, a file name template such as `rancher-backup-{version}.tgz`, or a Go module path), `lines` (which lines produce it), `registries` (an image's override of the default by release kind) |
| `Holding` | a named rule: where a piece of information lives in a repo, and what it holds | `project` (whose repo), and `artifact` (a release) or `from` (a project whose commit or branch it holds) | `at` (a location, or a list of locations: `file`, `key`, `format`, `lines`) and `holds` (`kind`: `release`, `commit` or `branch`; `form`; `lines`; `stage` or `stages`; `accept`) |
| `Edit` (later) | extra steps when shipping, beyond writing the form | `holding` | steps before and after, and a named strategy for a rule (a chart version rule, regeneration) |
| `Policy` | rules shared across projects | as needed | RC-first, registries |
| `RancherLines` | public line facts | none | unchanged |

* A project can have several streams (charts: one for each package), and a stream has one artifact for each thing a release produces (BRO: an image and two chart packages; shell: an image)
* An image artifact is checked in the registries its release kind goes to, a file artifact on the GitHub release, and a `gomod` artifact on the repo's tag and the Go module proxy; a published release missing a declared artifact is drift
* Holdings are declared by the consumer, which knows its own files; "who consumes X" is derived
* A holding can name an artifact or project with no configured project, so a component we do not trace can still be located and classified
* Read-only first: readiness, consistency, status and resolve need only Streams, Artifacts and Holdings

### Templates

A template is text with `{variable}` parts, each with optional modifiers. Two places use one, and they have different variables. An unknown variable is an error when the config loads. Variables come from release and layout facts, never from free text.

**In `at`** (`file` and `key`), filled for each line and stage the holding is read for:

| Variable | Is | Example |
|---|---|---|
| `{line}` | the line | `v2.16` |
| `{stage}` | the stage, as we name it | `dev`, `stable` |
| `{branch}` | the holder's branch for that line and stage | `release/v2.15` |

**In `holds.form`**, filled from the release and its artifact:

| Variable | Is | Example |
|---|---|---|
| `{image}` | an image artifact's repository path | `rancher/shell` |
| `{tag}` | the release tag as published | `v0.9.1-rc.2`, `helmtools/v0.1.2` |
| `{prefix}` | the stream's tag prefix: its `tagPattern` up to and including the last `/`; empty when there is none | `helmtools/` |
| `{bareTag}` | the tag without the prefix | `v0.1.2` |
| `{version}` | the tag without the prefix and without a leading `v` | `0.9.1-rc.2`, `0.1.2` |
| `{url}` | a file artifact's download URL | `https://github.com/…/rancher-backup-12.0.0.tgz` |
| `{file}` | a file artifact's name | `rancher-backup-12.0.0.tgz` |
| `{sourceRepo}` | the producer's owner/name | `rancher/shell` |
| `{commit}` | the release's commit | a sha |
| `{base}`, `{upstream}` | a chart version split at `+up` | `111.0.0`, `0.12.1-rc.16` |

More variables are added as a case needs them; each is a release or layout fact. `{version}` is `{bareTag|trimprefix:v}`, kept as a shorthand because it is so common. (Before the prefix variables it was `{tag|trimprefix:v}`, which left a prefixed tag unchanged: `pkg-a/v1.2.3` stayed `pkg-a/v1.2.3`.)

**Modifiers** respell a value where it is used. They follow the variable after `|` and apply left to right: `"{line|trimprefix:v}"` turns `v2.16` into `2.16`, and `"{stage|replace:stable=prod}"` turns `stable` into `prod` and leaves `dev` alone.

| Modifier | Does |
|---|---|
| `upper`, `lower`, `title` | change case |
| `trimprefix:P`, `trimsuffix:S` | remove a prefix or suffix |
| `replace:FROM=TO` | replace the whole value when it equals FROM (not a substring) |

The set is closed and small, and grows only when a case needs one. An unknown modifier is a load error. The template says how pieces are put together; a modifier says how one piece is spelled.

**Reading as well as writing.** A `form` is rendered to write a value and matched against the text to read one, so it may use only modifiers that can be undone: `trimprefix`, `trimsuffix` and `replace`. `upper`, `lower` and `title` lose information and are for `at`, which is only ever rendered.

### Formats

`at.format` names a kind of file. `at.key` means what that format says, and is rendered from its template before it is read.

| Format | `key` is | Example | Listed |
|---|---|---|---|
| `yaml` | a path in `yq` syntax: keys, quoted keys for a dot inside one, `[n]` for a list item | `.chart-versions."2.16".upstream-refs.dev.charts.commit` | now |
| `dockerfile` | an `ARG` or `ENV` name | `CHART_DEFAULT_BRANCH` | now |
| `json` | the same path syntax as `yaml` | `.version` | documented |
| `toml` | the same path syntax as `yaml`, with table names as keys | `.package.version` | documented |
| `env` | a variable name in a `KEY=value` file | `VERSION` | documented |
| `makefile` | a variable name in `VAR = value` or `VAR ?= value` | `IMAGE_TAG` | documented |
| `gomod` | a module path; reads and replaces the version on that module's `require` line in `go.mod` | `github.com/acme/libfoo` | now |
| `regex` | a pattern with one capture group; the escape hatch for any other file | `^version: (.*)$` | documented |

Every format must let a tool do two things: read the text at a key, and replace it leaving the rest of the file unchanged, comments included. A format the schema does not list is an error when the config loads. A documented format becomes listed when a project uses it.

### Examples

```yaml
kind: Holding
name: rancher-holds-assets
refs: {project: rancher, artifact: assets-image}
spec:
  at: {file: build.yaml, key: defaultAssetsImage, format: yaml}
  holds: {kind: release, form: "{image}:{tag}", accept: all, lines: {since: v2.16}}
```

The older Rancher branches keep a charts branch in a Dockerfile, which is the same shape with another format, and holds a branch of a project, not an artifact:

```yaml
kind: Holding
name: rancher-follows-charts-branch
refs: {project: rancher, from: rancher-charts}
spec:
  at: {file: package/Dockerfile, key: CHART_DEFAULT_BRANCH, format: dockerfile}
  holds: {kind: branch, lines: {until: v2.15}}
```

Assets holds a commit of charts, one for each line and stage, in a key that varies and spells the line as `2.16` and the second stage as `prod`:

```yaml
kind: Holding
name: assets-pins-charts
refs: {project: rancher-assets, from: rancher-charts}
spec:
  at:
    file: lock.yaml
    format: yaml
    key: '.chart-versions."{line|trimprefix:v}".upstream-refs.{stage|replace:stable=prod}.charts.commit'
  holds: {kind: commit, stages: [dev, stable]}
```

BRO's one release produces an image and two chart packages. Charts' package file holds the URL of each chart, so two holdings, each to its own artifact, not several edits on one edge:

```yaml
kind: Holding
name: charts-holds-bro-chart
refs: {project: rancher-charts, artifact: bro-chart}
spec:
  at: {file: packages/rancher-backup/package.yaml, key: url, format: yaml}
  holds: {kind: release, form: "{url}", stage: dev, accept: all}
---
kind: Holding
name: charts-holds-bro-crd-chart
refs: {project: rancher-charts, artifact: bro-crd-chart}
spec:
  at: {file: packages/rancher-backup/package.yaml, key: '.additionalCharts[0].upstreamOptions.url', format: yaml}
  holds: {kind: release, form: "{url}", stage: dev, accept: all}
```

### Where today's fields go

| Today | New home |
|---|---|
| `repo.*`, `finalDestination`, `pullRequests` | Project |
| `release.branching` and the branch for each line | Project `branches` |
| `release.lines[].version`, `scheme`, `requiresPrerelease` | Stream |
| `images` | Artifact (`kind: image`; `since` becomes `lines`) |
| `updateMethods` (`with.file`, `updateKey`) | Holding `at` (`file`, `key`) |
| edge `params.value` | Holding `holds.form` (a hand-built URL becomes `{url}`) |
| edge `lines`, `stage`; method `accept` | Holding `holds` |
| edge and method `preEdit` and `postEdit`; strategies | Edit |
| `downstream` | derived from Holdings |
| `External` and its relationships | no kind: a third-party repo is a Project with `repo.external: true`; `trackOnly` and `react` are a Holding that holds its release (observed); `notify` is not modelled until a case needs it |

## What the fixture showed

The [fixture](fixture/README.md) writes nine projects (the seven of the baseline plus the frameworks team's wrangler and lasso), plus a Go library, a monorepo, a prefix-tagged module and an external SDK, in this shape. All references resolve, and every tool command finds what it needs.

* **Five forms covered every holding:** `{image}:{tag}`, `{tag}`, `{version}`, `{url}` and (a Go module) `{bareTag}`; a commit or a branch needs none. Other projects will need more variables, which is why the catalog is open
* **Only one holding needs a modifier:** Assets' lock file (`{line|trimprefix:v}` and `{stage|replace:stable=prod}`). Everything else reads a plain key
* **BRO's two URLs need no multi-edit.** Two holdings do it. Only two Edits were needed in the whole fixture
* **Branches belong to the Project, series to the Stream.** A repo has one branch layout for every line (BRO's `release/v11.x`), while charts has about twenty streams each with its own series on the same branches (O5)
* **Stages follow the layout.** Charts' stages are its two branches for each line (a Project fact); Assets' come from its tag scheme (a Stream fact)
* **A commit or a branch is of a repo, not of an artifact.** Assets holds a charts commit and older Rancher holds a charts branch (O10)
* **Consumer steps attach to a file, not a holding.** Rancher regenerates its build config after any change to `build.yaml` (O11)
* **The shape is not Rancher-specific.** The library and monorepo fit with `universal`, `tagPattern` and the format `dockerfile`; lines and stages are simply absent
* **A Go module fits the same kinds.** Wrangler and lasso are a Project, a Stream for each minor and an `Artifact` of kind `gomod`; ob-charts-tool adds a prefix-tagged module. Rancher holds them in four `go.mod` files, so a Holding takes a list of locations
* **Consumers disagree, so the line mapping is theirs.** For one line, BRO and Rancher hold different wrangler minors (v2.16: 3.7 against 3.8), so which line holds which minor is written on each consumer's Holdings
* **Drift is visible in the data.** Rancher main holds wrangler `rc.3` in the root and `rc.2` in `pkg/client` and `pkg/plan`
* **Verbosity:** shell is three documents, Assets four, charts five, BRO six, Rancher eight. All can share one file

## Open

Provisional: none of these is forced by the shell and SCC to Rancher baseline, which the new shape expresses field for field. Each takes its lean until the charts bump is built, then is revisited. Each has the context, the options and where I lean.

### O4. What is the shape of an Edit now?

* **Context:** writing a holding is automatic when gero ships: render the form, put it at the location. What is left for an Edit is a rule or a step.
* **Evidence:** the fixture needed two Edits. Charts' bump carries a strategy and a package (`chartsPackageBump`). Rancher's regeneration after any change to `build.yaml` is a step that belongs to the consumer's file, not to one holding (O11).
* **Option A, steps only.** **Option B, steps and a named strategy** for a rule that needs code. **Option C, no Edit kind:** steps and strategy on the holding.
* **Lean B.** No multi-edit shape is needed.
* **Second case: `goBump`.** A Go module consumer needs more than the written line: `go mod tidy` follows the `require` change. It is an Edit with `strategy: goBump`, over a Holding with `format: gomod`, and takes no values of its own yet. Commits are the engine's (see Decided), so the `require` change and the tidy are two commits with nothing in the strategy about it ([`examples/gomod.yaml`](fixture/examples/gomod.yaml)). `goBump` is built: it is given the clone's directory, the step runner (so a program runs as a step's does, by name from an allow-list) and what its edit's holdings wrote, runs `go mod tidy` once in the directory of each `go.mod` written, and reports a change only for a module whose `go.mod` or `go.sum` tidy changed. A `goBump` Edit needs a holding with a `gomod` location. No real project uses it yet: Assets is only observed for now, and ob-charts-tool's `goBump` is an example, not a decision to ship into Assets.

### O5. Do lines and series live in the Stream or in their own kind?

* **Context:** a project's branch for each line, and a stream's version series for each line.
* **Evidence:** they are different things. A repo has one branch layout for every line, while a repo can have many streams, each with its own series (about twenty charts, each `111.x` for v2.16).
* **Option A, both in the Stream.** Repeats the branch layout for every stream of a repo. **Option B, branches on the Project and series on the Stream** (as the fixture does). **Option C, a `LineMap` kind.**
* **Lean B.** It removes the repetition for charts, and a branch is a fact about a repo.

### O6. How do `owned`, observed edges and sending fit?

* **Context:** `repo.owned` says gero ships from a repo, and an edge from an unowned producer is only observed. With holdings declared by the consumer, "who ships" is not where the edge is written. A producer may also send by itself (shell every release, SCC a stable and an RC on request).
* **Option A, derive it:** gero can ship a holding when the artifact's project is `owned` and an Edit has what it needs. **Option B, `mode: observe | ship` on the holding.** **Option C, a producer-side `Delivery` kind** for what a stream sends by itself.
* **Lean A now, C if a status view needs the expectation.**

### O7. Where do the policies live?

* **Context:** RC-first with a CVE override, registries by release kind, PR conventions; ORBS projects share most of them.
* **Option A, where they are** (`requiresPrerelease` on the Stream, PR conventions on the Project, registries in code). **Option B, a `Policy` kind** referenced by `refs`. **Option C, a default Policy with an override.**
* **Lean A now,** B when repeating a rule hurts.

### O8. What are the kinds called?

* **For `Stream`:** `Release`, `ReleaseStream`. **For `Artifact`:** `Output`, `Product`, `Asset` (clashes with a GitHub release asset, which is only one kind). **For `Holding`:** `Location`, `Locator`, `Pin` (fits a commit more than a tag).
* **Lean `Stream`, `Artifact`, `Holding`.**

### O9. What does an Artifact check?

* **Context:** an image is checked in registries, as `gero repo images exist` does; a file is checked on the GitHub release by name. A release should carry attestation, and a publish should end with every declared artifact present.
* **Option A, exists only.** **Option B, exists and is attested,** with an `attested` field. **Option C, a required flag,** so an optional artifact (a debug binary) does not fail a release.
* **Lean A now,** with C when an optional artifact appears and B when the tooling can read attestations.

### O10. How does a Holding name what it holds?

* **Context:** a release is held as an artifact (an image tag, a chart URL). A commit or a branch is of a repo, not of an artifact.
* **Evidence:** Assets holds a charts commit, and older Rancher holds a charts branch name; both name a project, not an artifact.
* **Option A, `refs.artifact` for a release and `refs.from` (a project) for a commit or a branch** (as the fixture does). **Option B, always a stream,** with a qualifier. **Option C, commits and branches also become artifacts.**
* **Lean A.**

### O11. Where do the consumer's steps attach?

* **Context:** after any change to `build.yaml`, Rancher runs a regeneration step, whichever holding changed it. Today it is `postEdit` on the consumer's method, with line ranges.
* **Option A, an Edit that refs every holding it follows** (as the fixture does). Repeats the list when a file gains a holding. **Option B, an Edit that refs the project and names files.** One place for a file. **Option C, steps in the holding's `at` block,** repeated for each holding of one file.
* **Lean A (changed from B).** B adds a concept (named files) the baseline does not need; A works today and can change when the repetition hurts.

### O12. How do two packages say their versions go together?

* **Context:** lasso `v0.2.*` goes with any wrangler `v3.*.*`. That is a fact about two producers' releases and matters to every project holding both (BRO, Rancher), not to how one flows into the other. No kind says it: a Holding is a consumer's file, a Stream is one producer's releases, and a Policy is a project's practice.
* **Option A, nothing yet:** today every pair held is inside the one range that exists, so a check would never fail. **Option B, a `Pairing` kind** that refs two or more `gomod` Artifacts and selects the releases of each that go together (`series`); a project holding both must hold a declared pair, and a failure is a warning. **Option C, derive it** from each module's own `go.mod`, which already holds the hard minimums.
* **Lean A now, B** when a case needs it: lasso `v0.3` or higher, or wrangler `v4` or higher. Until one exists the shape is a guess, and the cross-version matrix is what such a case would show. Go's own resolution keeps enforcing minimums meanwhile.

### O13. How is a warning acknowledged?

* **Context:** warnings are only ever shown. Today's are a lifecycle date, a line on the default branch after it is past development, a circular holding and an unknown key; this work adds two more (a Holding's locations disagree, and a perLine project holds a different stream than Rancher for a line). The two new ones will fire on real data from the first run (BRO v2.14 and v2.12), so a deliberate difference needs a way to be said.
* **Evidence:** a finding has a severity and the path of the document it names (`internal/platform/config/report.go`) but no stable id for its rule, so nothing can say "this one is expected".
* **Option A, none:** warnings stay and are counted by `gero doctor`. **Option B, an acknowledgement on the document the finding names:** `acknowledge: [{rule, reason}]`, with the reason required, and an acknowledgement that no longer matches a finding is itself a warning. **Option C, a separate `Acknowledgement` kind,** in one place. **Option D, a user file:** rejected, as it would differ by machine.
* **Decided B, in the envelope** ([CONFIG_SCHEMA](CONFIG_SCHEMA.md#acknowledging-a-warning)): `rule` and `reason` required, `lines` optional, no date. Silent by default and shown with its reason under `-v`. Built for the two warnings that exist; the new warnings take it when they are built. Errors have no id.
* **Why not C:** warnings are raised from central config and the bundle, which do not read a repo's `gero.yaml`, so an acknowledgement kept there would be invisible to them; and a `refs` to a document that later moved into a repo would dangle, where an envelope field moves with its document. A separate kind can be added later beside B, for a document someone else owns.

## How to settle it

1. List the questions the tools must answer; keep only fields some question needs (done, in the fixture)
2. Write the real configs of the projects in the new shape, plus non-Rancher examples (done, in the fixture)
3. Walk each existing command through the new kinds and check it still finds what it needs (done: all do)
4. Prototype the readiness walk on a small in-memory graph before the loader
5. Generate the JSON schema last, from the Go types
