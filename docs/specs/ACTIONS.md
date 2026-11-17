# Actions

**Status:** implemented.

Composite GitHub Actions in `actions/` that call `gero`. They replace the actions of the same job that ORBS repos copy around (a script plus an `action.yaml`), and keep those actions' **inputs and outputs**, which is the contract a workflow depends on. The logic lives in `gero`; an action only passes inputs in and appends the command's `KEY=value` lines to `$GITHUB_OUTPUT`.

## Rules

* **`gero` is on the `PATH`.** An action does not install it. The calling job provides the binary.
* **Same contract, better names.** An action keeps the input names, output names and output case of the script it replaces. Only the action's own name changes, so a repo migrates by changing the `uses:` path.
* **Thin.** One step runs one command. Nothing the command decides is repeated in YAML.
* **Inputs reach the shell through `env`,** never interpolated into the script, so a tag or branch name cannot inject a command.
* **Versioning.** Actions have the CLI's version and tags ([IMPLEMENTATION](../IMPLEMENTATION.md#versioning)). A workflow pins `rancher/orbs-automation/actions/<name>@<tag or SHA>`, and the `gero` it provides should be the same version.

## The actions

| Action | Command | Replaces | Inputs | Outputs |
|---|---|---|---|---|
| `classify-tag` | `gero semver check` ([CHECK_SEMVER](CHECK_SEMVER.md)) | `check-semver` | `tag`, default `github.ref_name` | `HAS_PRERELEASE`, `HAS_BUILD_META` |
| `build-tags` | `gero repo build-tags` ([BRANCH_TAGS](BRANCH_TAGS.md)) | `compute-branch-tags` | none | `branch_tag`, `branch_static_tag`, `prev_static_tag` |

`build-tags` reads the previous tag from the checkout, so the job must fetch tags (`fetch-depth: 0`, or `fetch-tags`). Without them `prev_static_tag` is empty.

## Test plan

The `actions` job of the CI workflow builds `gero`, puts it on the `PATH`, and runs each action from the checkout (`uses: ./actions/<name>`). `classify-tag` runs against a prerelease tag with build metadata and a stable tag, and the job asserts the outputs. `build-tags` runs against the run's real ref, because the runner sets `GITHUB_REF` and `GITHUB_SHA` inside a composite action and a step's `env` cannot override them: on a push to `main` it asserts the outputs, and on any other ref it asserts the action fails. The commands' behavior is tested in their own specs; this job tests the wiring.
