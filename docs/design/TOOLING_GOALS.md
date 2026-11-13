# Tooling Goals

**Status:** staging. Goals the tooling should meet, found while writing the [process docs](../process/README.md). They are kept here so the process docs stay about the process. They feed the config redesign first, then the Go design.

## Goal: a way to init `gero` user config

Answer, add a new `gero init` as an onboarding step to create the user config via user input/feedback.

### What is needed as user input?

We can start with minimal basic fields that can be helpful and expand init later as it's requested.
The primary user config fields that are worth prompting for are:

- `githubUser` - To identify their forks of repos
- `fork` & `upstream` - To identify the remote repo names for projects under their fork and Rancher upstream respectively
- `branchPrefix` - A branch prefix - support minimal vars - to use when creating PRs via the tooling. It is a lead-in: `gero` appends a fixed key after it so a bump's branches share a tail ([PUSH_TO_REPO](../specs/PUSH_TO_REPO.md#branches-and-pr-identity))
- `centralConfigDir` - The directory path to the ORBS automation repo's config dir. It matters less than the others: with none set, `gero` reads the config bundle of a release ([CENTRAL_BUNDLE](../specs/CENTRAL_BUNDLE.md))
- `repos.search` - Would be useful to have set
- `repos.cloneRoot` - Will be needed for repo clone tooling; if possible we prompt to set this (single dir value) to the first input search path?

**Built:** `gero init` asks for `githubUser`, `fork`, `upstream`, `branchPrefix`, `centralConfigDir`, `repos.search` and `repos.cloneRoot`, or takes them as flags, and refuses an existing file unless `--force` ([CLI_FRAMEWORK](../specs/CLI_FRAMEWORK.md)). An empty answer skips a question and leaves its key out, so there are no defaults, and `repos.cloneRoot` is not offered the first search path: the two are different keys ([CLI_CONFIG](../specs/CLI_CONFIG.md)), and the prompt says so.

## Result
After accepting user inputs it would save to relevant field(s) in file and then subsequent `gero` commands are more configured.
The file it writes starts with the `yaml-language-server` comment that points at the user file schema on the `main` branch of the repository, so an editor validates and completes the file from the first edit ([APP_CONFIG](../specs/APP_CONFIG.md#json-schemas)):

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/rancher/orbs-automation/main/generated/jsonschema/user-config.schema.json
```
On a fresh workstation it enables `gero repo clone [project...] [--all]`, which clones the named projects, or every project the active team owns and that is not external, under `repos.cloneRoot` or `--into`.

## Goal: a repo clone tool to fetch team projects

Answer, use the same configs collected by `gero init` to allow easier workstation project setup.

## Goal: stable readiness

Answer, for one line: would a stable Rancher build be valid with what Rancher holds today?

### Verdict

* **Ready:** every traced component Rancher holds is stable
* **Blocked:** a traced component is held as a prerelease; each blocker is listed with its path from Rancher
* **Unknown:** something could not be read

### Whole chain

* A line's chain is stable when every trackable component in it is stable
* A CLI tool result shows each traced part as stable or not, so the blockers are visible at a glance

### Rules

* A blocker is a held version that is a prerelease under its project's scheme
* A `dev` stage is a prerelease: an Assets `dev` tag blocks a stable
* A chart version holds its upstream version after `+up`, and the stable test reads that part (the `chartVersion` scheme)
* Reporting what Rancher holds is separate from readiness: holding a `dev` Assets tag is not an error, but it is not stable
* Only configured projects and traceable edges count; the answer says how many are traced and never claims readiness beyond them
* Unknown is never reported as ready
* The answer is necessary, not sufficient: QA sign-off and the release team's decision are outside it for now
* A CVE stable skips the RC; readiness only looks at what is held, so that is not an exception

### Kinds of blocker

* **Needs a bump:** a stable release of the component exists, but Rancher holds an older prerelease
* **Needs a release:** the component's own latest release is still a prerelease; its team un-RCs first

### What it needs from config

* What counts as a prerelease for each project, including which stages
* For each edge, where the held value is recorded and how to read a project and version from it
* The line each hop belongs to, so the check runs for one line at a time

### What it needs from the code

* One small interface for "is this version stable", with an implementation for each scheme (semver, calendar and stage, chart build metadata); the walk does not know which scheme it is using
* A walk that reads what each hop holds, never writes, and is safe on cycles (or add to existing walkers)

### Open

* A chart version holds its upstream version after `+up`; the stable test reads that part
* A component with no tracing config: how a view names it as not traced. It is settled while the view is built

## Goal: line consistency

Answer, for one line: does each hop hold a version of its own line?

* A project's version series follows the line (kuberlr's major, shell's minor, BRO's major, a chart's major), so the line can be read back from a held version
* A hop that holds a version of another line is a mismatch; today a branch can hold a dependency from the wrong line and nobody sees it
* Needs from config: how each project maps a version to a line; for kuberlr, shell, BRO and charts that is a function of the line, so it may be data, not a per-project rule
* The verdict is a list of mismatches with the path to each, never a block on its own

## Goal: say when a consumer holds less than a pin requires

A release that pins another says which version of it goes with the release ([CONFIG_UNITS](CONFIG_UNITS.md#o12-how-do-two-packages-say-their-versions-go-together)): wrangler's `go.mod` requires a lasso. `gero repo status` should report a consumer whose pinned module is older than that, for the pinning release it holds, as "requires at least vX (wrangler vY)", and not compare it with the latest release of the module, which is what a virtual Holding would otherwise be read against. `gero lines resolve` should show the pin as an edge between the two projects.

`gero ship` of the pinned Artifact alone works, as a floor. It should warn when the tag is below what the consumer's pinning release requires, which needs the go.mod of the wrangler the consumer holds.
