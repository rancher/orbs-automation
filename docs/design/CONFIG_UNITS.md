# Config Units

**Status:** decisions, built in `gero/v1alpha2`. Why the config is split into small kinds, and the choices behind them. [CONFIG_SCHEMA](CONFIG_SCHEMA.md) is the field reference, [CHAIN_MODEL](CHAIN_MODEL.md) is the graph built from the kinds, and the [fixture](fixture/README.md) writes more projects in this shape. Breaking changes are acceptable while the version is `gero/v1alpha2`.

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
* A `gomod` Artifact is a Go module. Its `name` is the module path, and a release exists when its tag is on the repo and the version is on the public Go module proxy. The proxy is not configurable; a later field can name another. The kind is named for the module type, not `module`, so another language's modules are a new kind
* A Holding with `format: gomod` writes the `require` line for a module path; what else a Go bump needs (`go mod tidy`) is an Edit with the named strategy `goBump`
* A tag may carry a path prefix (`helmtools/v0.1.2`, as Go requires for a module in a subdirectory). `{tag}` is the tag as published; `{prefix}`, `{bareTag}` and `{version}` split it ([CONFIG_SCHEMA](CONFIG_SCHEMA.md#templates-formats-and-paths))
* A module's major version (`/v3`) is part of its path and not of its tag, so each major is its own Stream (`tagPattern: "v3.*"`) and its own `gomod` Artifact. Moving a consumer to the next major is a new path, so a new Holding, not a bump
* A line of a consumer stays on one minor of a library and takes only its patches, so each minor is its own Stream (`tagPattern: "v3.8.*"`, `universal`) and its own Artifact, even when the module path is the same. Which line holds which minor is a fact about the consumer (BRO and Rancher hold different minors for one line), so it is written on the consumer's Holdings with `lines`, not on the Stream. The library's own Kubernetes ranges are not in the config
* **A Holding's locations agree.** The locations of one Holding should hold the same version for a line; a difference is a warning (a repo's nested modules out of step). `replace` directives in a `go.mod` are not modelled
* **Line consistency is derived from the layout.** A Project with `branches.layout: perLine` is compared with the `finalDestination` (Rancher): for a module or image both hold, the stream it holds for a line should be the stream Rancher holds for that line, and a difference is a warning. A holder with `layout: single` has no lines of its own and is not compared. It needs no field: it follows from the layouts and the Holdings
* **Commits are the engine's, one for each step.** Shipping commits after the Holding writes and after each Edit step or strategy phase, and skips a commit when a step changed nothing. There is no setting for it; a Holding, Edit or strategy does not decide where a commit falls
* The second stage is `stable`; a scheme supplies a project's default stages, and a project writes `stages:` only to override
* A Holding is one document with two blocks: `at` is where to look, and `holds` is what it holds of a producer and how the text is written. Every location has exactly one such tie. Split the kind if a case needs one location tied to two producers. The reverse, one producer held at several locations, is a list: `at` is a location or a list of locations, each with its own `file`, `key`, `format` and optional `lines`, all tied to the same artifact and `holds`
* The `form` is a template, and a key is rendered from its template first, then read as a path in the file's own style: for `yaml` that is the path syntax `yq` uses (`.global.kubectl.tag`, `.chart-versions."2.16"`, `.additionalCharts[0].url`). A key in a YAML flow mapping needs quoting when it contains `[`
* The schema lists only the formats a project needs now and documents the others, so adding one is a new row, not a new idea. Every format must let a tool read the text at a key and replace it, leaving the rest of the file unchanged, comments included
* Templates take modifiers (`{line|trimprefix:v}`, `{stage|replace:stable=prod}`) from a small closed set, so a respelled value needs no map of its own

## The kinds

| Kind | Does | `refs` |
|---|---|---|
| `Project` | the repo as a place, and its branch layout | none |
| `Stream` | what a project releases: the versions and the lines | `project` |
| `Artifact` | one thing each release of a stream produces | `stream` |
| `Holding` | a named rule: where a piece of information lives in a repo, and what it holds | `project` (whose repo), and `artifact` (a release) or `from` (a project whose commit or branch it holds) |
| `Edit` | extra steps when shipping, beyond writing the form | `holding` |
| `RancherLines` | public line facts | none |

The fields of each are in [CONFIG_SCHEMA](CONFIG_SCHEMA.md).

* A project can have several streams (charts: one for each package), and a stream has one artifact for each thing a release produces (BRO: an image and two chart packages; shell: an image)
* An image artifact is checked in the registries its release kind goes to, a file artifact on the GitHub release, and a `gomod` artifact on the repo's tag and the Go module proxy; a published release missing a declared artifact is drift
* Holdings are declared by the consumer, which knows its own files; "who consumes X" is derived
* A holding can name an artifact or project with no configured project, so a component we do not trace can still be located and classified
* Read-only first: readiness, consistency, status and resolve need only Streams, Artifacts and Holdings
* A third-party repo is a Project with `repo.external: true`, with no kind of its own

## Provisional choices

None of these is forced by the shell and SCC to Rancher baseline, which the shape expresses field for field. Each takes its lean until the charts bump is built, then is revisited.

### O4. What is the shape of an Edit?

* **Context:** writing a holding is automatic when gero ships: render the form, put it at the location. What is left for an Edit is a rule or a step.
* **Evidence:** charts' bump carries a strategy and a package (`chartsPackageBump`). Rancher's regeneration after any change to `build.yaml` is a step that belongs to the consumer's file, not to one holding (O11).
* **Option A, steps only.** **Option B, steps and a named strategy** for a rule that needs code. **Option C, no Edit kind:** steps and strategy on the holding.
* **Lean B.** No multi-edit shape is needed.
* **Second case: `goBump`.** A Go module consumer needs more than the written line: `go mod tidy` follows the `require` change. It is an Edit with `strategy: goBump`, over a Holding with `format: gomod`, and takes no values of its own. Commits are the engine's, so the `require` change and the tidy are two commits with nothing in the strategy about it ([`examples/gomod.yaml`](fixture/examples/gomod.yaml)). It is given the clone's directory, the step runner (so a program runs as a step's does, by name from an allow-list) and what its edit's holdings wrote, runs `go mod tidy` once in the directory of each `go.mod` written, and reports a change only for a module whose `go.mod` or `go.sum` tidy changed. A `goBump` Edit needs a holding with a `gomod` location.

### O5. Do lines and series live in the Stream or in their own kind?

* **Context:** a project's branch for each line, and a stream's version series for each line.
* **Evidence:** they are different things. A repo has one branch layout for every line, while a repo can have many streams, each with its own series (about twenty charts, each `111.x` for v2.16).
* **Option A, both in the Stream.** Repeats the branch layout for every stream of a repo. **Option B, branches on the Project and series on the Stream.** **Option C, a `LineMap` kind.**
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
* **Option A, `refs.artifact` for a release and `refs.from` (a project) for a commit or a branch.** **Option B, always a stream,** with a qualifier. **Option C, commits and branches also become artifacts.**
* **Lean A.**

### O11. Where do the consumer's steps attach?

* **Context:** after any change to `build.yaml`, Rancher runs a regeneration step, whichever holding changed it.
* **Option A, an Edit that refs every holding it follows.** Repeats the list when a file gains a holding. **Option B, an Edit that refs the project and names files.** One place for a file. **Option C, steps in the holding's `at` block,** repeated for each holding of one file.
* **Lean A.** B adds a concept (named files) the baseline does not need; A works and can change when the repetition hurts.

### O12. How do two packages say their versions go together?

* **Context:** lasso `v0.2.*` goes with any wrangler `v3.*.*`. That is a fact about two producers' releases and matters to every project holding both (BRO, Rancher), not to how one flows into the other. No kind says it: a Holding is a consumer's file, a Stream is one producer's releases, and a Policy is a project's practice.
* **Option A, nothing yet:** every pair held is inside the one range that exists, so a check would never fail. **Option B, a `Pairing` kind** that refs two or more `gomod` Artifacts and selects the releases of each that go together (`series`); a project holding both must hold a declared pair, and a failure is a warning. **Option C, derive it** from each module's own `go.mod`, which already holds the hard minimums.
* **Lean A now, B** when a case needs it: lasso `v0.3` or higher, or wrangler `v4` or higher. Until one exists the shape is a guess, and the cross-version matrix is what such a case would show. Go's own resolution keeps enforcing minimums meanwhile.

### O13. How is a warning acknowledged?

* **Context:** a deliberate difference between a project and Rancher, such as one line on another minor of a library, raises a warning on real data from the first run, so it needs a way to be said. A finding has a severity and the path of the document it names, and a stable id for its rule.
* **Option A, none:** warnings stay and are counted by `gero doctor`. **Option B, an acknowledgement on the document the finding names:** `acknowledge: [{rule, reason}]`, with the reason required, and an acknowledgement that no longer matches a finding is itself a warning. **Option C, a separate `Acknowledgement` kind,** in one place. **Option D, a user file:** rejected, as it would differ by machine.
* **Decided B, in the envelope** ([CONFIG_SCHEMA](CONFIG_SCHEMA.md#acknowledging-a-warning)): `rule` and `reason` required, `lines` optional, no date. Silent by default and shown with its reason under `-v`. Errors have no id.
* **Why not C:** warnings are raised from central config and the bundle, which do not read a repo's `gero.yaml`, so an acknowledgement kept there would be invisible to them; and a `refs` to a document that later moved into a repo would dangle, where an envelope field moves with its document. A separate kind can be added later beside B, for a document someone else owns.
