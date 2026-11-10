# TODO audit (working file)

A scratch file for rebuilding `docs/TODO.md` from what the code and docs say. It is deleted when the new TODO lands.

**Process:** (1) bucket every open item by source; (2) verify each against current code and accurate specs; (3) the maintainer curates onto the milestones Before V1, After V1, Before V2, Moonshot.

**Verdicts:** `real` (true and unbuilt), `done` (built; the note is stale), `moved` (the need is real, the shape changed), `question` (a decision, not work), `duplicate of <id>`, `unverified` (not yet checked).

**Groups:** A config and CLI framework specs; B workspace, doctor, setup, update, repl, testing, layout specs; C releases, tags, ship and status specs; D chain, bundle, images, metadata specs; E design, process and implementation notes; F code `TODO(...)` markers; T the current `docs/TODO.md`, kept to cross-check and not as a source.



## A. Config and CLI framework (29)

### A01  docs/specs/APP_CONFIG.md:89  [Not built yet]

* A schema for `WorkflowsConfig`, which has no Go type yet.

- Verdict: unverified
- Evidence:

### A02  docs/specs/APP_CONFIG.md:90  [Not built yet]

* Validation of a repo's `gero.yaml` (the allowed-kinds check) and of `WorkflowsConfig`.

- Verdict: unverified
- Evidence:

### A03  docs/specs/APP_CONFIG.md:91  [Not built yet]

* Cycle warnings across more than two projects.

- Verdict: unverified
- Evidence:

### A04  docs/specs/APP_CONFIG.md:92  [Not built yet]

* `gero config show --resolved`.

- Verdict: unverified
- Evidence:

### A05  docs/specs/APP_CONFIG.md:93  [Not built yet]

* `gero doctor` checking the engine's findings as `config validate` does not yet either.

- Verdict: unverified
- Evidence:

### A06  docs/specs/APP_CONFIG.md:94  [Not built yet]

* The `chartsPackageBump` strategy, which the real config names, and writing the `dockerfile` format.

- Verdict: unverified
- Evidence:

### A07  docs/specs/APP_CONFIG.md:95  [Not built yet]

* The `lifecycle-date` warning.

- Verdict: unverified
- Evidence:

### A08  docs/specs/APP_CONFIG.md:99  [Open questions]

* Final kind and field names.

- Verdict: unverified
- Evidence:

### A09  docs/specs/APP_CONFIG.md:100  [Open questions]

* Whether any config other than `WorkflowsConfig` ever moves into target repos.

- Verdict: unverified
- Evidence:

### A10  docs/specs/CLI_CONFIG.md:156  [Not built yet]

* A `host` setting (`GERO_HOST`, `--host`) for a GitHub host other than github.com. Today the host is `GH_HOST`, else the one in `gh`'s config, else github.com ([GITHUB_ACCESS](GITHUB_ACCESS.md)).

- Verdict: unverified
- Evidence:

### A11  docs/specs/CLI_CONFIG.md:160  [Open questions]

* Final names for the user-file keys.

- Verdict: unverified
- Evidence:

### A12  docs/specs/CLI_FRAMEWORK.md:102  [Not built yet]

* `gero workflows sync`, `gero workflows verify` and `gero ci:reconcile` are stubs that return `errs.NotImplemented`. `gero ci:ship` and `gero dashboard` are not registered. The remaining steps of `ship` are [use cases](TASK_COMMANDS.md#use-cases-not-yet-commands) without command names.

- Verdict: unverified
- Evidence:

### A13  docs/specs/CLI_FRAMEWORK.md:103  [Not built yet]

* The exit-code helper. Until it exists, a usage or validation error exits 1, not 2.

- Verdict: unverified
- Evidence:

### A14  docs/specs/CLI_FRAMEWORK.md:104  [Not built yet]

* `--json` for `config validate`, and `--output env` for `auth status`.

- Verdict: unverified
- Evidence:

### A15  docs/specs/CLI_FRAMEWORK.md:108  [Open questions]

* Whether exit code 3 for drift is wanted, or `verify` output alone is enough.

- Verdict: unverified
- Evidence:

### A16  docs/specs/CLI_FRAMEWORK.md:109  [Open questions]

* Final name for `gero dashboard` versus `triage`.

- Verdict: unverified
- Evidence:

### A17  docs/specs/CLI_OUTPUT.md:58  [Not built yet]

* A shared yes/no confirmation in `internal/ui`: `ship`, `repo tag`, `shell install` and `doctor self-update` each ask their own.

- Verdict: unverified
- Evidence:

### A18  docs/specs/CLI_OUTPUT.md:59  [Not built yet]

* The final report of `ship` appended to `$GITHUB_STEP_SUMMARY` in CI, which belongs to `ci:ship`.

- Verdict: unverified
- Evidence:

### A19  docs/specs/CLI_OUTPUT.md:60  [Not built yet]

* The logger handed to every command: `ship` gets one built in `cmd/`, and the others do not log yet.

- Verdict: unverified
- Evidence:

### A20  docs/specs/CLI_OUTPUT.md:61  [Not built yet]

* The terminal and the prompt are verified by hand on a real terminal: tests run without one.

- Verdict: unverified
- Evidence:

### A21  docs/specs/DEPENDENCY_POLICY.md:40  [Open questions]

* Whether the integration sub-module needs a fake GitHub server library or hand-rolled `httptest` handlers, decided with the first integration test.

- Verdict: unverified
- Evidence:

### A22  docs/specs/EXECUTION_CONTEXT.md:92  [Not built yet]

* The cross-repo token check (`cigate.RequireCrossRepoToken`) and the workstation-signs detection (`clicfg.WorkstationSigns`) exist, and no command calls them yet.

- Verdict: unverified
- Evidence:

### A23  docs/specs/EXECUTION_CONTEXT.md:93  [Not built yet]

* The `auth status` fallback treats a 403 from `GET /user` as an installation token, which is unverified against a real one.

- Verdict: unverified
- Evidence:

### A24  docs/specs/EXECUTION_CONTEXT.md:94  [Not built yet]

* The preflight line that names workstation signs under a forced CI context.

- Verdict: unverified
- Evidence:

### A25  docs/specs/EXECUTION_CONTEXT.md:95  [Not built yet]

* `gero repo branch` and `gero repo tag` follow the one-confirmation and `--dry-run` rules but do not print the preflight line, which only `gero ship` prints.

- Verdict: unverified
- Evidence:

### A26  docs/specs/EXECUTION_CONTEXT.md:107  [Open questions]

* Whether workstation signs under a forced CI should do more than warn, for example refuse `--non-interactive` while a TTY is attached. A TTY is itself one of the signs, so this would block scripted local debugging.

- Verdict: unverified
- Evidence:

### A27  docs/specs/GITHUB_ACCESS.md:58  [Not built yet]

* The reconciler's interface over the client, and the reads and writes of issues, check runs and releases.

- Verdict: unverified
- Evidence:

### A28  docs/specs/GITHUB_ACCESS.md:59  [Not built yet]

* Enterprise hosts under `*.ghe.com` and `github.localhost`: `NewClient` refuses them until their API address is worked out.

- Verdict: unverified
- Evidence:

### A29  docs/specs/GITHUB_ACCESS.md:63  [Open questions]

* Whether we ever target hosts other than `github.com`.

- Verdict: unverified
- Evidence:


## B. Workspace, doctor, setup, update, repl (31)

### B01  docs/specs/WORKSPACE.md:58  [Not built yet]

* `config resolve repos --verbose` **(leaning)**: log the scanned directories, the remotes read from each clone, and why each candidate matched or not.

- Verdict: unverified
- Evidence:

### B02  docs/specs/DOCTOR.md:130  [Not built yet]

* The exit-code helper: until it exists a usage error, such as an unknown check name, exits 1, not 2.

- Verdict: unverified
- Evidence:

### B03  docs/specs/DOCTOR.md:131  [Not built yet]

* Checks for what later features need: token permissions for opening pull requests, access to the tracker repo, push access to each fork, and clones far behind upstream.

- Verdict: unverified
- Evidence:

### B04  docs/specs/DOCTOR.md:135  [Open questions]

* Whether checks should run in parallel, and when. Sequential until it hurts.

- Verdict: unverified
- Evidence:

### B05  docs/specs/DOCTOR.md:136  [Open questions]

* Whether a redacted, paste-friendly support mode is wanted (paths shortened, host names kept) or the default output is already safe enough. The output never contains a token, but it does contain local paths. Undecided; revisit when someone pastes it into an issue.

- Verdict: unverified
- Evidence:

### B06  docs/specs/SHELL_SETUP.md:54  [Not built yet]

* Windows PowerShell 5, and other shells (nushell, elvish).

- Verdict: unverified
- Evidence:

### B07  docs/specs/SHELL_SETUP.md:55  [Not built yet]

* Running the fish and PowerShell scripts in a real shell in tests.

- Verdict: unverified
- Evidence:

### B08  docs/specs/SHELL_SETUP.md:56  [Not built yet]

* Completing the project argument of `gero cd` from the config.

- Verdict: unverified
- Evidence:

### B09  docs/specs/SHELL_SETUP.md:57  [Not built yet]

* A `gero doctor` check that the block is installed and current.

- Verdict: unverified
- Evidence:

### B10  docs/specs/SELF_UPDATE.md:38  [Not built yet]

* Tests of the signature itself against a real release: the tests drive the real `binup` and `ghreleases` code and the real signer pattern, with a stand-in verifier for the signature, in memory and with no network.

- Verdict: unverified
- Evidence:

### B11  docs/specs/SELF_UPDATE.md:39  [Not built yet]

* `--version`, to install a named release or roll back.

- Verdict: unverified
- Evidence:

### B12  docs/specs/SELF_UPDATE.md:40  [Not built yet]

* An offline path (a `trusted_root.json` supplied by file), though `selfupdate.Config.TrustedRoot` takes one.

- Verdict: unverified
- Evidence:

### B13  docs/specs/SELF_UPDATE.md:41  [Not built yet]

* Windows: gero is released for macOS and Linux only.

- Verdict: unverified
- Evidence:

### B14  docs/specs/SELF_UPDATE.md:45  [Open questions]

* Whether `gero doctor` should report that a newer release exists, as a warning on `--online`.

- Verdict: unverified
- Evidence:

### B15  docs/specs/REPL.md:111  [Not built yet]

* **`help`.** It is cobra's help, which does not list `pwd`, `ls`, `clear` or `exit`, and describes the tree's `gero shell` (completion setup) where the REPL's `shell` is the child shell. A short block of the REPL's own commands before cobra's help, with `help <command>` left to cobra, is the first thing to try; a REPL-only help is the larger option.

- Verdict: unverified
- Evidence:

### B16  docs/specs/REPL.md:112  [Not built yet]

* **An unknown name** gets cobra's unknown-command error. The intended version is one line that points to `help` and `shell`, not a full help screen.

- Verdict: unverified
- Evidence:

### B17  docs/specs/REPL.md:113  [Not built yet]

* **The prompt** has no way to show that the last line failed, and only the placeholders `{project}`, `{dir}` and `{path}`; more come when a user asks for something it cannot say.

- Verdict: unverified
- Evidence:

### B18  docs/specs/REPL.md:114  [Not built yet]

* **`!<command line>`** and **marking the prompt of the child `shell`** ([Deferred ideas](#deferred-ideas)).

- Verdict: unverified
- Evidence:

### B19  docs/specs/REPL.md:120  [Deferred ideas]

* **`GERO_REPL=1` is the hook.** It is in the child's environment for this. A user can put `${GERO_REPL:+(gero) }` in their own `PS1` today.

- Verdict: unverified
- Evidence:

### B20  docs/specs/REPL.md:121  [Deferred ideas]

* **Setting `PS1` on the child does not work.** `PS1` is normally a shell variable and not exported, and `~/.bashrc` or `~/.zshrc` (and oh-my-zsh, starship, powerlevel10k) set it after the shell starts, so a value from the environment is overwritten for most users. Fish and PowerShell have no `PS1`.

- Verdict: unverified
- Evidence:

### B21  docs/specs/REPL.md:122  [Deferred ideas]

* **Option, preferred if it is ever done: an opt-in snippet in the block `gero shell install` manages** ([SHELL_SETUP](SHELL_SETUP.md)), prefixing the prompt for bash and zsh, with a fish equivalent, only when `GERO_REPL` is set. A prompt framework that redraws the prompt later can still overwrite it, and for those the documented route is the framework's own environment-variable module keyed on `GERO_REPL`.

- Verdict: unverified
- Evidence:

### B22  docs/specs/REPL.md:123  [Deferred ideas]

* **Option, advised against: start the child with an rc file of ours** (`bash --rcfile`, a temporary `ZDOTDIR` for zsh). It would force the marker, but it is shell-specific, changes how the user's shell starts, rewrites a prompt without being asked, and breaks the rule that `shell` runs the user's `$SHELL` as is.

- Verdict: unverified
- Evidence:

### B23  docs/specs/REPL.md:129  [Open questions]

* Which phase the REPL belongs to, if any ([PHASES](../PHASES.md)). It is not scheduled.

- Verdict: unverified
- Evidence:

### B24  docs/specs/TESTING.md:22  [Not built yet]

* The `test/integration` sub-module and its CI job.

- Verdict: unverified
- Evidence:

### B25  docs/specs/TESTING.md:23  [Not built yet]

* Golden files under `testdata/` for command output, which tests compare as strings today.

- Verdict: unverified
- Evidence:

### B26  docs/specs/TESTING.md:24  [Not built yet]

* A CI run of `go test ./...` in a deliberately hostile environment, to catch a test that reads the machine.

- Verdict: unverified
- Evidence:

### B27  docs/specs/PROJECT_LAYOUT.md:169  [Not built yet]

* A shared yes/no confirmation in `internal/ui`: `ship`, `repo tag`, `shell install` and `doctor self-update` each ask their own `[y/N]` through the style renderer ([CLI_OUTPUT](CLI_OUTPUT.md)).

- Verdict: unverified
- Evidence:

### B28  docs/specs/PROJECT_LAYOUT.md:170  [Not built yet]

* A task for closing superseded PRs, extracted from `internal/ship` when it is needed, and commands for `pushbranch`, `createpr` and `bumpcommit`, which are use cases without names ([TASK_COMMANDS](TASK_COMMANDS.md#use-cases-not-yet-commands)). `gero repo branch` and `gero repo tag` run `createbranch` and `createtag` alone.

- Verdict: unverified
- Evidence:

### B29  docs/specs/PROJECT_LAYOUT.md:171  [Not built yet]

* Narrowing `ship.Git` and `ship.Remote` now that the tasks define what they call: both still carry every method any task needs.

- Verdict: unverified
- Evidence:

### B30  docs/specs/PROJECT_LAYOUT.md:172  [Not built yet]

* `test/integration/` ([TESTING](TESTING.md)).

- Verdict: unverified
- Evidence:

### B31  docs/specs/PROJECT_LAYOUT.md:176  [Open questions]

* Whether generated code (for example JSON schema from config types) gets its own directory.

- Verdict: unverified
- Evidence:


## C. Releases, tags, ship and status (43)

### C01  docs/specs/BRANCH_TAGS.md:138  [Open questions]

* **Branch families as config.** Supported ref patterns default to `main` and `release/*`, and an optional config could override them, possibly in the repo-level `gero.yaml`. [ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md) currently allows only `WorkflowsConfig` in a target repo's `gero.yaml`, so this needs a decision there first. Unconfigured behavior stays the script's.

- Verdict: unverified
- Evidence:

### C02  docs/specs/BRANCH_TAGS.md:139  [Open questions]

* **Flags.** Whether `--ref`, `--sha` and `--repo <path>` are worth adding to run the command without the CI environment. Undecided; the environment rule stays the default.

- Verdict: unverified
- Evidence:

### C03  docs/specs/BRANCH_TAGS.md:140  [Open questions]

* **Default local short commit.** Decided: ask the `git` binary (section 3). Revisit only if a go-git abbreviation that matches git appears, since the subprocess inherits the environment and git config.

- Verdict: unverified
- Evidence:

### C04  docs/specs/BRANCH_TAGS.md:141  [Open questions]

* **Opt-in flag names and grouping.** One flag per improvement in section 6, or a single flag that turns on the related ones.

- Verdict: unverified
- Evidence:

### C05  docs/specs/BRANCH_TAGS.md:142  [Open questions]

* **Error reporting.** The stderr row in section 6 touches a global rule ([CLI_OUTPUT](CLI_OUTPUT.md): diagnostics to stderr). Whether it still needs a per-command opt-in.

- Verdict: unverified
- Evidence:

### C06  docs/specs/CHECK_SEMVER.md:115  [Open questions]

* **Opt-in flag names and grouping.** One flag per improvement in section 6, or one flag (for example for the SemVer grammar) that turns on the related ones together.

- Verdict: unverified
- Evidence:

### C07  docs/specs/CHECK_SEMVER.md:116  [Open questions]

* **Extra env format name.** `--output env-full` or `env-extra`. Decided with the CLI output work, since it adds a value to `--output`.

- Verdict: unverified
- Evidence:

### C08  docs/specs/CHECK_SEMVER.md:117  [Open questions]

* **Strict mode.** Whether `--strict` is worth adding (section 6).

- Verdict: unverified
- Evidence:

### C09  docs/specs/CHECK_SEMVER.md:118  [Open questions]

* **Non-SemVer `+`.** Whether the grammar mode should keep the script's "any `+`" meaning for non-SemVer text. Deferred until a real workflow depends on it.

- Verdict: unverified
- Evidence:

### C10  docs/specs/TASK_COMMANDS.md:90  [Not built yet]

* The use cases above as commands, under names not chosen.

- Verdict: unverified
- Evidence:

### C11  docs/specs/TASK_COMMANDS.md:94  [Open questions]

* Whether each use case is wanted as a command of its own, and under what name.

- Verdict: unverified
- Evidence:

### C12  docs/specs/TASK_COMMANDS.md:95  [Open questions]

* Whether `repo branch` gets `--from <ref>`.

- Verdict: unverified
- Evidence:

### C13  docs/specs/TASK_COMMANDS.md:96  [Open questions]

* For `repo tag`: whether the default bump after a stable is always a patch with `--minor` and `--major` for the rest, and the name `--stable` for the promotion. Whether an annotated or signed tag is ever wanted, which the API does not make.

- Verdict: unverified
- Evidence:

### C14  docs/specs/TASK_COMMANDS.md:97  [Open questions]

* Whether a later token such as `@` should stand for the checked-out branch where a branch is a positional, which none is now.

- Verdict: unverified
- Evidence:

### C15  docs/specs/TASK_COMMANDS.md:98  [Open questions]

* Whether the push and PR use cases share one push-target resolution call or each repeats it. A shared one belongs in `internal/ship` with `resolvePushTarget` until a task needs it, then in a domain package.

- Verdict: unverified
- Evidence:

### C16  docs/specs/REPO_STATUS.md:53  [Not built yet]

* How many releases a branch is behind, which needs the tags between two releases and the version the strategy found (`ship.Holding`, and `git.LatestReleases` for the per-series index).

- Verdict: unverified
- Evidence:

### C17  docs/specs/REPO_STATUS.md:54  [Not built yet]

* The open PR for a branch, so a consumer that is behind can say a bump is already proposed (`ship.Holding`). It would be a tier of its own, with its cost, using the PR lookup and the marker `ship` uses.

- Verdict: unverified
- Evidence:

### C18  docs/specs/REPO_STATUS.md:55  [Not built yet]

* The images a release publishes, as `repo images exist` checks them (`repostatus.Options`).

- Verdict: unverified
- Evidence:

### C19  docs/specs/REPO_STATUS.md:59  [Open questions]

* Whether a project that is both a producer and a consumer, such as one that takes from another and ships on, should show the Sources and Consumers sections together or only the one asked for.

- Verdict: unverified
- Evidence:

### C20  docs/specs/LINES_RESOLVE.md:35  [Not built yet]

* Following the chain beyond one hop, such as `ob-team-charts` into `charts` into `rancher` (`also` lists the project and its own links today; it does not connect them back to the hub).

- Verdict: unverified
- Evidence:

### C21  docs/specs/LINES_RESOLVE.md:36  [Not built yet]

* A line the lines document has no entry for, but a project has: validation rejects it when the config is loaded, so there is no gap for it here.

- Verdict: unverified
- Evidence:

### C22  docs/specs/LINES_RESOLVE.md:37  [Not built yet]

* The answer for one producer and one line, "where does shell's `v2.15` go", which is a different command and not this map.

- Verdict: unverified
- Evidence:

### C23  docs/specs/LINES_RESOLVE.md:41  [Open questions]

* How the chain through `rancher/charts` connects back to the hub and leaves `also`: the holdings of Assets in Rancher and of charts in Assets are in the config, and following them is the walk of [CHAIN_TRACING](CHAIN_TRACING.md), which is not built.

- Verdict: unverified
- Evidence:

### C24  docs/specs/PUSH_TO_REPO.md:188  [Not built yet]

* A test of the push against a real scratch repository in the integration tier ([TESTING](TESTING.md)), which would keep the credential questions in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#push-credentials-and-signing) settled by a test. `gitcli` has a spike test that runs only when `GERO_SPIKE_PUSH_URL` names one.

- Verdict: unverified
- Evidence:

### C25  docs/specs/PUSH_TO_REPO.md:189  [Not built yet]

* A way to name a step's commit, or fold it into the previous one, which is the first knob to add if a project needs it.

- Verdict: unverified
- Evidence:

### C26  docs/specs/PUSH_TO_REPO.md:190  [Not built yet]

* `gero ci:ship` ([CI_SHIP](CI_SHIP.md)).

- Verdict: unverified
- Evidence:

### C27  docs/specs/PUSH_TO_REPO.md:191  [Not built yet]

* The sequence above as chained tasks. Making the branch is the `createbranch` task, the edit and its commits are `bumpcommit`, pushing is `pushbranch` and opening and labelling the PR is `createpr`; the other steps work today inside `internal/ship`, and moving each to `internal/task/` changes no behavior and none of this spec, apart from this list ([PROJECT_LAYOUT](PROJECT_LAYOUT.md#tasks-single-actions-that-compose)).

- Verdict: unverified
- Evidence:

### C28  docs/specs/PUSH_TO_REPO.md:192  [Not built yet]

* `from: producer` and `from: central` for steps, which need the producer's checkout and the bundle.

- Verdict: unverified
- Evidence:

### C29  docs/specs/PUSH_TO_REPO.md:196  [Open questions]

* Whether the default PR title should carry something that identifies the tooling. Identity is the marker, the author and, in shared repositories, a label such as `origin/bot`; a title is a project's own convention, and matching on it is fragile.

- Verdict: unverified
- Evidence:

### C30  docs/specs/PUSH_TO_REPO.md:197  [Open questions]

* How steps such as `go generate` get a toolchain that matches the target branch. A workstation without it fails with a `gero doctor` hint.

- Verdict: unverified
- Evidence:

### C31  docs/specs/PUSH_TO_REPO.md:198  [Open questions]

* The shape of `gero ci:ship`, in [CI_SHIP](CI_SHIP.md#open-questions).

- Verdict: unverified
- Evidence:

### C32  docs/specs/PUSH_TO_REPO.md:199  [Open questions]

* Whether a clean clone with untracked files should be worked in place when none collide, as designed here, or sent to a worktree whenever there are any.

- Verdict: unverified
- Evidence:

### C33  docs/specs/PUSH_TO_REPO.md:200  [Open questions]

* How a strategy reads the value it wrote for the downgrade check. A holding reads its own value, and `chartsPackageBump` is not built.

- Verdict: unverified
- Evidence:

### C34  docs/specs/CI_SHIP.md:35  [Not built yet]

* The command, its `cmd/` registration (CI-only, [CLI_FRAMEWORK](CLI_FRAMEWORK.md)) and its loop.

- Verdict: unverified
- Evidence:

### C35  docs/specs/CI_SHIP.md:36  [Not built yet]

* Resolving the bot's identity in a workflow: the domain passes it to the commits, and `GERO_BOT_USER` with the id lookup is tested without a real bot.

- Verdict: unverified
- Evidence:

### C36  docs/specs/CI_SHIP.md:37  [Not built yet]

* The temporary clone as a `Workspace`, and the token as a one-shot credential for the push (`gitcli.Push` and `gitcli.PushTag` take the engineer's own today).

- Verdict: unverified
- Evidence:

### C37  docs/specs/CI_SHIP.md:38  [Not built yet]

* The markdown step summary, and the workflow run URL in the PR body.

- Verdict: unverified
- Evidence:

### C38  docs/specs/CI_SHIP.md:39  [Not built yet]

* `from: producer` and `from: central` for steps.

- Verdict: unverified
- Evidence:

### C39  docs/specs/CI_SHIP.md:40  [Not built yet]

* A real workflow that calls it, beside the bash and then in place of it.

- Verdict: unverified
- Evidence:

### C40  docs/specs/CI_SHIP.md:44  [Open questions]

* **Who makes the clone.** Either `gero` does its own setup (a shallow temporary clone with the workflow's token, the bot identity resolved, a push to the upstream), or workflow steps check out the consumer and hand `gero` the directory. Both are one more `Workspace`, and only the setup differs. The first keeps the workflow to one step and tests the same code in both contexts. The second keeps checkout and authentication in the actions the workflow already uses, and needs no clone code. Decide when the first workflow is written.

- Verdict: unverified
- Evidence:

### C41  docs/specs/CI_SHIP.md:45  [Open questions]

* Whether `ci:ship` fans out across lines in one run, as `gero ship` does, or a workflow runs it once for each line it is given.

- Verdict: unverified
- Evidence:

### C42  docs/specs/CI_SHIP.md:46  [Open questions]

* How two runs of the same bump at once are made safe. The branch name is deterministic and a push replaces only a branch it saw, so the second push is rejected, but what that run reports and whether it retries is not decided.

- Verdict: unverified
- Evidence:

### C43  docs/specs/CI_SHIP.md:47  [Open questions]

* Whether the step summary and the PR comments are the same text, or the summary is longer.

- Verdict: unverified
- Evidence:


## D. Chain, bundle, images, metadata (26)

### D01  docs/specs/CHAIN_TRACING.md:85  [Not built yet]

* Reading a commit or a branch holding at a ref, which `assets-pins-charts` and `rancher-follows-charts-branch` need, and the `lock.yaml` read built on it ([UPDATE_ENGINE](../design/UPDATE_ENGINE.md)).

- Verdict: unverified
- Evidence:

### D02  docs/specs/CHAIN_TRACING.md:86  [Not built yet]

* The walk through several hops, the tree in `lines resolve` and `--depth`.

- Verdict: unverified
- Evidence:

### D03  docs/specs/CHAIN_TRACING.md:87  [Not built yet]

* Checking a held tag's stage against the release team's rule.

- Verdict: unverified
- Evidence:

### D04  docs/specs/CHAIN_TRACING.md:88  [Not built yet]

* `chartsPackageBump` and BRO's tag into a charts bump.

- Verdict: unverified
- Evidence:

### D05  docs/specs/CHAIN_TRACING.md:101  [Open questions]

* The key in Rancher's `build.yaml` that names the charts branch before v2.16, and whether it holds the branch name alone.

- Verdict: unverified
- Evidence:

### D06  docs/specs/CHAIN_TRACING.md:102  [Open questions]

* How a pinned commit in `lock.yaml` is read: a small read-only helper over the key-path code, or something else, and what "release" a pin is compared with (the head of the stage's branch). The key path needs a quoting rule for a key with a dot, such as the line `2.16`.

- Verdict: unverified
- Evidence:

### D07  docs/specs/CHAIN_TRACING.md:103  [Open questions]

* Whether `since` and `until` on a holding is all the line-range a holding needs.

- Verdict: unverified
- Evidence:

### D08  docs/specs/CHAIN_TRACING.md:104  [Open questions]

* How BRO's tag maps to the chart version in `release.yaml` (`110.0.3+up11.0.4`), and whether that rule is written down by the release team or only encoded in the scripts that make it.

- Verdict: unverified
- Evidence:

### D09  docs/specs/CHAIN_TRACING.md:105  [Open questions]

* Whether an Assets tag's charts commit is enough to answer "is this chart version in this tag", which is traced through `lock.yaml` at that tag to the contents of `rancher/charts` at the commit it records.

- Verdict: unverified
- Evidence:

### D10  docs/specs/CHAIN_TRACING.md:106  [Open questions]

* How a charts line differs in how it is bumped beyond the steps that `since` and `until` already vary.

- Verdict: unverified
- Evidence:

### D11  docs/specs/CENTRAL_BUNDLE.md:101  [Not built yet]

* The newer of a local copy and the bundle: in `auto` a local source wins, and `--config-source` picks one or the other.

- Verdict: unverified
- Evidence:

### D12  docs/specs/CENTRAL_BUNDLE.md:102  [Not built yet]

* Conditional requests (`ghreleases` has none), so the daily refresh is by the age of the cache alone.

- Verdict: unverified
- Evidence:

### D13  docs/specs/CENTRAL_BUNDLE.md:103  [Not built yet]

* A refresh interval setting. Reproducing an old run needs no more: download that release's bundle and give it with `--bundle-file`.

- Verdict: unverified
- Evidence:

### D14  docs/specs/CENTRAL_BUNDLE.md:104  [Not built yet]

* The templates and generated chunks.

- Verdict: unverified
- Evidence:

### D15  docs/specs/CENTRAL_BUNDLE.md:108  [Open questions]

* What `ghreleases` does about the REST API's unauthenticated rate limit (60 requests an hour per IP, shared by CI runners and office networks). If it uses the API, the daily refresh limit matters more.

- Verdict: unverified
- Evidence:

### D16  docs/specs/CENTRAL_BUNDLE.md:109  [Open questions]

* Whether `centralConfigDir`, which names the central repo's `config/` directory, should name the whole central repo once the bundle holds templates as well as config ([CLI_CONFIG](CLI_CONFIG.md)).

- Verdict: unverified
- Evidence:

### D17  docs/specs/CENTRAL_BUNDLE.md:110  [Open questions]

* How `gero ci:*` commands in other repos get config. Today the bundle is never used in CI, which sets `GERO_CENTRAL_CONFIG_DIR` to a checkout of the central repo. Whether a job without a checkout should use the bundle of the installed release is not decided.

- Verdict: unverified
- Evidence:

### D18  docs/specs/CENTRAL_BUNDLE.md:111  [Open questions]

* Whether the refresh interval is a setting.

- Verdict: unverified
- Evidence:

### D19  docs/specs/IMAGES_EXIST.md:67  [Open questions]

* Whether a registry that ignores `HEAD` needs a `GET` fallback. The three here are expected to answer `HEAD`.

- Verdict: unverified
- Evidence:

### D20  docs/specs/IMAGES_EXIST.md:68  [Open questions]

* Whether the check should compare digests across registries, which the script does not do.

- Verdict: unverified
- Evidence:

### D21  docs/specs/IMAGES_EXIST.md:69  [Open questions]

* Whether a mirror's timeout and retry policy needs to be a setting.

- Verdict: unverified
- Evidence:

### D22  docs/specs/METADATA.md:82  [Not built yet]

* The gero-owned labels (`gero/stale`, `gero/superseded`), their creation on demand and their reconciliation.

- Verdict: unverified
- Evidence:

### D23  docs/specs/METADATA.md:83  [Not built yet]

* The control labels (`gero/hold`) and the check of one before work.

- Verdict: unverified
- Evidence:

### D24  docs/specs/METADATA.md:84  [Not built yet]

* Labels on tracking issues, which wait for the reconciler.

- Verdict: unverified
- Evidence:

### D25  docs/specs/METADATA.md:88  [Open questions]

* Whether a `pullRequests` block can name labels per class, or a global config injects the existing Rancher labels.

- Verdict: unverified
- Evidence:

### D26  docs/specs/METADATA.md:89  [Open questions]

* The full gero-owned vocabulary, such as phase labels on tracking issues.

- Verdict: unverified
- Evidence:


## E. Design, process and implementation notes (13)

### E01  docs/design/TOOLING_GOALS.md:82  [Open]

* A chart version holds its upstream version after `+up`; the stable test reads that part

- Verdict: unverified
- Evidence:

### E02  docs/design/TOOLING_GOALS.md:83  [Open]

* A component with no tracing config: how a view names it as not traced

- Verdict: unverified
- Evidence:

### E03  docs/design/TOOLING_GOALS.md:84  [Open]

* A CVE stable skips the RC; readiness only looks at what is held, so this is not an exception here

- Verdict: unverified
- Evidence:

### E04  docs/process/RELEASE_PROCESS.md:115  [Open]

* Who adds a line in rancher-assets

- Verdict: unverified
- Evidence:

### E05  docs/IMPLEMENTATION.md:101  [Open Questions]

* Scope and order of the milestones: see [PHASES](PHASES.md).

- Verdict: unverified
- Evidence:

### E06  docs/IMPLEMENTATION.md:102  [Open Questions]

* Config-shape questions (final names, whether config moves into target repos): see [APP_CONFIG](specs/APP_CONFIG.md). CLI-layer questions are in the spec each belongs to ([CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md), [DEPENDENCY_POLICY](specs/DEPENDENCY_POLICY.md)).

- Verdict: unverified
- Evidence:

### E07  docs/IMPLEMENTATION.md:103  [Open Questions]

* Reconciler triggering: cron only, or `repository_dispatch` where needed.

- Verdict: unverified
- Evidence:

### E08  docs/IMPLEMENTATION.md:104  [Open Questions]

* Which existing workflows to standardize first, and whether the central repo publishes them as reusable workflows or composite actions.

- Verdict: unverified
- Evidence:

### E09  docs/IMPLEMENTATION.md:105  [Open Questions]

* Whether the CLI replaces the existing "push to X" scripts or wraps them at first (decided before v1.0.0, see [PHASES](PHASES.md)).

- Verdict: unverified
- Evidence:

### E10  docs/IMPLEMENTATION.md:106  [Open Questions]

* How new Rancher lines are cut and registered in the line map (manual edit, or a command that updates it). Updating the dates of lines already in the map is planned as `gero lines sync`; a new line also needs a Kubernetes range, which the lifecycle page does not give.

- Verdict: unverified
- Evidence:

### E11  docs/IMPLEMENTATION.md:107  [Open Questions]

* How downstream repos pin and call reusable workflows; resolve when we review a repo that consumes the existing core branches.

- Verdict: unverified
- Evidence:

### E12  docs/IMPLEMENTATION.md:108  [Open Questions]

* The `VERSION.md` format: one standard layout across repos, which fields it shows (branch, tag, Rancher line, Kubernetes range, EOL section), and whether other tools that read `VERSION.md` today (for example frameworks-automation reads per-repo `VERSION.md` at runtime) need the old formats to keep working.

- Verdict: unverified
- Evidence:

### E13  docs/IMPLEMENTATION.md:109  [Open Questions]

* The "branch for X" workflow itself: which steps `gero` automates (create branches, update `branches.lines`, regenerate `VERSION.md`, re-render workflows) and in what order.

- Verdict: unverified
- Evidence:


## F. Code TODO markers (5)

### F01  internal/platform/git/gitcli/clone.go:15  [code marker]

`// not, and it must run the same global config and hooks. TODO(go-git): revisit`

- Verdict: unverified
- Evidence:

### F02  internal/platform/git/latest.go:33  [code marker]

`// TODO(release-state): this answers "what is newest" for one line or project.`

- Verdict: unverified
- Evidence:

### F03  internal/cli/repoclone/repoclone.go:221  [code marker]

`// TODO(host): github.com only; the host of a GitHub Enterprise Server is not read.`

- Verdict: unverified
- Evidence:

### F04  internal/cli/repostatus/repostatus.go:83  [code marker]

`// TODO(release-state): a tier for the images a release publishes (the fourth`

- Verdict: unverified
- Evidence:

### F05  internal/ship/holdings.go:32  [code marker]

`// TODO(release-state): two things gero repo status cannot say yet, both in`

- Verdict: unverified
- Evidence:


## T. Current docs/TODO.md (cross-check only) (74)

### T01  docs/TODO.md:15  [Needed for v1.0.0 / Ship from CI]

* **L: Build the `gero ci:ship` command.** Register it as CI-only and run the bump loop of `gero ship` as the bot: the bot identity from `GERO_BOT_USER`, and `cigate.RequireCrossRepoToken` called before work. Done when a test drives it end to end against fakes.

- Verdict: unverified
- Evidence:

### T02  docs/TODO.md:16  [Needed for v1.0.0 / Ship from CI]

* **M: Push with the workflow's token from a temporary clone.** The clone is a `Workspace` and the token is a one-shot credential for `gitcli.Push` and `PushTag`. Done when `ci:ship` pushes without the engineer's own credentials.

- Verdict: unverified
- Evidence:

### T03  docs/TODO.md:17  [Needed for v1.0.0 / Ship from CI]

* **S: Write the markdown step summary and put the workflow run URL in the PR body** of a `ci:ship` run.

- Verdict: unverified
- Evidence:

### T04  docs/TODO.md:18  [Needed for v1.0.0 / Ship from CI]

* **M: Run steps `from: producer` and `from: central`.** `steprun` refuses both. Done when a step can run a script of the producer's checkout or one from the bundle, in `ci:ship` only. [APP_CONFIG](specs/APP_CONFIG.md)

- Verdict: unverified
- Evidence:

### T05  docs/TODO.md:19  [Needed for v1.0.0 / Ship from CI]

* **M: Let `ci:*` commands run without a checkout of the central config.** Both CI workflows point `GERO_CENTRAL_CONFIG_DIR` at a checkout. Decide whether a job may use the bundle of the installed release, and build it. [CENTRAL_BUNDLE](specs/CENTRAL_BUNDLE.md)

- Verdict: unverified
- Evidence:

### T06  docs/TODO.md:20  [Needed for v1.0.0 / Ship from CI]

* **M: Add a workflow to `scc-operator` and `shell` that installs a pinned, signed `gero` and runs `ci:ship`,** beside the bash script, then in place of it. Needs `ci:ship`.

- Verdict: unverified
- Evidence:

### T07  docs/TODO.md:21  [Needed for v1.0.0 / Ship from CI]

* **S (needs a real run): Verify the `auth status` 403 path against a real installation token.** It is tested with fakes only. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)

- Verdict: unverified
- Evidence:

### T08  docs/TODO.md:27  [Needed for v1.0.0 / The chain]

* **S (no code): Answer the open questions of [CHAIN_TRACING](specs/CHAIN_TRACING.md)** with the release team: the Rancher key for the charts branch before v2.16, how to read the commit pinned in `lock.yaml`, and how a BRO tag maps to a chart version. Done when each answer is in the spec. The two items below need them.

- Verdict: unverified
- Evidence:

### T09  docs/TODO.md:28  [Needed for v1.0.0 / The chain]

* **M: Read a commit or a branch that a repo holds at a ref.** The config models both kinds and validates them; `resolveHolding` handles only releases. Includes the `lock.yaml` read. Done when `gero repo status` and `gero lines resolve` report what such a holding holds. [CHAIN_TRACING](specs/CHAIN_TRACING.md), [UPDATE_ENGINE](design/UPDATE_ENGINE.md)

- Verdict: unverified
- Evidence:

### T10  docs/TODO.md:29  [Needed for v1.0.0 / The chain]

* **S: Let the update engine read and write the `dockerfile` format.** It returns `ErrFormatNotBuilt`. Done when the refusal test is replaced by a read and write test. [UPDATE_ENGINE](design/UPDATE_ENGINE.md)

- Verdict: unverified
- Evidence:

### T11  docs/TODO.md:30  [Needed for v1.0.0 / The chain]

* **L: Walk several hops in `gero lines resolve`,** with the tree and `--depth`. Needs the reader above. [LINES_RESOLVE](specs/LINES_RESOLVE.md)

- Verdict: unverified
- Evidence:

### T12  docs/TODO.md:31  [Needed for v1.0.0 / The chain]

* **L: Build the `chartsPackageBump` strategy,** so BRO's tag becomes a charts bump. It is registered as a stub that stops a run before it writes. Done when the stub test is replaced by a test of a real bump. [CHAIN_TRACING](specs/CHAIN_TRACING.md)

- Verdict: unverified
- Evidence:

### T13  docs/TODO.md:32  [Needed for v1.0.0 / The chain]

* **M: Check a held tag's stage against the release team's rule.** Done when a mismatch is reported by `gero repo status` or the walk. [CHAIN_TRACING](specs/CHAIN_TRACING.md)

- Verdict: unverified
- Evidence:

### T14  docs/TODO.md:33  [Needed for v1.0.0 / The chain]

* **S: Make `gero config validate` and `gero doctor` build the update engine and report its errors,** so a strategy that cannot run fails there and not at `ship`. [APP_CONFIG](specs/APP_CONFIG.md)

- Verdict: unverified
- Evidence:

### T15  docs/TODO.md:34  [Needed for v1.0.0 / The chain]

* **S (no code): Check the config against the repos.** Confirm charts' stable branch names (`release-v2.X`) and find the package of `ob-team-charts`, which is left out until it is known.

- Verdict: unverified
- Evidence:

### T16  docs/TODO.md:38  [Needed for v1.0.0 / Confidence in the foundation]

* **M (no code): Get the process docs reviewed by the release team.** All of them are still drafts. Done when each says reviewed, with a date and the reviewer. [process](process/README.md)

- Verdict: unverified
- Evidence:

### T17  docs/TODO.md:39  [Needed for v1.0.0 / Confidence in the foundation]

* **M: Rename the config kind from `gero/v1alpha2` to `gero/v1`** once the field names are settled. Done when the schemas, `config/`, the tests and [APP_CONFIG](specs/APP_CONFIG.md) agree.

- Verdict: unverified
- Evidence:

### T18  docs/TODO.md:40  [Needed for v1.0.0 / Confidence in the foundation]

* **S: Add the exit-code helper,** so a usage or validation error exits 2. Done when a typed error maps to 2 in `Execute` and each command class has a test. [CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md), [DOCTOR](specs/DOCTOR.md)

- Verdict: unverified
- Evidence:

### T19  docs/TODO.md:41  [Needed for v1.0.0 / Confidence in the foundation]

* **M: Add the integration test tier,** a `test/integration` module that runs the built binary, and a CI job for it. Done when one black-box test runs and the module adds nothing to the root `go.mod`. [TESTING](specs/TESTING.md)

- Verdict: unverified
- Evidence:

### T20  docs/TODO.md:42  [Needed for v1.0.0 / Confidence in the foundation]

* **M: Add an integration test that pushes to a scratch repository.** Needs the tier. [PUSH_TO_REPO](specs/PUSH_TO_REPO.md)

- Verdict: unverified
- Evidence:

### T21  docs/TODO.md:43  [Needed for v1.0.0 / Confidence in the foundation]

* **S: Run `go test ./...` once in CI with a hostile environment** (a junk `HOME`, `GERO_*`, `GITHUB_TOKEN`, `XDG_*`). Done when a test that reads the machine fails the job. [TESTING](specs/TESTING.md)

- Verdict: unverified
- Evidence:

### T22  docs/TODO.md:44  [Needed for v1.0.0 / Confidence in the foundation]

* **S: Add a Go test that validates every file of `config/` against the JSON Schemas.** Needs a validator module. [APP_CONFIG](specs/APP_CONFIG.md#json-schemas)

- Verdict: unverified
- Evidence:

### T23  docs/TODO.md:45  [Needed for v1.0.0 / Confidence in the foundation]

* **S (needs a real run): Confirm a release end to end:** `cosign` runs in the `ci-image/go1.27` container, the bundles land on the release, and `gero doctor self-update` installs it. [SELF_UPDATE](specs/SELF_UPDATE.md)

- Verdict: unverified
- Evidence:

### T24  docs/TODO.md:46  [Needed for v1.0.0 / Confidence in the foundation]

* **S (needs a real run): Confirm a pushed tag starts the release workflow** of a real repo, for `gero repo tag --push`. [TASK_COMMANDS](specs/TASK_COMMANDS.md)

- Verdict: unverified
- Evidence:

### T25  docs/TODO.md:47  [Needed for v1.0.0 / Confidence in the foundation]

* **S (no code): Correct the specs that no longer match the code.** `REPO_STATUS` names a `TODO(releasetag)` marker that does not exist. `BRANCH_TAGS` lists the default short commit as open, and the code settles it. `PROJECT_LAYOUT` asks where generated code goes, and `generated/` and `tools/` answer it. `CENTRAL_BUNDLE` asks about rate limits, and `bundlestore` reports `ErrRateLimited`.

- Verdict: unverified
- Evidence:

### T26  docs/TODO.md:55  [Good places to start / Commands]

* **S: Make `gero doctor --online` warn when a newer `gero` release exists.** Done when a stand-in releases source drives a test. [SELF_UPDATE](specs/SELF_UPDATE.md)

- Verdict: unverified
- Evidence:

### T27  docs/TODO.md:56  [Good places to start / Commands]

* **S: Add `--json` to `gero config validate`,** a list of problems with the same exit code as text. [CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md)

- Verdict: unverified
- Evidence:

### T28  docs/TODO.md:57  [Good places to start / Commands]

* **S: Make `gero config resolve repos -v` log the scanned directories, the remotes read and why each candidate matched or not.** [WORKSPACE](specs/WORKSPACE.md)

- Verdict: unverified
- Evidence:

### T29  docs/TODO.md:58  [Good places to start / Commands]

* **S: Complete the project argument of `gero cd` from the config.** [SHELL_SETUP](specs/SHELL_SETUP.md)

- Verdict: unverified
- Evidence:

### T30  docs/TODO.md:59  [Good places to start / Commands]

* **S: Add a `gero doctor` check that the shell block is installed and current.** [SHELL_SETUP](specs/SHELL_SETUP.md)

- Verdict: unverified
- Evidence:

### T31  docs/TODO.md:63  [Good places to start / `ship` and `repo status`]

* **S: Find an existing bump PR by its marker in `gero ship`,** not by listing the newest 100 open PRs. On a busy base such as `main` of `rancher/rancher` an older PR of the bump is missed, and `ship` only warns. Done when a test with more than 100 open PRs finds it. [PUSH_TO_REPO](specs/PUSH_TO_REPO.md)

- Verdict: unverified
- Evidence:

### T32  docs/TODO.md:64  [Good places to start / `ship` and `repo status`]

* **S: Print the preflight line that names workstation signs under a forced CI context** in `gero ship`. `clicfg.WorkstationSigns` has no caller. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)

- Verdict: unverified
- Evidence:

### T33  docs/TODO.md:65  [Good places to start / `ship` and `repo status`]

* **S: Print the preflight line in `gero repo branch` and `gero repo tag`,** which follow the other rules and do not print it. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)

- Verdict: unverified
- Evidence:

### T34  docs/TODO.md:66  [Good places to start / `ship` and `repo status`]

* **M: Show in `gero repo status` how many releases a consumer is behind.** Needs the tags between two releases (`TODO(release-state)`). [REPO_STATUS](specs/REPO_STATUS.md)

- Verdict: unverified
- Evidence:

### T35  docs/TODO.md:67  [Good places to start / `ship` and `repo status`]

* **M: Show in `gero repo status` the open bump PR of a consumer that is behind,** as a tier of its own using the marker lookup. [REPO_STATUS](specs/REPO_STATUS.md)

- Verdict: unverified
- Evidence:

### T36  docs/TODO.md:68  [Good places to start / `ship` and `repo status`]

* **S: Show in `gero repo status` the images a release publishes,** using `internal/registry.CheckProject`. [REPO_STATUS](specs/REPO_STATUS.md)

- Verdict: unverified
- Evidence:

### T37  docs/TODO.md:72  [Good places to start / Config]

* **S: Raise the `lifecycle-date` warning** when a release is shipped to a line past a lifecycle date. Done when the rule is in `rules.go` and the [CONFIG_SCHEMA](design/CONFIG_SCHEMA.md#warnings) table says so.

- Verdict: unverified
- Evidence:

### T38  docs/TODO.md:73  [Good places to start / Config]

* **S: Warn on holding cycles longer than two projects.** Done when a three-project cycle raises `circular-holding`. [APP_CONFIG](specs/APP_CONFIG.md)

- Verdict: unverified
- Evidence:

### T39  docs/TODO.md:74  [Good places to start / Config]

* **L: Design the opt-in flags of `gero semver check` and `gero repo build-tags`,** such as a SemVer grammar mode and literal prefix matching, then build them with parity tests. The open questions of both specs come first. [CHECK_SEMVER](specs/CHECK_SEMVER.md), [BRANCH_TAGS](specs/BRANCH_TAGS.md)

- Verdict: unverified
- Evidence:

### T40  docs/TODO.md:78  [Good places to start / Internals]

* **S: Extract one `ui.Confirm`** for `ship`, `repo tag`, `shell install` and `doctor self-update`, which each draw their own `[y/N]`. Done when the four use it. [CLI_OUTPUT](specs/CLI_OUTPUT.md)

- Verdict: unverified
- Evidence:

### T41  docs/TODO.md:79  [Good places to start / Internals]

* **M: Hand the logger to every command through the context.** `ship` builds its own in `cmd/`, and the others do not log. [CLI_OUTPUT](specs/CLI_OUTPUT.md)

- Verdict: unverified
- Evidence:

### T42  docs/TODO.md:80  [Good places to start / Internals]

* **M: Narrow `ship.Git` and `ship.Remote` to what the tasks call.** Both carry every method any task needs. [PROJECT_LAYOUT](specs/PROJECT_LAYOUT.md)

- Verdict: unverified
- Evidence:

### T43  docs/TODO.md:90  [Later (v1.x and beyond) / Tracking issues and the reconciler]

* **L: Write and render the state block of a tracking issue,** the part a machine reads and the view a person reads.

- Verdict: unverified
- Evidence:

### T44  docs/TODO.md:91  [Later (v1.x and beyond) / Tracking issues and the reconciler]

* **M: Create the `gero/stale` and `gero/superseded` labels on demand and reconcile them.**

- Verdict: unverified
- Evidence:

### T45  docs/TODO.md:92  [Later (v1.x and beyond) / Tracking issues and the reconciler]

* **M: Honor `gero/hold`:** check for it before work, so a held line is skipped.

- Verdict: unverified
- Evidence:

### T46  docs/TODO.md:93  [Later (v1.x and beyond) / Tracking issues and the reconciler]

* **L: Build `gero ci:reconcile`,** which brings the tracking issues in line with what the repos hold. It is a stub. Needs the items above and the reconciler's reads and writes of issues, check runs and releases.

- Verdict: unverified
- Evidence:

### T47  docs/TODO.md:99  [Later (v1.x and beyond) / Checks over the chain]

* **L: Check stable readiness:** would a stable Rancher build be valid with what Rancher holds? Needs the multi-hop walk.

- Verdict: unverified
- Evidence:

### T48  docs/TODO.md:100  [Later (v1.x and beyond) / Checks over the chain]

* **M: Check line consistency:** does each hop hold a version of its own line?

- Verdict: unverified
- Evidence:

### T49  docs/TODO.md:106  [Later (v1.x and beyond) / The central bundle and workflows]

* **M: Add the `WorkflowsConfig` type and schema,** and validate a repo's `gero.yaml` against the allowed kinds. [APP_CONFIG](specs/APP_CONFIG.md)

- Verdict: unverified
- Evidence:

### T50  docs/TODO.md:107  [Later (v1.x and beyond) / The central bundle and workflows]

* **L: Build `gero workflows sync` and `gero workflows verify`.** Both are stubs. Sync brings the templates and generated chunks into a repo, and verify fails when a copy has drifted. Needs the type above.

- Verdict: unverified
- Evidence:

### T51  docs/TODO.md:108  [Later (v1.x and beyond) / The central bundle and workflows]

* **M: Generate `VERSION.md` from config.**

- Verdict: unverified
- Evidence:

### T52  docs/TODO.md:109  [Later (v1.x and beyond) / The central bundle and workflows]

* **S: Send conditional requests when refreshing the bundle,** so an unchanged bundle is not downloaded again.

- Verdict: unverified
- Evidence:

### T53  docs/TODO.md:110  [Later (v1.x and beyond) / The central bundle and workflows]

* **S: Add a setting for the bundle refresh interval.** `bundlestore.Resolver` takes one; no setting supplies it.

- Verdict: unverified
- Evidence:

### T54  docs/TODO.md:111  [Later (v1.x and beyond) / The central bundle and workflows]

* **S: Decide whether `auto` should ever prefer a newer bundle to a local copy,** and record the answer in the spec.

- Verdict: unverified
- Evidence:

### T55  docs/TODO.md:115  [Later (v1.x and beyond) / Lines and lifecycle]

* **M: Build `gero lines sync`,** which fetches the SUSE lifecycle page and updates the dates in the lines file, showing the diff first. `internal/lifecycle` already parses the page; nothing calls it. [IMPLEMENTATION](IMPLEMENTATION.md)

- Verdict: unverified
- Evidence:

### T56  docs/TODO.md:116  [Later (v1.x and beyond) / Lines and lifecycle]

* **S: Make `gero doctor` flag drift between the lifecycle dates and the page.** Needs `lines sync`.

- Verdict: unverified
- Evidence:

### T57  docs/TODO.md:117  [Later (v1.x and beyond) / Lines and lifecycle]

* **M: Enforce `requiresPrerelease` for tags made by hand or by a workflow.** `gero repo tag` enforces it only for its own tags. [IMPLEMENTATION](IMPLEMENTATION.md)

- Verdict: unverified
- Evidence:

### T58  docs/TODO.md:123  [Later (v1.x and beyond) / Doctor]

* **S: Check that the token can open pull requests.**

- Verdict: unverified
- Evidence:

### T59  docs/TODO.md:124  [Later (v1.x and beyond) / Doctor]

* **S: Check access to the tracker repo.**

- Verdict: unverified
- Evidence:

### T60  docs/TODO.md:125  [Later (v1.x and beyond) / Doctor]

* **S: Check push access to each fork.**

- Verdict: unverified
- Evidence:

### T61  docs/TODO.md:126  [Later (v1.x and beyond) / Doctor]

* **S: Warn of a clone far behind its upstream.**

- Verdict: unverified
- Evidence:

### T62  docs/TODO.md:130  [Later (v1.x and beyond) / Other]

* **M: Decide which steps of `gero ship` are used alone, and name them.** The candidates are goals, not command names: send a branch, open a PR for a pushed branch, apply a release to one branch and commit it, and close superseded PRs. Decide whether each is needed when one is built. [TASK_COMMANDS](specs/TASK_COMMANDS.md#use-cases-not-yet-commands)

- Verdict: unverified
- Evidence:

### T63  docs/TODO.md:131  [Later (v1.x and beyond) / Other]

* **S: Add `--from <ref>` to `gero repo branch`.** [TASK_COMMANDS](specs/TASK_COMMANDS.md)

- Verdict: unverified
- Evidence:

### T64  docs/TODO.md:132  [Later (v1.x and beyond) / Other]

* **M: Add golden files under `testdata/` for command output,** which tests compare as strings. Optional. [TESTING](specs/TESTING.md)

- Verdict: unverified
- Evidence:

### T65  docs/TODO.md:133  [Later (v1.x and beyond) / Other]

* **L: Write the spec for the QA bridge.** [PHASES](PHASES.md)

- Verdict: unverified
- Evidence:

### T66  docs/TODO.md:134  [Later (v1.x and beyond) / Other]

* **L: Write the spec for the dashboard.** [PHASES](PHASES.md)

- Verdict: unverified
- Evidence:

### T67  docs/TODO.md:142  [Wishlist]

* **L: Infer the upstream, fork, `githubUser` and release line from the working directory,** with an `inferred` source in `gero config show`. [WORKSPACE](specs/WORKSPACE.md#wishlist)

- Verdict: unverified
- Evidence:

### T68  docs/TODO.md:143  [Wishlist]

* **L: Add a go-git client** for the git work that runs the `git` binary. The seam is there (`TODO(go-git)`). [GITHUB_ACCESS](specs/GITHUB_ACCESS.md)

- Verdict: unverified
- Evidence:

### T69  docs/TODO.md:144  [Wishlist]

* **S: Publish the JSON Schemas at a stable URL.** [APP_CONFIG](specs/APP_CONFIG.md#json-schemas)

- Verdict: unverified
- Evidence:

### T70  docs/TODO.md:145  [Wishlist]

* **S (no code): Wire the published schemas into the team's editors.** Needs the stable URL.

- Verdict: unverified
- Evidence:

### T71  docs/TODO.md:146  [Wishlist]

* **S: Add `gero doctor self-update --version`** to install a named release or roll back. [SELF_UPDATE](specs/SELF_UPDATE.md)

- Verdict: unverified
- Evidence:

### T72  docs/TODO.md:147  [Wishlist]

* **S: Add a way to give `gero doctor self-update` a `trusted_root.json` by file** for offline use. `selfupdate.Config.TrustedRoot` takes one; no flag or setting supplies it. [SELF_UPDATE](specs/SELF_UPDATE.md)

- Verdict: unverified
- Evidence:

### T73  docs/TODO.md:148  [Wishlist]

* **M: Add a `host` setting (`GERO_HOST`, `--host`) and read it in `gero repo clone`,** so the SSH URL is not github.com only (`TODO(host)`). [CLI_CONFIG](specs/CLI_CONFIG.md), [GITHUB_ACCESS](specs/GITHUB_ACCESS.md)

- Verdict: unverified
- Evidence:

### T74  docs/TODO.md:149  [Wishlist]

* **S: Let a `repo clone`'s fork have a repository name other than the upstream's** under `githubUser`. [CLI_CONFIG](specs/CLI_CONFIG.md)

- Verdict: unverified
- Evidence:
