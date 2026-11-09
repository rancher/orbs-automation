# Config Schema

**Status:** field reference, built in `internal/apis/v1alpha2`, 2026-11-08, except the Go module additions: the loader reads the `gomod` Artifact kind and format, `at` as a location or a list with `lines`, and `{prefix}` and `{bareTag}` in a form, and a release gives `{version}` without a tag's prefix; `gero ship` and the checks read and write every location of a Holding that exists for the line, in the `yaml` and `gomod` formats, and `goBump` runs `go mod tidy` in each `go.mod` its holdings changed; the commit of a strategy and the check of a `gomod` Artifact on the Go proxy are built; not yet built: `{version}` in a PR text and in a step's environment, and `locations-differ`. Each kind in [CONFIG_UNITS](CONFIG_UNITS.md) with its fields, types, defaults and rules. Items marked **(provisional)** take the lean of an open question and are revisited when the charts bump is built. The shapes are written in [`fixture/`](fixture/README.md). Design only: nothing here says how it is built.

## Every document

| Field | Is | Rules |
|---|---|---|
| `apiVersion` | the schema version | `gero/v1alpha2`; the old shape is `gero/v1alpha1`, and a document says which it is |
| `kind` | what the document is | one of the kinds below |
| `name` | its identity | a lowercase slug (`a-z`, `0-9`, `-`); unique within its kind; used in logs, errors and `refs` |
| `refs` | references to other documents | names only, a name or a list of names; the key says the target kind; every name must exist |
| `acknowledge` | warnings of this document that are expected | a list of `{rule, reason, lines}`; see [Warnings](#warnings) |
| `spec` | the document's own fields | strict: an unknown field is an error |

Nothing depends on which file a document is in or its place in the file. Several documents may share a file.

## Project

The repo as a place. `refs`: none.

| Field | Type | Default | Rules |
|---|---|---|---|
| `repo.name` | `owner/name` | required | no space; unique across Projects |
| `repo.owned` | bool | `false` | gero ships from the repo; a holding of an unowned producer is only observed |
| `repo.external` | bool | `false` | the repo is outside our organization: gero never ships into it and expects no `managedBy`; implies `owned: false`, so setting both is an error |
| `repo.managedBy` | slug | `orbs` if owned, else unspecified | `shared` is valid |
| `repo.defaultBranch` | string | `main` | |
| `finalDestination` | bool | `false` | at most one Project |
| `pullRequests.title`, `.branchPrefix` | template | none | variables as today: `project`, `source`, `sourceRepo`, `tag`, `version`, `branch`, `targetBranch`, `consumer`, … |
| `pullRequests.labels`, `.botLabels` | list | none | |
| `pullRequests.pushTo` | `fork` or `upstream` | none | |
| `branches.layout` | `single` or `perLine` | `single` | `single`: every release is on the default branch, and `lines` is not given |
| `branches.stages` | list of names | none | for a layout whose stages are branches (charts: `dev`, `stable`) |
| `branches.lines[].line` | a line id | required | must be in `RancherLines`; listed once |
| `branches.lines[].branch` | see below | omitted | |

`branch` has four forms:

| Written | Means |
|---|---|
| omitted | the default branch, for the line in development; after that a warning |
| `""` | the line has no branch yet; reported as unresolved |
| a string | that branch |
| a map of stage to string | a branch for each stage; the keys must be in `branches.stages` |

At most one line of a Project resolves to the default branch.

## Stream

What a project releases: the versions and the lines. `refs`: `project` (required).

| Field | Type | Default | Rules |
|---|---|---|---|
| `scheme` | `semver`, `calverLineStage` or `chartVersion` | `semver` | decides how releases are listed, ordered, classified as stable or prerelease, and mapped to a line |
| `tagPattern` | glob | `v*` | which tags are releases |
| `tagExclude` | list of globs | none | tags that match are not releases (`*-hotfix*`) |
| `stages` | list of `{name, prerelease}` | from the scheme | override only; `dev` is a prerelease |
| `lines[].line` | a line id | | must be in `RancherLines` |
| `lines[].series` | text | | `9.x`, `0.9.x`, `111.x`; the grammar belongs to the scheme; series must not overlap within a stream; none for `calverLineStage`, whose tag carries the line |
| `universal` | bool | `false` | any release serves every line; then `lines` is not given; a stream gives its `lines` or is universal |
| `requiresPrerelease` | bool | `false` | a stable follows an RC of its version; a release may override it, explicitly and recorded |

Scheme defaults for stages: `calverLineStage` gives `dev` (prerelease) and `stable`; `semver` gives none; `chartVersion` takes the stages of the project's branch layout.

## Artifact

One thing each release of a stream produces. `refs`: `stream` (required).

| Field | Type | Default | Rules |
|---|---|---|---|
| `kind` | `image`, `file` or `gomod` | required | |
| `name` | text | required | an image: its repository path. A file: a name template; variables `tag`, `version`. A `gomod`: the module path, no space |
| `lines` | `{since, until}` | every line | the lines that produce it |
| `registries` | list | by release kind | an image only; `dockerhub`, `prime-staging`, `prime-production`; the default is Docker Hub and Prime staging always, Prime production for a stable |

An image is checked in its registries; a file on the GitHub release **(provisional: exists only)**. A `gomod` exists when its tag is on the repo and the version is listed by the public Go module proxy, which `gero repo images exist` and `gero ship` ask over HTTP ([IMAGES_EXIST](../specs/IMAGES_EXIST.md#go-modules)) **(provisional: the proxy is not configurable; a field can name another later)**.

## Holding

A named rule for where a piece of information lives in a repo, and what it holds of a producer.

`refs`:

| Key | Target | Rules |
|---|---|---|
| `project` | Project | required: the repo whose file it is |
| `artifact` | Artifact | for `holds.kind: release` |
| `from` | Project | for `holds.kind: commit` or `branch`: the project whose commit or branch it holds **(provisional)** |

Exactly one of `artifact` and `from`, matching `holds.kind`.

`spec.at`: where to look. One location, or a list of locations for one piece of a producer held in several places (Rancher's nested `go.mod` files, or one tag in two files). Every location is tied to the same artifact and `holds` rule.

| Field | Type | Default | Rules |
|---|---|---|---|
| `file` | template | required | variables `line`, `stage`, `branch` |
| `key` | template | required | rendered first, then read in the format's own style (below) |
| `lines` | `{since, until}` | every line of the holding | a location only: the lines the file exists for. A location's lines and `holds.lines` overlap; where they do not, the location is not read. The two ranges work on different layers (`holds.lines` is the lines the tie covers; a location's `lines` is where a file exists), and validation warns where they leave a line with no location |
| `format` | name | `yaml` | `yaml`, `dockerfile` or `gomod` now; others documented below. `gomod` needs a `release` holding of a `gomod` Artifact **(provisional)** |

`spec.holds`: what it holds.

| Field | Type | Default | Rules |
|---|---|---|---|
| `kind` | `release`, `commit` or `branch` | required | |
| `form` | template | `{tag}` | for `release`; how the text is written; variables `image`, `tag`, `prefix`, `bareTag`, `version`, `url`, `file`, `sourceRepo`, `commit`, `base`, `upstream`; only modifiers that can be undone |
| `lines` | `{since, until}` | every line the holder and the producer both list | |
| `stage` | stage name | none | the stage branch of the holder where the holding is found |
| `stages` | list of stage names | none | for `commit` or `branch`: which stages of the producer are read; the key uses `{stage}` |
| `accept` | `stable` or `all` | `stable` | which releases the holding takes; an RC to a `stable` holding needs an explicit request |

## Edit **(provisional shape)**

Extra steps when shipping, beyond writing the form. `refs`: `holding`, a list (required).

| Field | Type | Rules |
|---|---|---|
| `preEdit`, `postEdit` | list of steps | a step is `run` (a program, or a list of arguments), `since`, `until`, `from` |
| `strategy` | name | a named rule that needs code (`chartsPackageBump`, `goBump`); `goBump` needs a holding with a `gomod` location |
| `with` | map | the strategy's own values, declared by the strategy; `goBump` declares none |

Commits are the engine's, not a field: shipping commits after the Holding writes and after each step or strategy phase, and skips a commit when a step changed nothing. `goBump` over a `gomod` Holding is two commits, the `require` change and `go mod tidy`.

## Templates, formats and paths

* A template is text with `{variable}` parts and optional modifiers after `|`: `upper`, `lower`, `title`, `trimprefix:P`, `trimsuffix:S`, `replace:FROM=TO` (whole value). An unknown variable or modifier is an error.
* Variables in `at`: `line` (`v2.16`), `stage`, `branch`. Variables in `form` and an artifact's `name` are listed in the tables above and in [CONFIG_UNITS](CONFIG_UNITS.md#templates).
* A format reads the text at a key and replaces it, leaving the rest of the file unchanged, comments included.

| Format | `key` is | Listed |
|---|---|---|
| `yaml` | a path in `yq` syntax: keys, quoted keys for a dot inside one, `[n]` for a list item; a leading dot is optional | now |
| `dockerfile` | an `ARG` or `ENV` name | now |
| `json`, `toml` | the same path syntax as `yaml` | documented |
| `env` | a variable name in a `KEY=value` file | documented |
| `makefile` | a variable name in `VAR = value` or `VAR ?= value` | documented |
| `gomod` | a module path: the version on its `require` line in `go.mod` | now |
| `regex` | a pattern with one capture group | documented |

## Unchanged kinds

There is no `External` kind: a third-party repo is a Project with `repo.external: true`.

* **`RancherLines`:** a line's id, dates, Kubernetes range and optional phase override ([config/README](../../config/README.md))
* **`WorkflowsConfig`:** rendering of workflows into a repo, kept in the repo

## Rules across documents

* Names are unique within a kind; every `refs` name exists and is of the kind its key says
* Repo slugs are unique; at most one `finalDestination`; at most one line of a Project resolves to the default branch
* Every line named is in `RancherLines`; a stream's series do not overlap
* A holding's `from` or `artifact` matches its `holds.kind`; its templates use only the variables and modifiers allowed where they are written; its `format` is listed
* A selector's `since` is not after its `until`
* A project that holds something of another, which holds something of it, is a warning: the dependency is circular
* `repo.external` and `repo.owned` are not both true
* Repo slugs are unique whatever their case; an image name is unique within a project; there is one `RancherLines` document
* A calver stream is on a project whose layout is `single`; a project holds nothing of itself
* A holding's `stage` is a stage of its holder's `branches.stages`, and its `stages` are stages of the producer's streams
* The locations of one Holding should hold the same version for a line; a difference is a warning (a repo's nested modules out of step). A project's locations at the same file and key, in one Holding or several, do not overlap in lines; a `gomod` name is unique within a stream, not a project (the same module is one Artifact for each minor)
* A project with `layout: perLine` that holds an artifact (matched by its image name or module path) which the `finalDestination` also holds (a project is not compared with itself) should hold the same stream for each line; a difference is a warning. A `single` layout is not compared
* A `gomod` Artifact's `name` is a module path; a Holding with `format: gomod` holds a `gomod` Artifact; a `goBump` Edit has at least one such Holding
* A step run `from: producer` needs a producer that is owned
* A lifecycle date raises a warning, never an error ([CONSTRAINTS](CONSTRAINTS.md))

## Warnings

A warning never makes a document invalid. It names its rule by a stable id, shown at the end of the finding in brackets (`... is circular [circular-holding]`); an id is never renamed, because a document will name it to acknowledge the warning ([O13](CONFIG_UNITS.md#o13-how-is-a-warning-acknowledged)).

| Rule | Raised when | Built |
|---|---|---|
| `circular-holding` | a project holds something of another, which holds something of it | yes |
| `default-branch-past-development` | a line with no `branch` is past development, so it is read from the default branch | yes |
| `unknown-key` | a user file has a key it does not know (the user file is not a document, so it cannot be acknowledged) | yes |
| `locations-differ` | the locations of one Holding hold different versions for a line: it reads the files of a repo at a ref, only when a command asks (not when the graph is built), so the commands that read repos raise it, not `config validate`; the read of every location is built, the finding is not | no |
| `stream-differs-from-destination` | a project with `layout: perLine` holds another stream of an artifact (matched by its kind and name) than the `finalDestination` for a line; on the holder's Holding, naming the line | yes |
| `acknowledgement-unused` | an acknowledgement matches no warning of its document | yes |
| `lifecycle-date` | a release is shipped to a line past a lifecycle date | no |

The names of the rules not built are provisional until they are.

### Acknowledging a warning

A document says a warning of its own is expected with an `acknowledge` list in its envelope, on any kind:

```yaml
kind: Holding
name: bro-holds-wrangler-3-7
acknowledge:
  - rule: stream-differs-from-destination
    lines: {since: v2.16, until: v2.16}
    reason: BRO main stays on wrangler 3.7 until the v12 release
```

| Field | Rules |
|---|---|
| `rule` | required; the id of a rule of a document (`circular-holding`, `default-branch-past-development`, `stream-differs-from-destination`); the unknown key of a user file and `acknowledgement-unused` cannot be acknowledged |
| `reason` | required, and not only space: it is what a reader of the config is told |
| `lines` | `{since, until}`, optional; limits it to warnings about those lines, ends included. With no `lines` it covers every warning of the rule on the document, whether it is about a line or not; with `lines` it covers only a warning about a line in the range |

* An acknowledgement matches a warning of the same document and rule. One on another document does not
* An acknowledged warning is not a warning: it is not counted by `gero doctor` and not shown by `gero config validate`, which shows it with its reason under `-v`
* An acknowledgement that matches no warning is itself a warning, `acknowledgement-unused`
* It lives in the central config, because warnings are raised from it and from the bundle; a repo's own `gero.yaml` is not read for it

## Where today's fields went

[CONFIG_UNITS](CONFIG_UNITS.md#where-todays-fields-go) has the mapping. `release.branching: perLineTag` is no longer a layout: a project whose lines are only in its tags is `branches.layout: single` with a Stream whose scheme gives the line.
