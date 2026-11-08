# Config Shape Review

**Status:** second pass, 2026-11-07. Compares the current `gero/v1alpha1` config ([`internal/config`](../../internal/config/config.go), [`config/`](../../config/README.md), [APP_CONFIG](../specs/APP_CONFIG.md)) with the [process docs](../process/README.md), the [tooling goals](TOOLING_GOALS.md), the [constraints](CONSTRAINTS.md) and [VISION](../VISION.md). It finds gaps and proposes directions; nothing here is decided. Where sources disagree, the real `config/` directory wins, then the code, then the specs, then `config-mock/`. Sketches are illustrations, not final field names.

## What fits

* `repo.owned` and `repo.managedBy`: `shared` is a valid slug, so Rancher's "owned by all teams" needs no new field (the real Rancher config sets it)
* `release.branching` (`single`, `perLine`, `perLineTag`) covers all seven projects
* `release.lines` with a branch for each line, and the default branch for the line in development
* The consumer owns the method, the producer owns the edge: matches every hand-off found
* `finalDestination`, `RancherLines` and a derived phase
* `images[].since`, steps with `since` and `until`
* A default registry rule by release kind, with an optional per-image override ([IMAGES_EXIST](../specs/IMAGES_EXIST.md))
* Strict decoding

## What the real configs cannot say yet

The real `config/` directory has files that do not validate or hit limits, and they show where the shape runs out. `gero config validate` reports one error (the unbuilt `chartsPackageBump`), and the files carry TODOs and unsupported syntax for the rest.

| Where | What the config tries to say | What stops it |
|---|---|---|
| `backup-restore-operator`, edge to charts | one release updates the chart's `url` and also the CRD chart's `additionalCharts` URL (a TODO in the file) | an edit is one key and one value; an edge cannot make two (gap 12) |
| `backup-restore-operator`, edge value | `{repo}` and `{tag (without V)}` | the value template has `image`, `tag`, `version`, `sourceRepo`; `{sourceRepo}` and `{version}` are the real names (gap 13) |
| `rancher-charts`, method | `charts-package-bump`, "maybe needs to be per package?" and "sort out the layers of `with` and params" | the package file path depends on the package, and `with.file` is a fixed string; `chartsPackageBump` is also unbuilt, the first validation error (gap 13) |
| `rancher-charts`, method | RCs and stables both flow in from BRO | the method lost its `accept: when: all`, so it takes only stables (gap 5) |
| `rancher-charts`, `lines` | "how to define the dev versus prod branches for each line" (a TODO) | one string for each line's branch (gap 1) |
| `rancher-charts`, edge to Assets | `updateKey: ".chart-versions[{line}].upstream-refs[{stage}].charts.commit"`, so one edge reads the right key for each line and stage | params have no variables; the key path reader splits on `.` only (no leading dot, no brackets, no dotted key such as `2.16`); `{line}` would be `v2.16` where the file says `2.16`; the file says `prod` where we say `stable`; the value is a commit (gaps 4 and 14) |

## Gaps

### 1. Stages are not a project property

* **Process:** charts has `dev` and `stable` (two branches for each line); Assets has `dev` and `stable` (a tag suffix); the rest have none
* **Today:** `branch` is a string; a stage appears only as an edge's `stage`; tag schemes give stage names in code, `dev` and `stable`; the real charts file has one branch for each line and a TODO
* **Goals:** readiness needs, for each project, which stages are prereleases (a `dev` stage is one)
* **Direction:** a project lists its stages, in order, each with a prerelease flag; `branch` is a string or a map of stage to branch (as [CHAIN_TRACING](../specs/CHAIN_TRACING.md) drafts)

```yaml
release:
  stages:
    - name: dev
      prerelease: true
    - name: stable
  lines:
    - line: v2.16
      branch: {dev: dev-v2.16, stable: release-v2.16}
```

### 2. A release is not always a tag

* **Process:** charts releases are versions in a list on a branch, spelled `CHART+upUPSTREAM`; Assets uses calendar tags with a stage suffix; Rancher spells RCs `-rcN` and `-rcs…` and has `-hotfix` tags outside the release
* **Today:** `tagScheme` is `semver` or `calverLineStage`, and all releases are tags matched by `tagPattern`
* **Goals:** readiness and line consistency need, for any version a hop holds, its line, whether it is a prerelease, and its order
* **Direction:** a scheme answers four things (enumerate releases from a source, classify stable or prerelease, order, line of); add a chart-version scheme whose source is a file at a ref and whose prerelease test reads the part after `+up`; let the semver scheme read `-rcN` as an RC and order it numerically; let a project exclude tags (hotfix)

### 3. The version series is a function of the line, but is only free text

* **Process:** kuberlr's major, BRO's major, shell's minor and a chart's major each follow the line; SCC has none (universal); Assets has the line in the tag; Rancher's minor is the line
* **Today:** `release.lines[].version` is a string such as `9.x`, used to choose releases but not read back
* **Goals:** line consistency needs version to line, the inverse, to spot a hop holding another line's version (BRO on `main` holds kuberlr v8, v2.16 wants v9)
* **Direction:** keep the explicit table, since offsets differ (charts 111 is 2.16, kuberlr 9 is 2.16); give `version` a defined grammar per scheme, validate that series do not overlap within a project, and add the inverse lookup

### 4. What an edge holds, and how to read it

* **Process:** a hop holds an image tag (Rancher), a chart version (charts, Rancher), a commit pin (Assets' lock file), a branch name (Rancher's charts branch), or a value inside a chart's files (kuberlr); the repo's file is the source of truth for each
* **Today:** a consumer's `updateMethods` (`yamlBump`) read and write a file by key; the read half serves observed edges; the value is assumed to be a tag; one edge has one `updateKey`
* **Real config:** the charts-to-Assets edge now writes the key as a template over `{line}` and `{stage}`, so one edge stands for a read of each line and stage; the value is a commit
* **Key path today:** `yamledit` splits the path on `.`, so a leading dot, `[…]` and a key that holds a dot (`2.16`) do not work
* **Goals:** readiness and consistency read a hop, never write, and must know what the value is
* **Direction:** a method states the kind of value it reads (`tag`, `chartVersion`, `commit`, `branch`) so the walk knows how to classify it; a commit is compared with a branch head, not a scheme; key paths need a quoting rule for a dot inside a key (the line `2.16`); "update method" is a misleading name for a read-only use; the key for a lock file is layout the reader should know (Q9)

### 5. What the consumer accepts is not what the producer sends

* **Process:** Rancher holds component RCs and stables alike (shell, Assets either stage, BRO's RCs in charts); shell sends every release, SCC a stable on its own and an RC only on request, BRO every RC and stable, Assets either stage
* **Today:** `accept.when` is a consumer setting, `stable` by default; a consumer that wants both has two methods, and an RC to a stable-only method needs `--prerelease` ([APP_CONFIG](../specs/APP_CONFIG.md), [PUSH_TO_REPO](../specs/PUSH_TO_REPO.md)); the real Rancher method and the real shell and SCC edges are stable-only; the real charts method dropped the `accept: when: all` the mock had, so BRO's RCs would be refused
* **Vision:** the reconciler observes what GitHub holds and does not push; a send setting is the expectation it checks against
* **Direction:** the shape already says what a consumer takes and has an explicit RC request, so the first step is data: charts takes all, and Rancher takes all of what it holds; whether a producer also says what it sends (`stable`, `all`, or on request) is Q2

### 6. Reach of an edge and unreleased lines

* **Process:** SCC reaches every active line; shell one branch for each line; charts reach Assets from v2.16 and Rancher on every line; several unreleased lines may exist, but only one can sit on `main` and none resolves into Rancher until Rancher has its branch
* **Today:** `lines: all|universal`, derived; a line whose `branch` is omitted resolves to the default branch only while the line is in development, and is an error after; `since` and `until` exist on steps and images, not edges
* **Direction:** add `since` and `until` to an edge (already drafted); tell two cases apart: `branch` omitted (use the default branch, as today) and `branch` set to an empty value (the line has no branch yet); at most one line of a project may resolve to the default branch; a line with an empty branch, or with no branch in the consumer, is reported as unresolved
* **Note:** the two cases need a field that can tell omitted from empty; in Go that is a pointer to a string, and `branch: ""` is distinguishable while `branch: null` is not (it decodes to nil, the same as omitted)
* **Note:** the current error for an omitted branch after development reads the lifecycle date; under the [no release calendar state](CONSTRAINTS.md) constraint a lifecycle date gives a warning, so this becomes a warning or a structural check

### 7. One edge depends on another

* **Process:** a system chart goes to charts first, then its version goes into Rancher's build file; the version must be in the index before Rancher can hold it
* **Today:** nothing; each edge stands alone
* **Goals:** readiness should flag a held version that does not exist upstream
* **Direction:** an edge can name the edge it follows; the check is a validity rule on the held version, not an order of actions

### 8. Registries are a policy, not an image property

* **Process:** a prerelease or dev build goes to Docker Hub and Prime staging, for every line; a component's stable goes to Docker Hub and Prime production; the destination's stable goes to Prime for every line and to community only for the newest minor; the community rule is product policy and applies from a point forward, never retroactively
* **Today:** the default is by release kind in code (Docker Hub and Prime staging always, Prime production for a stable); `images[].registries` is an optional override
* **Vision:** release workflows are rendered into each repo, and every repo needs the same registry answer
* **Direction:** keep the kind-based default; what is missing is only Rancher's own newest-minor rule, which matters only if the tool checks Rancher's own images; add a shared policy with a rule that applies from a given line forward when it does

### 9. RC-first and QA

* **Process:** a stable follows an RC with QA sign-off, except a CVE or patch stable; kuberlr asks QA by exception; QA sign-off lives outside the tooling for now
* **Today:** `release.requiresPrerelease` (a bool) and an explicit per-release override
* **Direction:** keep the bool as the default; the per-release override is designed ([IMPLEMENTATION](../IMPLEMENTATION.md#release-rules)) but not built, so build it explicit and recorded; do not model QA in config (VISION v3 may record sign-off in the tracking issue)

### 10. Two things that belong elsewhere

* **The publish order** (CI, draft, image, assets, attestation, publish, post-steps) is the same for ORBS projects; it belongs in the shared workflow templates, not in a project's config
* **Per-chart ownership** (a chart's owning team) is CODEOWNERS data in charts; it is out of scope for config

### 11. The same fact is written in several places

* **Process:** a line's Kubernetes range sits in the lines data, and the same range is the `kubectl` list in kuberlr's and shell's images and the k3s versions BRO's workflows test; BRO's `main` already drifted (rendered Go version ahead of its config)
* **Today:** each is written by hand per branch or per repo
* **Direction:** derive repeated facts from the line where config reads them (a principle for the redesign), and write one only when it differs from the line

### 12. One edge makes several edits

* **Process:** BRO publishes `rancher-backup` and `rancher-backup-crd` chart packages; in charts one package file holds the chart's `url` and an `additionalCharts` entry with the CRD chart's `url`; one BRO release updates both, each to its own release asset
* **Today:** `yamlBump` makes one edit, one key and one value, for each call; `params` is a map of strings, so an edge cannot carry a list of edits
* **Real config:** the BRO edge has a TODO for the second URL
* **Direction:** a way for one method to make several edits to a file, each a key and a value template; where the edits live (the edge, the method, or code that knows the file) is Q7

### 13. The layers of `with` and params, and a method for many packages

* **Process:** charts has about twenty packages, each at `packages/<package>/package.yaml`, with the same bump (edit the file, a chart version rule, `release.yaml`, regenerate)
* **Today:** `with.file` is a fixed string, so one method edits one file; a producer's params fill the method's `allowedParams`; a strategy's own values (`chartVersion`, `action`) come from its code; the `value` template has `image`, `tag`, `version`, `sourceRepo`
* **Real config:** "maybe needs to be per package?" and "sort out the layers of `with` and params"; the BRO edge's `{repo}` and `{tag (without V)}` are not variables (`{sourceRepo}` and `{version}` are)
* **Direction:** name the layers: the strategy (code, knows a file layout), the method's `with` (what the consumer fixes), the edge's params (what the producer passes); decide how a package file path is reached (Q8); fix the BRO template to the real variables once the strategy exists

### 14. A line and a stage are spelled differently in each file

* **Process:** the second stage is `stable` in our docs, `prod` in Assets' config and lock files, a `release-vX` branch in charts, and no suffix in an Assets tag; a line is `v2.16` in our config and `2.16` in Assets' lock file
* **Today:** no variables carry a line or a stage into a key, and nothing maps one spelling to another
* **Real config:** the charts edge's `{line}` and `{stage}` need exactly that
* **Direction:** the variables give the line as `v2.16` and as `2.16`, and a stage as our name; a consumer's reader maps our stage name to its own spelling (`stable` to `prod`), so the mapping lives with the file it describes

## Coverage by project

| Project | Fits today | Needs |
|---|---|---|
| kuberlr-kubectl | all | version grammar (3) |
| shell | all | send setting (Q2); version grammar (3) |
| scc-operator | all | send setting (Q2) |
| backup-restore-operator | lines, the kuberlr method | `chartsPackageBump` strategy; several edits on one edge (12); real template variables (13); chart edge reach (6); version grammar (3) |
| rancher-assets | lines in tags | stage names and prerelease flag (1); commit value kind (4); `stable` to `prod` in its own files (14) |
| rancher-charts | lines | stages and branch map (1); chart-version scheme (2); layers and per-package (13); `accept` all (5); key per line and stage, with line and stage spellings (4, 14); since and until (6) |
| rancher | lines, finalDestination, `managedBy: shared` | accept all (5); component kinds (4) |

## Go seams this suggests

*Parked: the Go design follows once the config design lands.*

* **Scheme:** enumerate, classify, order, line of; implemented for semver (and loose RC spelling), calendar-and-stage, chart version; already the `tagscheme` shape
* **Holding reader:** reads one hop at a ref and returns a typed value (tag, chart version, commit, branch); read-only
* **Edit set:** a strategy makes one or more edits to a file from one release, and reports each
* **Policy:** registries by release kind and line; one lookup the code asks
* **Walk:** cycle-safe, asks the graph for edges and the reader for values; readiness and consistency are two views over it
* **Warnings:** validation returns errors and warnings; today it returns errors only. A lifecycle date or a drift finding is a warning, never an error, so a release exception stays possible ([CONSTRAINTS](CONSTRAINTS.md)); the CLI shows warnings and exits clean

## Questions

Each has the problem, the options with an example, and where I lean. Q1, Q7, Q8 and Q9 block rewriting the real charts, BRO and Assets files; Q2 and Q3 shrank; Q4 is small; Q5 can wait; Q6 is a principle that shapes the rest.

### Q1. Where do a project's stages come from?

* **Problem:** the readiness check must know that `dev` is not stable. Charts and Assets both have `dev` and `stable`, but they get them differently: charts from two branches, Assets from a tag suffix.
* **Option A, write them in the project:** as in gap 1. Explicit; the same two lines repeat in charts and Assets.
* **Option B, derive them from the scheme:** a scheme named `calverLineStage` implies `dev` and `stable`, and a chart scheme does the same. Less to write, but the stages are hidden in code, and charts still needs its branch names written.
* **Your answer:** you lean B, since every semver release already has implied rules, even if charts needs its own semver flavor, with a config escape hatch
* **Hybrid that fits:** the scheme supplies default stages (names, order, prerelease flags); each line's `branch` map names the branches and must use the scheme's stage names (checked at load); a project writes `stages:` only when the default does not fit
* **Traps to watch:** the stage names live in code, so a new stage name is a code change; a branch-based project (charts) still writes its branch names, so B removes only the stage list, not the map
* **Lean now:** B with the override

### Q2. Does the producer say what it sends?

* **Problem:** the consumer decides what it will take (`accept`); the producer decides what it pushes by itself. Today only the first exists.

  | Producer | Sends by itself |
  |---|---|
  | shell | every release, RCs too |
  | SCC | stables; an RC only when a person asks |
  | BRO | every RC and stable, into charts |
  | Assets | both stages |

* **Option A, add a small setting on the edge:** `send: stable | all | manual`, default `stable`. A status view can then tell "shell's RC is not in Rancher yet, which is unexpected" from "SCC's RC was never sent, by design".
* **Option B, no new field:** the consumer accepts all and what a producer's own automation sends stays outside the config. Simpler, but the view cannot tell those two cases apart.
* **Evidence:** VISION's reconciler is observational, so it needs an expectation to compare against; the shape already has an accept and an RC request, so this is now only about that status view
* **I lean A,** if you want the view. If not, B is fine.

### Q3. Where do the registry rules live?

* **Status:** mostly answered. The kind-based rule exists in code; a new document is needed only for rules the tool does not have (Rancher's own newest-minor rule).
* **Option A, a new shared document kind** (for example `ReleasePolicy`), one file. **Option B, a section of the lines document.** **Option C, keep it in each project's `images`.**
* **I lean to leave it** until the tool checks Rancher's own images, then A.

### Q4. Keep each line's version series as an explicit table?

* **Problem:** to check that a hop holds a version of its own line, we must go from a version back to its line. The offsets differ by project, so a formula would be fragile.

  | Project | v2.16 series |
  |---|---|
  | kuberlr-kubectl | `9.x` |
  | backup-restore-operator | `12.x` |
  | shell | `0.9.x` |
  | charts (major) | `111.x` |

* **Option A, keep the explicit table** (as today) and add a defined format for each scheme, a check that no two lines in a project overlap, and a lookup from version to line.
* **Option B, a rule per project** such as "major is line index plus four". Shorter, but it breaks as soon as a project skips a number.
* **I lean A.** This is mostly a yes or no.

### Q5. Does an edge say which edge it follows?

* **Problem:** a system chart goes to charts first, then its version goes into Rancher's build file, and the version must be in the index before Rancher can hold it.
* **Why it can wait:** webhook, fleet and turtles are not configured projects, so nothing we trace today has this dependency. BRO's chart is not a system chart in Rancher.
* **Option A, name it on the edge** (for example `after: rancher-charts`) when those projects are added. **Option B, check it from the chart scheme:** any chart version Rancher holds must exist in charts' branch.
* **I lean to wait.**

### Q6. Derive repeated facts from the line?

* **Problem:** gap 11. A line's Kubernetes range is written again in image lists and test matrices, and they drift.
* **Option A, derive from the line** where config reads them, write only differences. **Option B, keep writing each,** with a check that they agree (a warning).
* **I lean A,** with B as the first step where something cannot be derived yet.

### Q7. How does one edge make several edits?

* **Problem:** one BRO release must change two URLs in a charts package file, the chart's and the CRD chart's, each to a different release asset. The edit tool changes one key to one value.
* **Option A, a list of edits on the edge:**

  ```yaml
  downstream:
    - to: rancher-charts
      method: charts-package-bump
      params:
        package: rancher-backup
      edits:
        - key: url
          value: "https://github.com/{sourceRepo}/releases/download/{tag}/rancher-backup-{version}.tgz"
        - key: additionalCharts.0.upstreamOptions.url
          value: "https://github.com/{sourceRepo}/releases/download/{tag}/rancher-backup-crd-{version}.tgz"
  ```

  The producer writes the file's keys, which is the consumer's layout.
* **Option B, the consumer's method owns the layout:** charts' method knows a package file has `url` and `additionalCharts`; the producer passes only what is its own, the asset names.

  ```yaml
      params:
        package: rancher-backup
        assets: [rancher-backup-{version}.tgz, rancher-backup-crd-{version}.tgz]
  ```

* **Option C, two edges, one for each URL:** no new shape, but two PRs for one release unless they are merged, which is wrong.
* **I lean B.** The consumer owns the mechanics, the producer owns the wiring ([APP_CONFIG](../specs/APP_CONFIG.md)), and the file layout is the consumer's.

### Q8. The layers of `with` and params: one method for many packages?

* **Problem:** charts has about twenty packages, each at `packages/<package>/package.yaml`. `with.file` is a fixed string, so a method edits one file. The real file asks "per package?".
* **Layers today:** the strategy (code: knows a layout, a chart version rule), the method's `with` (the consumer fixes a file), the edge's params (the producer passes values)
* **Option A, a method for each package:** twenty near-identical methods; a new package means a new method.
* **Option B, one method and a templated path:** `with.file: packages/{package}/package.yaml`, filled from the edge's `package` param. Needs variables in `with`, which it does not have.
* **Option C, the strategy owns the path:** `chartsPackageBump` knows `packages/<package>/package.yaml`, so `with` is empty and the edge passes `package` (this matches Q7 B).
* **I lean C** for charts, since the layout is code-known like the chart version rule; B only if another strategy needs a path from a param.

### Q9. How does the charts-to-Assets edge say which key to read?

* **Problem:** what Assets holds of charts is a commit at `chart-versions."2.16".upstream-refs.dev.charts.commit` in its lock file, a different key for each line and stage. The real file now tries a template, `.chart-versions[{line}].upstream-refs[{stage}].charts.commit`, and its TODO says it needs the substitution.
* **Option A, a template over the line and stage** in the key, as the real file writes it. What it needs, none of which exists: variables in params; a path grammar with a leading dot, brackets and a quoted dotted key; `{line}` as `2.16` for this file; `{stage}` mapped to `prod` for the stable stage; an edge that stands for a read of each line and stage.
* **Option B, a read-only reader that knows the lock file's layout,** and the edge says only where the pin is for (charts, the stage). CHAIN_TRACING already describes it as "a small helper over the file reader and the key-path code, not a strategy".
* **Option C, one edge for each line and stage:** nothing new, and ten edges for one fact.
* **I lean B,** the same reasoning as Q7 and Q8: the consumer's file layout stays with the consumer's code. Option A is what the real file tried; its cost is the four additions above, and each one is also a place where the producer's config must know the consumer's spelling.

## Housekeeping

* **A term:** `RELEASE_PROCESS` says "chain of a line"; VISION says "release train". Pick one.
* **`gero init` and `gero repo clone`:** the open items (SSH or HTTPS, which remote is `origin`, how `--all` reads `owned`) are user-config questions, not part of these.
* **Real files:** the Assets method `read-lock-yaml` describes its key as in `build.yaml`, but it reads `lock.yaml`.
