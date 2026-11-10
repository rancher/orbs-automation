# TODO audit (working file)

A scratch file for rebuilding `docs/TODO.md` from what the code and docs say. It is deleted when the new TODO lands.

**Process:** (1) bucket every open item by source; (2) verify each against current code and accurate specs; (3) the maintainer curates onto the milestones Before V1, After V1, Before V2, Moonshot.

**Verdicts:** `real` (true and unbuilt), `done` (built; the note is stale), `moved` (the need is real, the shape changed), `question` (a decision, not work), `duplicate of <id>`, `unverified` (not yet checked).

**Groups:** A config and CLI framework specs; B workspace, doctor, setup, update, repl, testing, layout specs; C releases, tags, ship and status specs; D chain, bundle, images, metadata specs; E design, process and implementation notes; F code `TODO(...)` markers; T the current `docs/TODO.md`, kept to cross-check and not as a source.



## A. Config and CLI framework (29)

### A01  docs/specs/APP_CONFIG.md:89  [Not built yet]

* A schema for `WorkflowsConfig`, which has no Go type yet.

- Verdict: real
- Evidence: No WorkflowsConfig Go type: kinds in internal/apis/v1alpha2/meta.go:10-14 are Project/Stream/Artifact/Holding/Edit; generated/jsonschema has no schema for it.
- Shape: Add the type, then generate its schema via tools/genschema. M

### A02  docs/specs/APP_CONFIG.md:90  [Not built yet]

* Validation of a repo's `gero.yaml` (the allowed-kinds check) and of `WorkflowsConfig`.

- Verdict: real
- Evidence: No Go code reads a repo's gero.yaml; no allowed-kinds check.
- Shape: Loader for a repo's gero.yaml that rejects any kind but WorkflowsConfig. Needs A01. M

### A03  docs/specs/APP_CONFIG.md:91  [Not built yet]

* Cycle warnings across more than two projects.

- Verdict: real
- Evidence: internal/apis/v1alpha2/set.go:324-350 circular() warns once per pair of projects only.
- Shape: Cycle detection over the holding graph, keeping the rule id and acknowledgement. S-M

### A04  docs/specs/APP_CONFIG.md:92  [Not built yet]

* `gero config show --resolved`.

- Verdict: real
- Evidence: cmd/config.go:67-83 `config show` has no flags; no --resolved anywhere.
- Shape: Mode that renders the chain.Graph with defaults applied. M

### A05  docs/specs/APP_CONFIG.md:93  [Not built yet]

* `gero doctor` checking the engine's findings as `config validate` does not yet either.

- Verdict: real
- Evidence: workspace.Bump() is called only by linesresolve, ship/resolve.go:42, ship/holdings.go:107,274; doctor config check and configvalidate call only Chain().
- Shape: Call ws.Bump() in doctor config check and config validate and report its errors. S

### A06  docs/specs/APP_CONFIG.md:94  [Not built yet]

* The `chartsPackageBump` strategy, which the real config names, and writing the `dockerfile` format.

- Verdict: partly
- Evidence: chartsPackageBump registered but unbuilt (internal/bump/strategy.go:73-90, Apply returns ErrNotBuilt); `dockerfile` format accepted by schema but absent from `formats` (internal/bump/holding.go:31-35).
- Shape: Registration is done. Left: real chartsPackageBump Apply (L) and dockerfile get/set (M)

### A07  docs/specs/APP_CONFIG.md:95  [Not built yet]

* The `lifecycle-date` warning.

- Verdict: real
- Evidence: internal/apis/v1alpha2/rules.go:14-20 has five warning rule ids, none for lifecycle dates.
- Shape: Rule warning on a line with a past or absent date, with an injectable clock. S-M

### A08  docs/specs/APP_CONFIG.md:99  [Open questions]

* Final kind and field names.

- Verdict: question
- Evidence: Names are v1alpha2 (alpha); WorkflowsConfig has no type yet.
- Shape: Freeze kind and field names, naturally with A01

### A09  docs/specs/APP_CONFIG.md:100  [Open questions]

* Whether any config other than `WorkflowsConfig` ever moves into target repos.

- Verdict: question
- Evidence: No loader reads repo-side config (see A02).
- Shape: Decide whether any config besides WorkflowsConfig moves into target repos

### A10  docs/specs/CLI_CONFIG.md:156  [Not built yet]

* A `host` setting (`GERO_HOST`, `--host`) for a GitHub host other than github.com. Today the host is `GH_HOST`, else the one in `gh`'s config, else github.com ([GITHUB_ACCESS](GITHUB_ACCESS.md)).

- Verdict: real
- Evidence: No host key in internal/platform/clicfg; host comes from go-gh DefaultHost (internal/platform/github/gogh.go:16-18); --hostname only on `auth status`.
- Shape: Add a host key (GERO_HOST/--host) to clicfg, feed credentials, update user-config schema. M. Depends on A29

### A11  docs/specs/CLI_CONFIG.md:160  [Open questions]

* Final names for the user-file keys.

- Verdict: question
- Evidence: Key names in clicfg/schema.go and generated user-config.schema.json; nothing marks them final.
- Shape: Freeze or rename user-file keys

### A12  docs/specs/CLI_FRAMEWORK.md:102  [Not built yet]

* `gero workflows sync`, `gero workflows verify` and `gero ci:reconcile` are stubs that return `errs.NotImplemented`. `gero ci:ship` and `gero dashboard` are not registered. The remaining steps of `ship` are [use cases](TASK_COMMANDS.md#use-cases-not-yet-commands) without command names.

- Verdict: real
- Evidence: cmd/workflows.go:19,28 and cmd/ci/reconcile.go:13 are NotImplemented stubs; cmd/ci/ci.go registers only reconcile; no dashboard/triage.
- Shape: workflows sync/verify, ci:reconcile, ci:ship, dashboard. L overall

### A13  docs/specs/CLI_FRAMEWORK.md:103  [Not built yet]

* The exit-code helper. Until it exists, a usage or validation error exits 1, not 2.

- Verdict: real
- Evidence: cmd/root.go:59-62 Execute does os.Exit(1) for every error; no usage-error type.
- Shape: Exit-code helper mapping usage and validation errors to 2. S-M. Same need as B02

### A14  docs/specs/CLI_FRAMEWORK.md:104  [Not built yet]

* `--json` for `config validate`, and `--output env` for `auth status`.

- Verdict: real
- Evidence: configvalidate returns text only; authstatus rejects any output but text or json.
- Shape: JSON for config validate, env for auth status. S each

### A15  docs/specs/CLI_FRAMEWORK.md:108  [Open questions]

* Whether exit code 3 for drift is wanted, or `verify` output alone is enough.

- Verdict: question
- Evidence: workflows verify is a stub; no exit-3 code.
- Shape: Decide whether verify exits 3 on drift

### A16  docs/specs/CLI_FRAMEWORK.md:109  [Open questions]

* Final name for `gero dashboard` versus `triage`.

- Verdict: question
- Evidence: Command not registered.
- Shape: Decide dashboard vs triage

### A17  docs/specs/CLI_OUTPUT.md:58  [Not built yet]

* A shared yes/no confirmation in `internal/ui`: `ship`, `repo tag`, `shell install` and `doctor self-update` each ask their own.

- Verdict: real
- Evidence: Four separate y/N prompts: ship/confirm.go:59,84, repotag.go:291, shellsetup.go:148, selfupdate.go:84-86; internal/ui has no confirm.
- Shape: Extract ui.Confirm and move the four callers. M. Same need as B27

### A18  docs/specs/CLI_OUTPUT.md:59  [Not built yet]

* The final report of `ship` appended to `$GITHUB_STEP_SUMMARY` in CI, which belongs to `ci:ship`.

- Verdict: duplicate of A12
- Evidence: STEP_SUMMARY appears nowhere in code; exists only with ci:ship.
- Shape: Fold into ci:ship. S once ci:ship exists. Also see C37

### A19  docs/specs/CLI_OUTPUT.md:60  [Not built yet]

* The logger handed to every command: `ship` gets one built in `cmd/`, and the others do not log yet.

- Verdict: partly
- Evidence: cmd/settings.go:148 builds a logger used for two startup messages and not stored in the context; cmd/ship.go:89 builds a second for ship.
- Shape: One logger in the context beside Settings; ship uses it. S-M

### A20  docs/specs/CLI_OUTPUT.md:61  [Not built yet]

* The terminal and the prompt are verified by hand on a real terminal: tests run without one.

- Verdict: real
- Evidence: No pty library in go.mod; no test drives a terminal.
- Shape: Pty test harness for REPL and prompts; needs a dependency decision. M

### A21  docs/specs/DEPENDENCY_POLICY.md:40  [Open questions]

* Whether the integration sub-module needs a fake GitHub server library or hand-rolled `httptest` handlers, decided with the first integration test.

- Verdict: question
- Evidence: No test/integration module; unit tests use hand-rolled httptest and a hand-written fakehub.
- Shape: Decide fake GitHub server library vs hand-rolled handlers

### A22  docs/specs/EXECUTION_CONTEXT.md:92  [Not built yet]

* The cross-repo token check (`cigate.RequireCrossRepoToken`) and the workstation-signs detection (`clicfg.WorkstationSigns`) exist, and no command calls them yet.

- Verdict: real
- Evidence: RequireCrossRepoToken (cigate.go:49) and WorkstationSigns (clicfg/context.go:38) have no callers outside tests.
- Shape: Call them from cross-repo write commands and the preflight. S. Spot-checked

### A23  docs/specs/EXECUTION_CONTEXT.md:93  [Not built yet]

* The `auth status` fallback treats a 403 from `GET /user` as an installation token, which is unverified against a real one.

- Verdict: unverified
- Evidence: Code treats 403 as ErrNotUserToken (github/login.go:15,34); real installation-token behaviour can only be checked against GitHub.
- Shape: Manual check with a real installation token. S

### A24  docs/specs/EXECUTION_CONTEXT.md:94  [Not built yet]

* The preflight line that names workstation signs under a forced CI context.

- Verdict: real
- Evidence: WorkstationSigns has no caller; only "acting as" line in ship/human.go:230.
- Shape: Warning line naming the signs when the context is forced CI. S

### A25  docs/specs/EXECUTION_CONTEXT.md:95  [Not built yet]

* `gero repo branch` and `gero repo tag` follow the one-confirmation and `--dry-run` rules but do not print the preflight line, which only `gero ship` prints.

- Verdict: real
- Evidence: repotag.go:281-295 and repobranch.go:131 print no acting-as line; ship does (human.go:230).
- Shape: Reuse ship's identity and signing line. S-M

### A26  docs/specs/EXECUTION_CONTEXT.md:107  [Open questions]

* Whether workstation signs under a forced CI should do more than warn, for example refuse `--non-interactive` while a TTY is attached. A TTY is itself one of the signs, so this would block scripted local debugging.

- Verdict: question
- Evidence: WorkstationSigns unused; spec says warn in the preflight line.
- Shape: Decide whether signs under forced CI do more than warn

### A27  docs/specs/GITHUB_ACCESS.md:58  [Not built yet]

* The reconciler's interface over the client, and the reads and writes of issues, check runs and releases.

- Verdict: real
- Evidence: No reconciler package; cmd/ci/reconcile.go:13 is a stub.
- Shape: Consumer-side interface for the reconciler's reads and writes. L

### A28  docs/specs/GITHUB_ACCESS.md:59  [Not built yet]

* Enterprise hosts under `*.ghe.com` and `github.localhost`: `NewClient` refuses them until their API address is worked out.

- Verdict: real
- Evidence: github/client.go:28-29 returns ErrUnsupportedHost for github.localhost and *.ghe.com; rawfile.go:27-30 supports github.com only.
- Shape: API base URLs for ghe.com and github.localhost. S once decided (A29)

### A29  docs/specs/GITHUB_ACCESS.md:63  [Open questions]

* Whether we ever target hosts other than `github.com`.

- Verdict: question
- Evidence: client.go:30-32 builds an enterprise client for any other host, rawfile.go refuses all but github.com.
- Shape: Decide whether other hosts are supported or only tolerated; A10 and A28 depend on it


## B. Workspace, doctor, setup, update, repl (31)

### B01  docs/specs/WORKSPACE.md:58  [Not built yet]

* `config resolve repos --verbose` **(leaning)**: log the scanned directories, the remotes read from each clone, and why each candidate matched or not.

- Verdict: real
- Evidence: `config resolve repos` has no verbose handling; slog used only in ship/logsink.go.
- Shape: Pass a logger into configresolve and the scan. M

### B02  docs/specs/DOCTOR.md:130  [Not built yet]

* The exit-code helper: until it exists a usage error, such as an unknown check name, exits 1, not 2.

- Verdict: real
- Evidence: cmd/root.go:59-62 os.Exit(1) on any error; no exit code 2; unknown check gives doctor.ErrUnknownCheck, exit 1.
- Shape: Usage-error type mapped to exit 2. S-M. Same need as A13

### B03  docs/specs/DOCTOR.md:131  [Not built yet]

* Checks for what later features need: token permissions for opening pull requests, access to the tracker repo, push access to each fork, and clones far behind upstream.

- Verdict: real
- Evidence: No doctor check for PR-create permission, tracker access, push access or clone behind upstream.
- Shape: Add online checks; ship.Remote has BehindBy to reuse. M-L

### B04  docs/specs/DOCTOR.md:135  [Open questions]

* Whether checks should run in parallel, and when. Sequential until it hurts.

- Verdict: question
- Evidence: doctor.go:122 runs checks in a sequential loop.
- Shape: Decide: parallelise only if sequential hurts

### B05  docs/specs/DOCTOR.md:136  [Open questions]

* Whether a redacted, paste-friendly support mode is wanted (paths shortened, host names kept) or the default output is already safe enough. The output never contains a token, but it does contain local paths. Undecided; revisit when someone pastes it into an issue.

- Verdict: question
- Evidence: No redaction mode; the report prints local paths.
- Shape: Decide whether a paste-safe mode is wanted

### B06  docs/specs/SHELL_SETUP.md:54  [Not built yet]

* Windows PowerShell 5, and other shells (nushell, elvish).

- Verdict: partly
- Evidence: shellenv supports bash, zsh, fish and PowerShell 7 (shell.go:22-26); no nushell, elvish, Windows PowerShell 5.
- Shape: New Shell constant, profile path and script for each. M each

### B07  docs/specs/SHELL_SETUP.md:55  [Not built yet]

* Running the fish and PowerShell scripts in a real shell in tests.

- Verdict: real
- Evidence: No test runs the generated scripts in a real fish or pwsh; tests compare strings.
- Shape: Run scripts in real fish/pwsh, skipped when absent, installed in CI. M

### B08  docs/specs/SHELL_SETUP.md:56  [Not built yet]

* Completing the project argument of `gero cd` from the config.

- Verdict: real
- Evidence: No ValidArgsFunction anywhere; only the REPL completes projects. Spot-checked.
- Shape: ValidArgsFunction on `gero cd` reusing repl.ProjectNames. S

### B09  docs/specs/SHELL_SETUP.md:57  [Not built yet]

* A `gero doctor` check that the block is installed and current.

- Verdict: real
- Evidence: No doctor check reads the shell profile.
- Shape: Offline doctor check comparing the managed block with what `shell init` writes. S-M

### B10  docs/specs/SELF_UPDATE.md:38  [Not built yet]

* Tests of the signature itself against a real release: the tests drive the real `binup` and `ghreleases` code and the real signer pattern, with a stand-in verifier for the signature, in memory and with no network.

- Verdict: partly
- Evidence: selfupdate_test.go tests the real SignerPattern with a fake verifier and in-memory release; no test verifies a real .sigstore.json bundle.
- Shape: Test or CI step with a captured real bundle. S-M

### B11  docs/specs/SELF_UPDATE.md:39  [Not built yet]

* `--version`, to install a named release or roll back.

- Verdict: real
- Evidence: selfupdate has only --check, --yes, --pre-release; Config has no target version.
- Shape: --version flag with a downgrade guard. M

### B12  docs/specs/SELF_UPDATE.md:40  [Not built yet]

* An offline path (a `trusted_root.json` supplied by file), though `selfupdate.Config.TrustedRoot` takes one.

- Verdict: real
- Evidence: selfupdate.Config.TrustedRoot is a library field; no flag or env sets it.
- Shape: Flag or setting that reads trusted_root.json. S

### B13  docs/specs/SELF_UPDATE.md:41  [Not built yet]

* Windows: gero is released for macOS and Linux only.

- Verdict: real
- Evidence: .goreleaser.yaml builds darwin and linux only.
- Shape: Windows build, archive, running-exe replacement, signing. L; likely a deliberate non-goal, reword as such

### B14  docs/specs/SELF_UPDATE.md:45  [Open questions]

* Whether `gero doctor` should report that a newer release exists, as a warning on `--online`.

- Verdict: question
- Evidence: doctor versionCheck reports the running version only, offline.
- Shape: Decide whether to add an online newer-release warning

### B15  docs/specs/REPL.md:111  [Not built yet]

* **`help`.** It is cobra's help, which does not list `pwd`, `ls`, `clear` or `exit`, and describes the tree's `gero shell` (completion setup) where the REPL's `shell` is the child shell. A short block of the REPL's own commands before cobra's help, with `help <command>` left to cobra, is the first thing to try; a REPL-only help is the larger option.

- Verdict: real
- Evidence: repl.go:107-124 sends `help` to the cobra tree; no REPL help text.
- Shape: Short block for pwd, ls, cd, clear, shell, exit. S

### B16  docs/specs/REPL.md:112  [Not built yet]

* **An unknown name** gets cobra's unknown-command error. The intended version is one line that points to `help` and `shell`, not a full help screen.

- Verdict: real
- Evidence: repl.go:123 returns cobra's unknown-command text; no one-line hint.
- Shape: Map unknown-command to a hint pointing at help and shell. S

### B17  docs/specs/REPL.md:113  [Not built yet]

* **The prompt** has no way to show that the last line failed, and only the placeholders `{project}`, `{dir}` and `{path}`; more come when a user asks for something it cannot say.

- Verdict: question
- Evidence: prompt.go:112-115 accepts only {project}, {dir}, {path}; deferred by design.
- Shape: Add a status placeholder only if asked

### B18  docs/specs/REPL.md:114  [Not built yet]

* **`!<command line>`** and **marking the prompt of the child `shell`** ([Deferred ideas](#deferred-ideas)).

- Verdict: real
- Evidence: repl Exec has no `!` handling.
- Shape: `!<cmd>` through $SHELL -c in the session dir. S-M

### B19  docs/specs/REPL.md:120  [Deferred ideas]

* **`GERO_REPL=1` is the hook.** It is in the child's environment for this. A user can put `${GERO_REPL:+(gero) }` in their own `PS1` today.

- Verdict: done
- Evidence: shell.go:50-57 markedEnv adds GERO_REPL=1; cmd/repl_test.go:245 asserts it.
- Shape: None; the note is a statement of fact

### B20  docs/specs/REPL.md:121  [Deferred ideas]

* **Setting `PS1` on the child does not work.** `PS1` is normally a shell variable and not exported, and `~/.bashrc` or `~/.zshrc` (and oh-my-zsh, starship, powerlevel10k) set it after the shell starts, so a value from the environment is overwritten for most users. Fish and PowerShell have no `PS1`.

- Verdict: question
- Evidence: No code sets PS1; the note records why.
- Shape: Rationale only; move to a design note

### B21  docs/specs/REPL.md:122  [Deferred ideas]

* **Option, preferred if it is ever done: an opt-in snippet in the block `gero shell install` manages** ([SHELL_SETUP](SHELL_SETUP.md)), prefixing the prompt for bash and zsh, with a fish equivalent, only when `GERO_REPL` is set. A prompt framework that redraws the prompt later can still overwrite it, and for those the documented route is the framework's own environment-variable module keyed on `GERO_REPL`.

- Verdict: real
- Evidence: Nothing in shellenv emits a GERO_REPL prompt prefix.
- Shape: Opt-in prompt snippet in the managed block for bash, zsh, fish. M

### B22  docs/specs/REPL.md:123  [Deferred ideas]

* **Option, advised against: start the child with an rc file of ours** (`bash --rcfile`, a temporary `ZDOTDIR` for zsh). It would force the marker, but it is shell-specific, changes how the user's shell starts, rewrites a prompt without being asked, and breaks the rule that `shell` runs the user's `$SHELL` as is.

- Verdict: question
- Evidence: repl/shell.go runs $SHELL as-is; the advised-against option is declined.
- Shape: Settled "no"; close

### B23  docs/specs/REPL.md:129  [Open questions]

* Which phase the REPL belongs to, if any ([PHASES](../PHASES.md)). It is not scheduled.

- Verdict: question
- Evidence: `gero repl` is built; PHASES.md:37,133 list it under "The tool".
- Shape: Likely stale; confirm and close

### B24  docs/specs/TESTING.md:22  [Not built yet]

* The `test/integration` sub-module and its CI job.

- Verdict: real
- Evidence: Single go.mod, no test/ directory, no integration job. Spot-checked.
- Shape: test/integration sub-module and a CI job. M

### B25  docs/specs/TESTING.md:23  [Not built yet]

* Golden files under `testdata/` for command output, which tests compare as strings today.

- Verdict: real
- Evidence: Only testdata is internal/lifecycle/testdata; no golden files.
- Shape: Golden files and an update flag for command output tests. M

### B26  docs/specs/TESTING.md:24  [Not built yet]

* A CI run of `go test ./...` in a deliberately hostile environment, to catch a test that reads the machine.

- Verdict: partly
- Evidence: cmd/main_test.go scrubs GERO_*, GH_*, GIT_*, XDG_* and sets a temp HOME; CI runs plain `go test ./...`.
- Shape: CI job with an adversarial or empty environment. S-M

### B27  docs/specs/PROJECT_LAYOUT.md:169  [Not built yet]

* A shared yes/no confirmation in `internal/ui`: `ship`, `repo tag`, `shell install` and `doctor self-update` each ask their own `[y/N]` through the style renderer ([CLI_OUTPUT](CLI_OUTPUT.md)).

- Verdict: real
- Evidence: Four separate [y/N] prompts, none in internal/ui. Spot-checked.
- Shape: ui.Confirm and move the four callers. M. Same need as A17

### B28  docs/specs/PROJECT_LAYOUT.md:170  [Not built yet]

* A task for closing superseded PRs, extracted from `internal/ship` when it is needed, and commands for `pushbranch`, `createpr` and `bumpcommit`, which are use cases without names ([TASK_COMMANDS](TASK_COMMANDS.md#use-cases-not-yet-commands)). `gero repo branch` and `gero repo tag` run `createbranch` and `createtag` alone.

- Verdict: partly
- Evidence: Task packages exist (bumpcommit, createbranch, createpr, createtag, pushbranch); only branch and tag are task commands; no closing task.
- Shape: Commands for pushbranch/createpr/bumpcommit (S each, when needed) and extract close-superseded from ship (M)

### B29  docs/specs/PROJECT_LAYOUT.md:171  [Not built yet]

* Narrowing `ship.Git` and `ship.Remote` now that the tasks define what they call: both still carry every method any task needs.

- Verdict: real
- Evidence: ship.Git has ~22 methods, ship.Remote 13; task packages import platform/git directly.
- Shape: Split into narrower per-task interfaces. M

### B30  docs/specs/PROJECT_LAYOUT.md:172  [Not built yet]

* `test/integration/` ([TESTING](TESTING.md)).

- Verdict: duplicate of B24
- Evidence: Same as B24.

### B31  docs/specs/PROJECT_LAYOUT.md:176  [Open questions]

* Whether generated code (for example JSON schema from config types) gets its own directory.

- Verdict: question
- Evidence: generated/jsonschema is a top-level directory written by tools/genschema; no go:generate.
- Shape: Decision only matters if generated Go code ever exists


## C. Releases, tags, ship and status (43)

### C01  docs/specs/BRANCH_TAGS.md:138  [Open questions]

* **Branch families as config.** Supported ref patterns default to `main` and `release/*`, and an optional config could override them, possibly in the repo-level `gero.yaml`. [ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md) currently allows only `WorkflowsConfig` in a target repo's `gero.yaml`, so this needs a decision there first. Unconfigured behavior stays the script's.

- Verdict: question
- Evidence: No code reads a ref-pattern config; branchtags has only built-in main and release/* patterns.
- Shape: Decide whether a target repo's gero.yaml may carry branch-family config

### C02  docs/specs/BRANCH_TAGS.md:139  [Open questions]

* **Flags.** Whether `--ref`, `--sha` and `--repo <path>` are worth adding to run the command without the CI environment. Undecided; the environment rule stays the default.

- Verdict: question
- Evidence: `repo build-tags` takes no flags (cmd/repo.go:325).
- Shape: Decide whether to add --ref/--sha/--repo

### C03  docs/specs/BRANCH_TAGS.md:140  [Open questions]

* **Default local short commit.** Decided: ask the `git` binary (section 3). Revisit only if a go-git abbreviation that matches git appears, since the subprocess inherits the environment and git config.

- Verdict: done
- Evidence: gitcli.go:15-24 runs `git rev-parse --short HEAD`; decided and built.
- Shape: Decision record, not work

### C04  docs/specs/BRANCH_TAGS.md:141  [Open questions]

* **Opt-in flag names and grouping.** One flag per improvement in section 6, or a single flag that turns on the related ones.

- Verdict: question
- Evidence: No improvement flags on build-tags.
- Shape: Decide one flag per improvement vs a group flag

### C05  docs/specs/BRANCH_TAGS.md:142  [Open questions]

* **Error reporting.** The stderr row in section 6 touches a global rule ([CLI_OUTPUT](CLI_OUTPUT.md): diagnostics to stderr). Whether it still needs a per-command opt-in.

- Verdict: question
- Evidence: No per-command stderr opt-in on build-tags.
- Shape: Decide against the global CLI_OUTPUT rule

### C06  docs/specs/CHECK_SEMVER.md:115  [Open questions]

* **Opt-in flag names and grouping.** One flag per improvement in section 6, or one flag (for example for the SemVer grammar) that turns on the related ones together.

- Verdict: question
- Evidence: `semver check` has no flags.
- Shape: Same decision as C04

### C07  docs/specs/CHECK_SEMVER.md:116  [Open questions]

* **Extra env format name.** `--output env-full` or `env-extra`. Decided with the CLI output work, since it adds a value to `--output`.

- Verdict: question
- Evidence: checksemver.Render emits only two default lines; no env-full value.
- Shape: Decide the name of the extra --output value

### C08  docs/specs/CHECK_SEMVER.md:117  [Open questions]

* **Strict mode.** Whether `--strict` is worth adding (section 6).

- Verdict: question
- Evidence: No --strict on `semver check` (only on doctor).
- Shape: Decide

### C09  docs/specs/CHECK_SEMVER.md:118  [Open questions]

* **Non-SemVer `+`.** Whether the grammar mode should keep the script's "any `+`" meaning for non-SemVer text. Deferred until a real workflow depends on it.

- Verdict: question
- Evidence: semver.Classify has no grammar mode.
- Shape: Deferred by the note; decide non-SemVer `+` meaning

### C10  docs/specs/TASK_COMMANDS.md:90  [Not built yet]

* The use cases above as commands, under names not chosen.

- Verdict: real
- Evidence: `repo` has info, list, status, branch, clone, tag, images exist, build-tags; no push-branch, open-PR, bump-commit or close-superseded command. Spot-checked.
- Shape: Four use cases without a command. M each once named (C11)

### C11  docs/specs/TASK_COMMANDS.md:94  [Open questions]

* Whether each use case is wanted as a command of its own, and under what name.

- Verdict: question
- Evidence: See C10.
- Shape: Decide whether each use case is wanted as a command and its name

### C12  docs/specs/TASK_COMMANDS.md:95  [Open questions]

* Whether `repo branch` gets `--from <ref>`.

- Verdict: question
- Evidence: `repo branch` has only --switch; no From field.
- Shape: Decide whether --from <ref> is wanted

### C13  docs/specs/TASK_COMMANDS.md:96  [Open questions]

* For `repo tag`: whether the default bump after a stable is always a patch with `--minor` and `--major` for the rest, and the name `--stable` for the promotion. Whether an annotated or signed tag is ever wanted, which the API does not make.

- Verdict: partly
- Evidence: --stable/--minor/--major exist (cmd/repo.go:274-276), default patch; CreateTag makes lightweight tags only (gitcli/tag.go:12).
- Shape: Only the annotated/signed tag question is left; first half of the note is stale

### C14  docs/specs/TASK_COMMANDS.md:97  [Open questions]

* Whether a later token such as `@` should stand for the checked-out branch where a branch is a positional, which none is now.

- Verdict: question
- Evidence: No branch positional where `@` could apply.
- Shape: Decide when one exists

### C15  docs/specs/TASK_COMMANDS.md:98  [Open questions]

* Whether the push and PR use cases share one push-target resolution call or each repeats it. A shared one belongs in `internal/ship` with `resolvePushTarget` until a task needs it, then in a domain package.

- Verdict: question
- Evidence: resolvePushTarget unexported (ship/pushtarget.go:48), used once.
- Shape: Decide when a second command needs it

### C16  docs/specs/REPO_STATUS.md:53  [Not built yet]

* How many releases a branch is behind, which needs the tags between two releases and the version the strategy found (`ship.Holding`, and `git.LatestReleases` for the per-series index).

- Verdict: real
- Evidence: TODO(release-state) at ship/holdings.go:32-47 and git/latest.go:33-37; Holding has a Verdict only; no count anywhere.
- Shape: Count tags in the line's series newer than the held version; engine returns the version found. M. Pointer to git.LatestReleases should be tagscheme

### C17  docs/specs/REPO_STATUS.md:54  [Not built yet]

* The open PR for a branch, so a consumer that is behind can say a bump is already proposed (`ship.Holding`). It would be a tier of its own, with its cost, using the PR lookup and the marker `ship` uses.

- Verdict: real
- Evidence: repostatus.trace calls only ship.Holdings and ship.Sources; no PR lookup; Options has two flags.
- Shape: Opt-in tier reusing pulls.Service and ParseMarker. M

### C18  docs/specs/REPO_STATUS.md:55  [Not built yet]

* The images a release publishes, as `repo images exist` checks them (`repostatus.Options`).

- Verdict: real
- Evidence: TODO at repostatus.go:83-88; no images field; repostatus does not import internal/registry.
- Shape: Opt-in line from registry.CheckProject. S

### C19  docs/specs/REPO_STATUS.md:59  [Open questions]

* Whether a project that is both a producer and a consumer, such as one that takes from another and ships on, should show the Sources and Consumers sections together or only the one asked for.

- Verdict: question
- Evidence: trace shows Sources and Consumers by default, each skippable.
- Shape: Decide whether both-by-default is final; probably record as is

### C20  docs/specs/LINES_RESOLVE.md:35  [Not built yet]

* Following the chain beyond one hop, such as `ob-team-charts` into `charts` into `rancher` (`also` lists the project and its own links today; it does not connect them back to the hub).

- Verdict: real
- Evidence: linesresolve.go:269-280 builds Also from a project's own links; no multi-hop walk.
- Shape: Hub-to-charts-to-assets walk. L

### C21  docs/specs/LINES_RESOLVE.md:36  [Not built yet]

* A line the lines document has no entry for, but a project has: validation rejects it when the config is loaded, so there is no gap for it here.

- Verdict: done
- Evidence: Validation rejects it at load (v1alpha2/set.go:227).
- Shape: Note explains a non-gap; close

### C22  docs/specs/LINES_RESOLVE.md:37  [Not built yet]

* The answer for one producer and one line, "where does shell's `v2.15` go", which is a different command and not this map.

- Verdict: question
- Evidence: `lines resolve` takes [line...], --hub, --phase; no "where does shell's v2.15 go" command.
- Shape: Decide whether wanted

### C23  docs/specs/LINES_RESOLVE.md:41  [Open questions]

* How the chain through `rancher/charts` connects back to the hub and leaves `also`: the holdings of Assets in Rancher and of charts in Assets are in the config, and following them is the walk of [CHAIN_TRACING](CHAIN_TRACING.md), which is not built.

- Verdict: duplicate of C20
- Evidence: Same chain walk.

### C24  docs/specs/PUSH_TO_REPO.md:188  [Not built yet]

* A test of the push against a real scratch repository in the integration tier ([TESTING](TESTING.md)), which would keep the credential questions in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#push-credentials-and-signing) settled by a test. `gitcli` has a spike test that runs only when `GERO_SPIKE_PUSH_URL` names one.

- Verdict: real
- Evidence: Spike push test skipped unless GERO_SPIKE_PUSH_URL; no integration tier, no build tags.
- Shape: Integration-tier push test. M

### C25  docs/specs/PUSH_TO_REPO.md:189  [Not built yet]

* A way to name a step's commit, or fold it into the previous one, which is the first knob to add if a project needs it.

- Verdict: real
- Evidence: Step has only run, since, until, from; commit message is a fixed template.
- Shape: Config knob on Step if a project needs it. S-M

### C26  docs/specs/PUSH_TO_REPO.md:190  [Not built yet]

* `gero ci:ship` ([CI_SHIP](CI_SHIP.md)).

- Verdict: duplicate of C34
- Evidence: cmd/ci registers only the reconcile stub.

### C27  docs/specs/PUSH_TO_REPO.md:191  [Not built yet]

* The sequence above as chained tasks. Making the branch is the `createbranch` task, the edit and its commits are `bumpcommit`, pushing is `pushbranch` and opening and labelling the PR is `createpr`; the other steps work today inside `internal/ship`, and moving each to `internal/task/` changes no behavior and none of this spec, apart from this list ([PROJECT_LAYOUT](PROJECT_LAYOUT.md#tasks-single-actions-that-compose)).

- Verdict: partly
- Evidence: Four tasks exist and ship uses them; closing superseded PRs, plan, preview, publish still live in internal/ship.
- Shape: Move remaining steps into internal/task. M, behaviour-neutral

### C28  docs/specs/PUSH_TO_REPO.md:192  [Not built yet]

* `from: producer` and `from: central` for steps, which need the producer's checkout and the bundle.

- Verdict: real
- Evidence: `from: producer
- Shape: central` validated but steprun refuses with ErrFromUnsupported (steprun.go:57-58). Spot-checked.|Run a script from the producer checkout or the bundle. M

### C29  docs/specs/PUSH_TO_REPO.md:196  [Open questions]

* Whether the default PR title should carry something that identifies the tooling. Identity is the marker, the author and, in shared repositories, a label such as `origin/bot`; a title is a project's own convention, and matching on it is fragile.

- Verdict: question
- Evidence: Title from prs.Title or templates.DefaultTitle() (ship/preview.go:73).
- Shape: Decide whether the default title identifies the tooling

### C30  docs/specs/PUSH_TO_REPO.md:197  [Open questions]

* How steps such as `go generate` get a toolchain that matches the target branch. A workstation without it fails with a `gero doctor` hint.

- Verdict: question
- Evidence: No toolchain selection; steps run with the machine's tools.
- Shape: Decide

### C31  docs/specs/PUSH_TO_REPO.md:198  [Open questions]

* The shape of `gero ci:ship`, in [CI_SHIP](CI_SHIP.md#open-questions).

- Verdict: duplicate of C40
- Evidence: ci:ship shape is the CI_SHIP open questions.

### C32  docs/specs/PUSH_TO_REPO.md:199  [Open questions]

* Whether a clean clone with untracked files should be worked in place when none collide, as designed here, or sent to a worktree whenever there are any.

- Verdict: question
- Evidence: Code settled it: in place when nothing collides, else a worktree (createbranch.go:96-117).
- Shape: Close as decided unless reversing

### C33  docs/specs/PUSH_TO_REPO.md:200  [Open questions]

* How a strategy reads the value it wrote for the downgrade check. A holding reads its own value, and `chartsPackageBump` is not built.

- Verdict: question
- Evidence: chartsPackageBump unbuilt; bump.Result has only Changed and Changes.
- Shape: Decide how a strategy reports the value it wrote

### C34  docs/specs/CI_SHIP.md:35  [Not built yet]

* The command, its `cmd/` registration (CI-only, [CLI_FRAMEWORK](CLI_FRAMEWORK.md)) and its loop.

- Verdict: real
- Evidence: cmd/ci/ci.go registers only newReconcileCmd(). Spot-checked.
- Shape: Register ship in the CI list and run the bump loop as the bot. L

### C35  docs/specs/CI_SHIP.md:36  [Not built yet]

* Resolving the bot's identity in a workflow: the domain passes it to the commits, and `GERO_BOT_USER` with the id lookup is tested without a real bot.

- Verdict: partly
- Evidence: actor.Bot/Resolve, GERO_BOT_USER and identity.ResolveIn built and tested; no CI command calls them.
- Shape: Resolution is done; wiring belongs to C34

### C36  docs/specs/CI_SHIP.md:37  [Not built yet]

* The temporary clone as a `Workspace`, and the token as a one-shot credential for the push (`gitcli.Push` and `gitcli.PushTag` take the engineer's own today).

- Verdict: real
- Evidence: gitcli push uses the engineer's SSH agent and helper; no token credential for push; no temp-clone Workspace for CI.
- Shape: CI temp-clone Workspace and a one-shot token credential. M

### C37  docs/specs/CI_SHIP.md:38  [Not built yet]

* The markdown step summary, and the workflow run URL in the PR body.

- Verdict: real
- Evidence: Nothing writes STEP_SUMMARY or a run URL into the PR body.
- Shape: Write the summary and add a run URL to the PR template. S

### C38  docs/specs/CI_SHIP.md:39  [Not built yet]

* `from: producer` and `from: central` for steps.

- Verdict: duplicate of C28
- Evidence: Same ErrFromUnsupported refusal.

### C39  docs/specs/CI_SHIP.md:40  [Not built yet]

* A real workflow that calls it, beside the bash and then in place of it.

- Verdict: real
- Evidence: No ci:ship, so no workflow can call it; .github not checked for the bash workflows.
- Shape: Workflows in scc-operator and shell once the command exists. M

### C40  docs/specs/CI_SHIP.md:44  [Open questions]

* **Who makes the clone.** Either `gero` does its own setup (a shallow temporary clone with the workflow's token, the bot identity resolved, a push to the upstream), or workflow steps check out the consumer and hand `gero` the directory. Both are one more `Workspace`, and only the setup differs. The first keeps the workflow to one step and tests the same code in both contexts. The second keeps checkout and authentication in the actions the workflow already uses, and needs no clone code. Decide when the first workflow is written.

- Verdict: question
- Evidence: No ci:ship or CI clone code.
- Shape: Decide: gero's own temp clone or a directory handed in

### C41  docs/specs/CI_SHIP.md:45  [Open questions]

* Whether `ci:ship` fans out across lines in one run, as `gero ship` does, or a workflow runs it once for each line it is given.

- Verdict: question
- Evidence: No ci:ship; gero ship fans out over lines.
- Shape: Decide whether ci:ship does too

### C42  docs/specs/CI_SHIP.md:46  [Open questions]

* How two runs of the same bump at once are made safe. The branch name is deterministic and a push replaces only a branch it saw, so the second push is rejected, but what that run reports and whether it retries is not decided.

- Verdict: question
- Evidence: PushReplacing leases on the commit seen; rejection built (ErrPushRejected); reporting and retry undecided.
- Shape: Decide reporting and retry

### C43  docs/specs/CI_SHIP.md:47  [Open questions]

* Whether the step summary and the PR comments are the same text, or the summary is longer.

- Verdict: question
- Evidence: No step summary exists (C37).
- Shape: Decide whether summary and PR comments carry the same text


## D. Chain, bundle, images, metadata (26)

### D01  docs/specs/CHAIN_TRACING.md:85  [Not built yet]

* Reading a commit or a branch holding at a ref, which `assets-pins-charts` and `rancher-follows-charts-branch` need, and the `lock.yaml` read built on it ([UPDATE_ENGINE](../design/UPDATE_ENGINE.md)).

- Verdict: partly
- Evidence: Holdings modelled (rancher-assets.yaml:30, rancher.yaml:77); bump.NewEngine skips non-release holdings (engine.go:75); `dockerfile` not in formats.
- Shape: Read-only reader for commit and branch holdings at a ref, plus a dockerfile format. M

### D02  docs/specs/CHAIN_TRACING.md:86  [Not built yet]

* The walk through several hops, the tree in `lines resolve` and `--depth`.

- Verdict: real
- Evidence: No --depth, no walk package; linesresolve builds flat links.
- Shape: Cycle-safe walk, reader interface, tree output, --depth. L

### D03  docs/specs/CHAIN_TRACING.md:87  [Not built yet]

* Checking a held tag's stage against the release team's rule.

- Verdict: real
- Evidence: No code compares lock.yaml at two commits or judges a held tag's stage.
- Shape: Depends on D01. M

### D04  docs/specs/CHAIN_TRACING.md:88  [Not built yet]

* `chartsPackageBump` and BRO's tag into a charts bump.

- Verdict: partly
- Evidence: chartsPackageBump is a stub (bump/strategy.go:64-100); BRO tag to chart version mapping unbuilt.
- Shape: Real Apply plus the mapping. L

### D05  docs/specs/CHAIN_TRACING.md:101  [Open questions]

* The key in Rancher's `build.yaml` that names the charts branch before v2.16, and whether it holds the branch name alone.

- Verdict: moved
- Evidence: config/rancher.yaml:77-82 puts the pre-v2.16 holding at package/Dockerfile key CHART_DEFAULT_BRANCH, not build.yaml. Spot-checked.
- Shape: Confirm the value is the bare branch name and which file holds the key for v2.16+. S

### D06  docs/specs/CHAIN_TRACING.md:102  [Open questions]

* How a pinned commit in `lock.yaml` is read: a small read-only helper over the key-path code, or something else, and what "release" a pin is compared with (the head of the stage's branch). The key path needs a quoting rule for a key with a dot, such as the line `2.16`.

- Verdict: partly
- Evidence: Quoting of `."2.16"` keys is built (yamledit); pin read at a ref and compare with the branch head are not.
- Shape: Pin read at a ref and the compare. M

### D07  docs/specs/CHAIN_TRACING.md:103  [Open questions]

* Whether `since` and `until` on a holding is all the line-range a holding needs.

- Verdict: question
- Evidence: Holding lines are a {since, until} range (common.go:10-11).
- Shape: Decide whether a range is enough or a list/gap is needed

### D08  docs/specs/CHAIN_TRACING.md:104  [Open questions]

* How BRO's tag maps to the chart version in `release.yaml` (`110.0.3+up11.0.4`), and whether that rule is written down by the release team or only encoded in the scripts that make it.

- Verdict: question
- Evidence: No code for the BRO tag to chart version rule; CONFIG_SCHEMA.md:142 defines only the +up split.
- Shape: Ask the release team whether the rule is written down

### D09  docs/specs/CHAIN_TRACING.md:105  [Open questions]

* Whether an Assets tag's charts commit is enough to answer "is this chart version in this tag", which is traced through `lock.yaml` at that tag to the contents of `rancher/charts` at the commit it records.

- Verdict: question
- Evidence: No code traces an Assets tag through lock.yaml into charts content; depends on D01 and D06.
- Shape: Decide whether the commit alone answers it

### D10  docs/specs/CHAIN_TRACING.md:106  [Open questions]

* How a charts line differs in how it is bumped beyond the steps that `since` and `until` already vary.

- Verdict: question
- Evidence: chartsPackageBump is a stub, so no line variation is encoded.
- Shape: Answerable only when D04 is built

### D11  docs/specs/CENTRAL_BUNDLE.md:101  [Not built yet]

* The newer of a local copy and the bundle: in `auto` a local source wins, and `--config-source` picks one or the other.

- Verdict: real
- Evidence: workspace/bundle.go:47-65 chooses by precedence; no freshness comparison.
- Shape: Compare a local copy with the bundle or drop the idea. S, mostly a decision

### D12  docs/specs/CENTRAL_BUNDLE.md:102  [Not built yet]

* Conditional requests (`ghreleases` has none), so the daily refresh is by the age of the cache alone.

- Verdict: real
- Evidence: No ETag or If-None-Match; refresh is by cache age (bundlestore.go DefaultRefresh = 24h).
- Shape: A stated limitation unless ghreleases adds support. S

### D13  docs/specs/CENTRAL_BUNDLE.md:103  [Not built yet]

* A refresh interval setting. Reproducing an old run needs no more: download that release's bundle and give it with `--bundle-file`.

- Verdict: partly
- Evidence: Resolver.Refresh exists (bundlestore.go:41); no setting is wired (nothing in clicfg or workspace/bundle.go). Spot-checked.
- Shape: Plumb a bundleRefresh setting to Resolver.Refresh. S

### D14  docs/specs/CENTRAL_BUNDLE.md:104  [Not built yet]

* The templates and generated chunks.

- Verdict: real
- Evidence: workflows sync/verify are stubs (cmd/workflows.go:19,28); bundle has only header and config chunk. Spot-checked.
- Shape: Template and generated chunks and loader after workflows sync. L

### D15  docs/specs/CENTRAL_BUNDLE.md:108  [Open questions]

* What `ghreleases` does about the REST API's unauthenticated rate limit (60 requests an hour per IP, shared by CI runners and office networks). If it uses the API, the daily refresh limit matters more.

- Verdict: partly
- Evidence: ErrRateLimited mapped (assetfetch.go:28), reported, cache used as offline fallback; ghreleases internals not read.
- Shape: Question left is whether 60 requests an hour is enough in CI. S

### D16  docs/specs/CENTRAL_BUNDLE.md:109  [Open questions]

* Whether `centralConfigDir`, which names the central repo's `config/` directory, should name the whole central repo once the bundle holds templates as well as config ([CLI_CONFIG](CLI_CONFIG.md)).

- Verdict: question
- Evidence: centralConfigDir is still the config directory; bundle has no templates yet.
- Shape: Decide rename or widen once templates ship. Depends on D14

### D17  docs/specs/CENTRAL_BUNDLE.md:110  [Open questions]

* How `gero ci:*` commands in other repos get config. Today the bundle is never used in CI, which sets `GERO_CENTRAL_CONFIG_DIR` to a checkout of the central repo. Whether a job without a checkout should use the bundle of the installed release is not decided.

- Verdict: question
- Evidence: bundleFor returns no bundle in CI unless --config-source=bundle; CI workflows set GERO_CENTRAL_CONFIG_DIR.
- Shape: Decide whether the CI default should change

### D18  docs/specs/CENTRAL_BUNDLE.md:111  [Open questions]

* Whether the refresh interval is a setting.

- Verdict: duplicate of D13
- Evidence: Same: expose Resolver.Refresh as a setting.
- Shape: D13 is the work, D18 the decision

### D19  docs/specs/IMAGES_EXIST.md:67  [Open questions]

* Whether a registry that ignores `HEAD` needs a `GET` fallback. The three here are expected to answer `HEAD`.

- Verdict: question
- Evidence: OCI client uses only HEAD (client.go:92). Spot-checked.
- Shape: Decide only if a registry ignores HEAD

### D20  docs/specs/IMAGES_EXIST.md:68  [Open questions]

* Whether the check should compare digests across registries, which the script does not do.

- Verdict: question
- Evidence: No digest handling; the check is existence only.
- Shape: Design decision

### D21  docs/specs/IMAGES_EXIST.md:69  [Open questions]

* Whether a mirror's timeout and retry policy needs to be a setting.

- Verdict: question
- Evidence: Timeout hardcoded 20s (client.go:58); no retry beyond the token retry.
- Shape: Decide whether timeout and retry are settings

### D22  docs/specs/METADATA.md:82  [Not built yet]

* The gero-owned labels (`gero/stale`, `gero/superseded`), their creation on demand and their reconciliation.

- Verdict: real
- Evidence: No gero/stale or gero/superseded labels; label code only adds existing ones.
- Shape: Needs the reconciler and a label-creating service. L

### D23  docs/specs/METADATA.md:83  [Not built yet]

* The control labels (`gero/hold`) and the check of one before work.

- Verdict: real
- Evidence: No gero/hold check in ship or ci.
- Shape: Label read and check before work, warning on workstation commands. M

### D24  docs/specs/METADATA.md:84  [Not built yet]

* Labels on tracking issues, which wait for the reconciler.

- Verdict: real
- Evidence: No tracking-issue or reconciler code.
- Shape: Waits for the reconciler. L

### D25  docs/specs/METADATA.md:88  [Open questions]

* Whether a `pullRequests` block can name labels per class, or a global config injects the existing Rancher labels.

- Verdict: question
- Evidence: Labels are per project (pullRequests.labels, botLabels); no per-class split.
- Shape: Decide whether per-class labels are needed

### D26  docs/specs/METADATA.md:89  [Open questions]

* The full gero-owned vocabulary, such as phase labels on tracking issues.

- Verdict: question
- Evidence: No gero/ label exists.
- Shape: Vocabulary decision, depends on D24


## E. Design, process and implementation notes (13)

### E01  docs/design/TOOLING_GOALS.md:82  [Open]

* A chart version holds its upstream version after `+up`; the stable test reads that part

- Verdict: partly
- Evidence: rancher-charts.yaml:26 sets scheme chartVersion; tagscheme.ForStream returns "scheme chartVersion is not built"; nothing reads +up. Spot-checked.
- Shape: chartVersion Scheme in tagscheme (Valid, LineOf, Latest, IsStable, Compare). M; the decision is made

### E02  docs/design/TOOLING_GOALS.md:83  [Open]

* A component with no tracing config: how a view names it as not traced

- Verdict: question
- Evidence: No readiness view in code.
- Shape: Decide when the view is designed

### E03  docs/design/TOOLING_GOALS.md:84  [Open]

* A CVE stable skips the RC; readiness only looks at what is held, so this is not an exception here

- Verdict: question
- Evidence: The note says it is not an exception; no code treats CVE stables specially.
- Shape: Close as a design statement

### E04  docs/process/RELEASE_PROCESS.md:115  [Open]

* Who adds a line in rancher-assets

- Verdict: question
- Evidence: rancher-assets.yaml says managedBy: release-team; nothing names who adds a line.
- Shape: Release team has to say

### E05  docs/IMPLEMENTATION.md:101  [Open Questions]

* Scope and order of the milestones: see [PHASES](PHASES.md).

- Verdict: question
- Evidence: A pointer to PHASES.md.
- Shape: Stale as a question; cross-reference

### E06  docs/IMPLEMENTATION.md:102  [Open Questions]

* Config-shape questions (final names, whether config moves into target repos): see [APP_CONFIG](specs/APP_CONFIG.md). CLI-layer questions are in the spec each belongs to ([CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md), [DEPENDENCY_POLICY](specs/DEPENDENCY_POLICY.md)).

- Verdict: question
- Evidence: Pointer to specs; kind is still gero/v1alpha2.
- Shape: Rename part duplicates T17

### E07  docs/IMPLEMENTATION.md:103  [Open Questions]

* Reconciler triggering: cron only, or `repository_dispatch` where needed.

- Verdict: question
- Evidence: ci:reconcile is a stub; no repository_dispatch anywhere.
- Shape: Decide before the reconciler is built

### E08  docs/IMPLEMENTATION.md:104  [Open Questions]

* Which existing workflows to standardize first, and whether the central repo publishes them as reusable workflows or composite actions.

- Verdict: partly
- Evidence: ci.yml:4 has workflow_call; workflows sync/verify are stubs.
- Shape: Decide reusable workflow vs composite action and the first workflow (S), then build (L)

### E09  docs/IMPLEMENTATION.md:105  [Open Questions]

* Whether the CLI replaces the existing "push to X" scripts or wraps them at first (decided before v1.0.0, see [PHASES](PHASES.md)).

- Verdict: question
- Evidence: PHASES.md:14,53-57 give the policy; ci:ship not registered.
- Shape: Close as decided: wrap, then replace; confirm

### E10  docs/IMPLEMENTATION.md:106  [Open Questions]

* How new Rancher lines are cut and registered in the line map (manual edit, or a command that updates it). Updating the dates of lines already in the map is planned as `gero lines sync`; a new line also needs a Kubernetes range, which the lifecycle page does not give.

- Verdict: partly
- Evidence: `gero lines` has only `resolve`; no sync; internal/lifecycle parses the page.
- Shape: Build `gero lines sync` (M); a new line is a hand edit today. Spot-checked

### E11  docs/IMPLEMENTATION.md:107  [Open Questions]

* How downstream repos pin and call reusable workflows; resolve when we review a repo that consumes the existing core branches.

- Verdict: question
- Evidence: workflows sync/verify are stubs; no code pins a reusable workflow downstream.
- Shape: Decide when a downstream repo is reviewed

### E12  docs/IMPLEMENTATION.md:108  [Open Questions]

* The `VERSION.md` format: one standard layout across repos, which fields it shows (branch, tag, Rancher line, Kubernetes range, EOL section), and whether other tools that read `VERSION.md` today (for example frameworks-automation reads per-repo `VERSION.md` at runtime) need the old formats to keep working.

- Verdict: partly
- Evidence: No VERSION.md code at all.
- Shape: Decide layout and old readers, then build generation. M

### E13  docs/IMPLEMENTATION.md:109  [Open Questions]

* The "branch for X" workflow itself: which steps `gero` automates (create branches, update `branches.lines`, regenerate `VERSION.md`, re-render workflows) and in what order.

- Verdict: partly
- Evidence: `gero repo branch` is built; no branches.lines update, no VERSION.md, workflow render is a stub.
- Shape: Decide the order, then compose. L; depends on E10, E12


## F. Code TODO markers (5)

### F01  internal/platform/git/gitcli/clone.go:15  [code marker]

`// not, and it must run the same global config and hooks. TODO(go-git): revisit`

- Verdict: question
- Evidence: gitcli/clone.go:12-16 comment is accurate; go-git is used for reads only.
- Shape: A revisit marker, not work

### F02  internal/platform/git/latest.go:33  [code marker]

`// TODO(release-state): this answers "what is newest" for one line or project.`

- Verdict: moved
- Evidence: git/latest.go:33-37 comment is accurate but points at the wrong place: newest-tag logic now lives in tagscheme.
- Shape: Per-series index and tags-since-a-version belong in tagscheme. Fix the comment (S); the work is C16

### F03  internal/cli/repoclone/repoclone.go:221  [code marker]

`// TODO(host): github.com only; the host of a GitHub Enterprise Server is not read.`

- Verdict: real
- Evidence: sshURL hard-codes git@github.com (repoclone.go:221-222). Spot-checked.
- Shape: Add a host setting and use it in sshURL. M. Same need as A10

### F04  internal/cli/repostatus/repostatus.go:83  [code marker]

`// TODO(release-state): a tier for the images a release publishes (the fourth`

- Verdict: real
- Evidence: repostatus.go:83-89; no images tier.
- Shape: Opt-in Images option calling registry.CheckProject. S-M. Same need as C18

### F05  internal/ship/holdings.go:32  [code marker]

`// TODO(release-state): two things gero repo status cannot say yet, both in`

- Verdict: real
- Evidence: ship/holdings.go:32-47; no count behind, no open-PR tier; its pointer to git.LatestReleases is stale.
- Shape: Count behind (C16) and open bump PR tier (C17)


## T. Current docs/TODO.md (cross-check only) (74)

### T01  docs/TODO.md:15  [Needed for v1.0.0 / Ship from CI]

* **L: Build the `gero ci:ship` command.** Register it as CI-only and run the bump loop of `gero ship` as the bot: the bot identity from `GERO_BOT_USER`, and `cigate.RequireCrossRepoToken` called before work. Done when a test drives it end to end against fakes.

- Verdict: real
- Evidence: Same as C34 (and C35: the bot identity is built, only the CI command is missing).

### T02  docs/TODO.md:16  [Needed for v1.0.0 / Ship from CI]

* **M: Push with the workflow's token from a temporary clone.** The clone is a `Workspace` and the token is a one-shot credential for `gitcli.Push` and `PushTag`. Done when `ci:ship` pushes without the engineer's own credentials.

- Verdict: real
- Evidence: Same as C36.

### T03  docs/TODO.md:17  [Needed for v1.0.0 / Ship from CI]

* **S: Write the markdown step summary and put the workflow run URL in the PR body** of a `ci:ship` run.

- Verdict: real
- Evidence: Same as C37 and A18.

### T04  docs/TODO.md:18  [Needed for v1.0.0 / Ship from CI]

* **M: Run steps `from: producer` and `from: central`.** `steprun` refuses both. Done when a step can run a script of the producer's checkout or one from the bundle, in `ci:ship` only. [APP_CONFIG](specs/APP_CONFIG.md)

- Verdict: real
- Evidence: Same as C28 and C38.

### T05  docs/TODO.md:19  [Needed for v1.0.0 / Ship from CI]

* **M: Let `ci:*` commands run without a checkout of the central config.** Both CI workflows point `GERO_CENTRAL_CONFIG_DIR` at a checkout. Decide whether a job may use the bundle of the installed release, and build it. [CENTRAL_BUNDLE](specs/CENTRAL_BUNDLE.md)

- Verdict: partly
- Evidence: Same as D17: a CI job can already choose the bundle with --config-source=bundle; the open part is the default and the decision.
- Shape: Reword: decide the CI default, not "build it"

### T06  docs/TODO.md:20  [Needed for v1.0.0 / Ship from CI]

* **M: Add a workflow to `scc-operator` and `shell` that installs a pinned, signed `gero` and runs `ci:ship`,** beside the bash script, then in place of it. Needs `ci:ship`.

- Verdict: real
- Evidence: Same as C39; needs ci:ship (T01).

### T07  docs/TODO.md:21  [Needed for v1.0.0 / Ship from CI]

* **S (needs a real run): Verify the `auth status` 403 path against a real installation token.** It is tested with fakes only. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)

- Verdict: unverified
- Evidence: Same as A23; needs a real installation token, so it cannot be checked from code.

### T08  docs/TODO.md:27  [Needed for v1.0.0 / The chain]

* **S (no code): Answer the open questions of [CHAIN_TRACING](specs/CHAIN_TRACING.md)** with the release team: the Rancher key for the charts branch before v2.16, how to read the commit pinned in `lock.yaml`, and how a BRO tag maps to a chart version. Done when each answer is in the spec. The two items below need them.

- Verdict: question
- Evidence: Same as D05, D08, D09: all are questions for the release team, no code settles them. D05 is partly answered by config/rancher.yaml:77-82.

### T09  docs/TODO.md:28  [Needed for v1.0.0 / The chain]

* **M: Read a commit or a branch that a repo holds at a ref.** The config models both kinds and validates them; `resolveHolding` handles only releases. Includes the `lock.yaml` read. Done when `gero repo status` and `gero lines resolve` report what such a holding holds. [CHAIN_TRACING](specs/CHAIN_TRACING.md), [UPDATE_ENGINE](design/UPDATE_ENGINE.md)

- Verdict: real
- Evidence: Same as D01 and D06 (the pin read).

### T10  docs/TODO.md:29  [Needed for v1.0.0 / The chain]

* **S: Let the update engine read and write the `dockerfile` format.** It returns `ErrFormatNotBuilt`. Done when the refusal test is replaced by a read and write test. [UPDATE_ENGINE](design/UPDATE_ENGINE.md)

- Verdict: real
- Evidence: Same as A06 and D01: `dockerfile` absent from the formats map.

### T11  docs/TODO.md:30  [Needed for v1.0.0 / The chain]

* **L: Walk several hops in `gero lines resolve`,** with the tree and `--depth`. Needs the reader above. [LINES_RESOLVE](specs/LINES_RESOLVE.md)

- Verdict: real
- Evidence: Same as D02 and C20.

### T12  docs/TODO.md:31  [Needed for v1.0.0 / The chain]

* **L: Build the `chartsPackageBump` strategy,** so BRO's tag becomes a charts bump. It is registered as a stub that stops a run before it writes. Done when the stub test is replaced by a test of a real bump. [CHAIN_TRACING](specs/CHAIN_TRACING.md)

- Verdict: real
- Evidence: Same as D04 and A06.

### T13  docs/TODO.md:32  [Needed for v1.0.0 / The chain]

* **M: Check a held tag's stage against the release team's rule.** Done when a mismatch is reported by `gero repo status` or the walk. [CHAIN_TRACING](specs/CHAIN_TRACING.md)

- Verdict: real
- Evidence: Same as D03.

### T14  docs/TODO.md:33  [Needed for v1.0.0 / The chain]

* **S: Make `gero config validate` and `gero doctor` build the update engine and report its errors,** so a strategy that cannot run fails there and not at `ship`. [APP_CONFIG](specs/APP_CONFIG.md)

- Verdict: real
- Evidence: Same as A05: doctor and config validate call Chain() only.

### T15  docs/TODO.md:34  [Needed for v1.0.0 / The chain]

* **S (no code): Check the config against the repos.** Confirm charts' stable branch names (`release-v2.X`) and find the package of `ob-team-charts`, which is left out until it is known.

- Verdict: unverified
- Evidence: No source item and no code to check; it is a comparison of config with the real repos.

### T16  docs/TODO.md:38  [Needed for v1.0.0 / Confidence in the foundation]

* **M (no code): Get the process docs reviewed by the release team.** All of them are still drafts. Done when each says reviewed, with a date and the reviewer. [process](process/README.md)

- Verdict: real
- Evidence: docs/process/*.md are all `draft` or `staging` (grep of Status lines); none says reviewed.

### T17  docs/TODO.md:39  [Needed for v1.0.0 / Confidence in the foundation]

* **M: Rename the config kind from `gero/v1alpha2` to `gero/v1`** once the field names are settled. Done when the schemas, `config/`, the tests and [APP_CONFIG](specs/APP_CONFIG.md) agree.

- Verdict: question
- Evidence: Same as A08 and E06: names are still v1alpha2 and the freeze is a decision.

### T18  docs/TODO.md:40  [Needed for v1.0.0 / Confidence in the foundation]

* **S: Add the exit-code helper,** so a usage or validation error exits 2. Done when a typed error maps to 2 in `Execute` and each command class has a test. [CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md), [DOCTOR](specs/DOCTOR.md)

- Verdict: real
- Evidence: Same as A13 and B02.

### T19  docs/TODO.md:41  [Needed for v1.0.0 / Confidence in the foundation]

* **M: Add the integration test tier,** a `test/integration` module that runs the built binary, and a CI job for it. Done when one black-box test runs and the module adds nothing to the root `go.mod`. [TESTING](specs/TESTING.md)

- Verdict: real
- Evidence: Same as B24 and A21: single go.mod, no test/ directory.

### T20  docs/TODO.md:42  [Needed for v1.0.0 / Confidence in the foundation]

* **M: Add an integration test that pushes to a scratch repository.** Needs the tier. [PUSH_TO_REPO](specs/PUSH_TO_REPO.md)

- Verdict: real
- Evidence: Same as C24.

### T21  docs/TODO.md:43  [Needed for v1.0.0 / Confidence in the foundation]

* **S: Run `go test ./...` once in CI with a hostile environment** (a junk `HOME`, `GERO_*`, `GITHUB_TOKEN`, `XDG_*`). Done when a test that reads the machine fails the job. [TESTING](specs/TESTING.md)

- Verdict: partly
- Evidence: Same as B26: cmd/main_test.go already scrubs the environment; no CI job runs a hostile one.

### T22  docs/TODO.md:44  [Needed for v1.0.0 / Confidence in the foundation]

* **S: Add a Go test that validates every file of `config/` against the JSON Schemas.** Needs a validator module. [APP_CONFIG](specs/APP_CONFIG.md#json-schemas)

- Verdict: real
- Evidence: No test validates config/ against the JSON Schemas; go.mod has only invopop/jsonschema (a generator), no validator.

### T23  docs/TODO.md:45  [Needed for v1.0.0 / Confidence in the foundation]

* **S (needs a real run): Confirm a release end to end:** `cosign` runs in the `ci-image/go1.27` container, the bundles land on the release, and `gero doctor self-update` installs it. [SELF_UPDATE](specs/SELF_UPDATE.md)

- Verdict: unverified
- Evidence: Needs a real release run; cannot be checked from code.

### T24  docs/TODO.md:46  [Needed for v1.0.0 / Confidence in the foundation]

* **S (needs a real run): Confirm a pushed tag starts the release workflow** of a real repo, for `gero repo tag --push`. [TASK_COMMANDS](specs/TASK_COMMANDS.md)

- Verdict: unverified
- Evidence: Needs a real repo and workflow; cannot be checked from code.

### T25  docs/TODO.md:47  [Needed for v1.0.0 / Confidence in the foundation]

* **S (no code): Correct the specs that no longer match the code.** `REPO_STATUS` names a `TODO(releasetag)` marker that does not exist. `BRANCH_TAGS` lists the default short commit as open, and the code settles it. `PROJECT_LAYOUT` asks where generated code goes, and `generated/` and `tools/` answer it. `CENTRAL_BUNDLE` asks about rate limits, and `bundlestore` reports `ErrRateLimited`.

- Verdict: real
- Evidence: REPO_STATUS.md:51 still names TODO(releasetag) and no such marker exists in code (markers are release-state, go-git, host); BRANCH_TAGS lists the short commit as open and C03 says built; B31 and D15 agree PROJECT_LAYOUT and CENTRAL_BUNDLE questions are settled.

### T26  docs/TODO.md:55  [Good places to start / Commands]

* **S: Make `gero doctor --online` warn when a newer `gero` release exists.** Done when a stand-in releases source drives a test. [SELF_UPDATE](specs/SELF_UPDATE.md)

- Verdict: question
- Evidence: Same as B14: nothing built; whether to add it is a decision.

### T27  docs/TODO.md:56  [Good places to start / Commands]

* **S: Add `--json` to `gero config validate`,** a list of problems with the same exit code as text. [CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md)

- Verdict: real
- Evidence: Same as A14.

### T28  docs/TODO.md:57  [Good places to start / Commands]

* **S: Make `gero config resolve repos -v` log the scanned directories, the remotes read and why each candidate matched or not.** [WORKSPACE](specs/WORKSPACE.md)

- Verdict: real
- Evidence: Same as B01.

### T29  docs/TODO.md:58  [Good places to start / Commands]

* **S: Complete the project argument of `gero cd` from the config.** [SHELL_SETUP](specs/SHELL_SETUP.md)

- Verdict: real
- Evidence: Same as B08.

### T30  docs/TODO.md:59  [Good places to start / Commands]

* **S: Add a `gero doctor` check that the shell block is installed and current.** [SHELL_SETUP](specs/SHELL_SETUP.md)

- Verdict: real
- Evidence: Same as B09.

### T31  docs/TODO.md:63  [Good places to start / `ship` and `repo status`]

* **S: Find an existing bump PR by its marker in `gero ship`,** not by listing the newest 100 open PRs. On a busy base such as `main` of `rancher/rancher` an older PR of the bump is missed, and `ship` only warns. Done when a test with more than 100 open PRs finds it. [PUSH_TO_REPO](specs/PUSH_TO_REPO.md)

- Verdict: real
- Evidence: pulls.DefaultMax = 100 (internal/platform/github/pulls/pulls.go:51); execute.go:263 calls OpenForBase(..., 0), so the default applies; no lookup by marker.

### T32  docs/TODO.md:64  [Good places to start / `ship` and `repo status`]

* **S: Print the preflight line that names workstation signs under a forced CI context** in `gero ship`. `clicfg.WorkstationSigns` has no caller. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)

- Verdict: real
- Evidence: Same as A24 and A22.

### T33  docs/TODO.md:65  [Good places to start / `ship` and `repo status`]

* **S: Print the preflight line in `gero repo branch` and `gero repo tag`,** which follow the other rules and do not print it. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)

- Verdict: real
- Evidence: Same as A25.

### T34  docs/TODO.md:66  [Good places to start / `ship` and `repo status`]

* **M: Show in `gero repo status` how many releases a consumer is behind.** Needs the tags between two releases (`TODO(release-state)`). [REPO_STATUS](specs/REPO_STATUS.md)

- Verdict: moved
- Evidence: Same as C16 and F02: the need is real, but the per-series index belongs in tagscheme, not git.LatestReleases.
- Shape: Reword to name tagscheme

### T35  docs/TODO.md:67  [Good places to start / `ship` and `repo status`]

* **M: Show in `gero repo status` the open bump PR of a consumer that is behind,** as a tier of its own using the marker lookup. [REPO_STATUS](specs/REPO_STATUS.md)

- Verdict: real
- Evidence: Same as C17 and F05.

### T36  docs/TODO.md:68  [Good places to start / `ship` and `repo status`]

* **S: Show in `gero repo status` the images a release publishes,** using `internal/registry.CheckProject`. [REPO_STATUS](specs/REPO_STATUS.md)

- Verdict: real
- Evidence: Same as C18 and F04.

### T37  docs/TODO.md:72  [Good places to start / Config]

* **S: Raise the `lifecycle-date` warning** when a release is shipped to a line past a lifecycle date. Done when the rule is in `rules.go` and the [CONFIG_SCHEMA](design/CONFIG_SCHEMA.md#warnings) table says so.

- Verdict: real
- Evidence: Same as A07.

### T38  docs/TODO.md:73  [Good places to start / Config]

* **S: Warn on holding cycles longer than two projects.** Done when a three-project cycle raises `circular-holding`. [APP_CONFIG](specs/APP_CONFIG.md)

- Verdict: real
- Evidence: Same as A03.

### T39  docs/TODO.md:74  [Good places to start / Config]

* **L: Design the opt-in flags of `gero semver check` and `gero repo build-tags`,** such as a SemVer grammar mode and literal prefix matching, then build them with parity tests. The open questions of both specs come first. [CHECK_SEMVER](specs/CHECK_SEMVER.md), [BRANCH_TAGS](specs/BRANCH_TAGS.md)

- Verdict: question
- Evidence: Same as C02-C09: all questions; the TODO text already says the questions come first.

### T40  docs/TODO.md:78  [Good places to start / Internals]

* **S: Extract one `ui.Confirm`** for `ship`, `repo tag`, `shell install` and `doctor self-update`, which each draw their own `[y/N]`. Done when the four use it. [CLI_OUTPUT](specs/CLI_OUTPUT.md)

- Verdict: real
- Evidence: Same as A17 and B27.

### T41  docs/TODO.md:79  [Good places to start / Internals]

* **M: Hand the logger to every command through the context.** `ship` builds its own in `cmd/`, and the others do not log. [CLI_OUTPUT](specs/CLI_OUTPUT.md)

- Verdict: partly
- Evidence: Same as A19.

### T42  docs/TODO.md:80  [Good places to start / Internals]

* **M: Narrow `ship.Git` and `ship.Remote` to what the tasks call.** Both carry every method any task needs. [PROJECT_LAYOUT](specs/PROJECT_LAYOUT.md)

- Verdict: real
- Evidence: Same as B29.

### T43  docs/TODO.md:90  [Later (v1.x and beyond) / Tracking issues and the reconciler]

* **L: Write and render the state block of a tracking issue,** the part a machine reads and the view a person reads.

- Verdict: real
- Evidence: Same as D24.

### T44  docs/TODO.md:91  [Later (v1.x and beyond) / Tracking issues and the reconciler]

* **M: Create the `gero/stale` and `gero/superseded` labels on demand and reconcile them.**

- Verdict: real
- Evidence: Same as D22.

### T45  docs/TODO.md:92  [Later (v1.x and beyond) / Tracking issues and the reconciler]

* **M: Honor `gero/hold`:** check for it before work, so a held line is skipped.

- Verdict: real
- Evidence: Same as D23.

### T46  docs/TODO.md:93  [Later (v1.x and beyond) / Tracking issues and the reconciler]

* **L: Build `gero ci:reconcile`,** which brings the tracking issues in line with what the repos hold. It is a stub. Needs the items above and the reconciler's reads and writes of issues, check runs and releases.

- Verdict: real
- Evidence: Same as A27 and E07.

### T47  docs/TODO.md:99  [Later (v1.x and beyond) / Checks over the chain]

* **L: Check stable readiness:** would a stable Rancher build be valid with what Rancher holds? Needs the multi-hop walk.

- Verdict: real
- Evidence: No readiness code (grep); TOOLING_GOALS goal, depends on the multi-hop walk (D02).

### T48  docs/TODO.md:100  [Later (v1.x and beyond) / Checks over the chain]

* **M: Check line consistency:** does each hop hold a version of its own line?

- Verdict: real
- Evidence: No line-consistency code (grep); TOOLING_GOALS goal.

### T49  docs/TODO.md:106  [Later (v1.x and beyond) / The central bundle and workflows]

* **M: Add the `WorkflowsConfig` type and schema,** and validate a repo's `gero.yaml` against the allowed kinds. [APP_CONFIG](specs/APP_CONFIG.md)

- Verdict: real
- Evidence: Same as A01 and A02.

### T50  docs/TODO.md:107  [Later (v1.x and beyond) / The central bundle and workflows]

* **L: Build `gero workflows sync` and `gero workflows verify`.** Both are stubs. Sync brings the templates and generated chunks into a repo, and verify fails when a copy has drifted. Needs the type above.

- Verdict: real
- Evidence: Same as D14 and A12.

### T51  docs/TODO.md:108  [Later (v1.x and beyond) / The central bundle and workflows]

* **M: Generate `VERSION.md` from config.**

- Verdict: partly
- Evidence: Same as E12: nothing in code; decide layout and old readers first.

### T52  docs/TODO.md:109  [Later (v1.x and beyond) / The central bundle and workflows]

* **S: Send conditional requests when refreshing the bundle,** so an unchanged bundle is not downloaded again.

- Verdict: question
- Evidence: Same as D12: a stated limitation unless ghreleases adds support.

### T53  docs/TODO.md:110  [Later (v1.x and beyond) / The central bundle and workflows]

* **S: Add a setting for the bundle refresh interval.** `bundlestore.Resolver` takes one; no setting supplies it.

- Verdict: partly
- Evidence: Same as D13: Resolver.Refresh exists, no setting supplies it.

### T54  docs/TODO.md:111  [Later (v1.x and beyond) / The central bundle and workflows]

* **S: Decide whether `auto` should ever prefer a newer bundle to a local copy,** and record the answer in the spec.

- Verdict: question
- Evidence: Same as D11: a decision, mostly.

### T55  docs/TODO.md:115  [Later (v1.x and beyond) / Lines and lifecycle]

* **M: Build `gero lines sync`,** which fetches the SUSE lifecycle page and updates the dates in the lines file, showing the diff first. `internal/lifecycle` already parses the page; nothing calls it. [IMPLEMENTATION](IMPLEMENTATION.md)

- Verdict: real
- Evidence: Same as E10: no `lines sync`.

### T56  docs/TODO.md:116  [Later (v1.x and beyond) / Lines and lifecycle]

* **S: Make `gero doctor` flag drift between the lifecycle dates and the page.** Needs `lines sync`.

- Verdict: real
- Evidence: No lifecycle check in doctor (grep); needs T55.

### T57  docs/TODO.md:117  [Later (v1.x and beyond) / Lines and lifecycle]

* **M: Enforce `requiresPrerelease` for tags made by hand or by a workflow.** `gero repo tag` enforces it only for its own tags. [IMPLEMENTATION](IMPLEMENTATION.md)

- Verdict: real
- Evidence: RequiresPrerelease is enforced only by releasetag (repo tag); nothing enforces it for hand or workflow tags, and how it could is undecided.

### T58  docs/TODO.md:123  [Later (v1.x and beyond) / Doctor]

* **S: Check that the token can open pull requests.**

- Verdict: real
- Evidence: Same as B03.

### T59  docs/TODO.md:124  [Later (v1.x and beyond) / Doctor]

* **S: Check access to the tracker repo.**

- Verdict: real
- Evidence: Same as B03.

### T60  docs/TODO.md:125  [Later (v1.x and beyond) / Doctor]

* **S: Check push access to each fork.**

- Verdict: real
- Evidence: Same as B03.

### T61  docs/TODO.md:126  [Later (v1.x and beyond) / Doctor]

* **S: Warn of a clone far behind its upstream.**

- Verdict: real
- Evidence: Same as B03.

### T62  docs/TODO.md:130  [Later (v1.x and beyond) / Other]

* **M: Decide which steps of `gero ship` are used alone, and name them.** The candidates are goals, not command names: send a branch, open a PR for a pushed branch, apply a release to one branch and commit it, and close superseded PRs. Decide whether each is needed when one is built. [TASK_COMMANDS](specs/TASK_COMMANDS.md#use-cases-not-yet-commands)

- Verdict: question
- Evidence: Same as C10 and C11: the four use cases have no command; naming is the decision.

### T63  docs/TODO.md:131  [Later (v1.x and beyond) / Other]

* **S: Add `--from <ref>` to `gero repo branch`.** [TASK_COMMANDS](specs/TASK_COMMANDS.md)

- Verdict: question
- Evidence: Same as C12.

### T64  docs/TODO.md:132  [Later (v1.x and beyond) / Other]

* **M: Add golden files under `testdata/` for command output,** which tests compare as strings. Optional. [TESTING](specs/TESTING.md)

- Verdict: real
- Evidence: Same as B25.

### T65  docs/TODO.md:133  [Later (v1.x and beyond) / Other]

* **L: Write the spec for the QA bridge.** [PHASES](PHASES.md)

- Verdict: real
- Evidence: No QA bridge spec exists (only PHASES.md mentions it).

### T66  docs/TODO.md:134  [Later (v1.x and beyond) / Other]

* **L: Write the spec for the dashboard.** [PHASES](PHASES.md)

- Verdict: real
- Evidence: No dashboard spec exists; the command name is open (A16).

### T67  docs/TODO.md:142  [Wishlist]

* **L: Infer the upstream, fork, `githubUser` and release line from the working directory,** with an `inferred` source in `gero config show`. [WORKSPACE](specs/WORKSPACE.md#wishlist)

- Verdict: real
- Evidence: No inference in code; tests assert nothing is inferred.

### T68  docs/TODO.md:143  [Wishlist]

* **L: Add a go-git client** for the git work that runs the `git` binary. The seam is there (`TODO(go-git)`). [GITHUB_ACCESS](specs/GITHUB_ACCESS.md)

- Verdict: question
- Evidence: Same as F01: a revisit marker, not work.

### T69  docs/TODO.md:144  [Wishlist]

* **S: Publish the JSON Schemas at a stable URL.** [APP_CONFIG](specs/APP_CONFIG.md#json-schemas)

- Verdict: real
- Evidence: No $id in generated/jsonschema files.

### T70  docs/TODO.md:145  [Wishlist]

* **S (no code): Wire the published schemas into the team's editors.** Needs the stable URL.

- Verdict: real
- Evidence: Needs T69.

### T71  docs/TODO.md:146  [Wishlist]

* **S: Add `gero doctor self-update --version`** to install a named release or roll back. [SELF_UPDATE](specs/SELF_UPDATE.md)

- Verdict: real
- Evidence: Same as B11.

### T72  docs/TODO.md:147  [Wishlist]

* **S: Add a way to give `gero doctor self-update` a `trusted_root.json` by file** for offline use. `selfupdate.Config.TrustedRoot` takes one; no flag or setting supplies it. [SELF_UPDATE](specs/SELF_UPDATE.md)

- Verdict: real
- Evidence: Same as B12.

### T73  docs/TODO.md:148  [Wishlist]

* **M: Add a `host` setting (`GERO_HOST`, `--host`) and read it in `gero repo clone`,** so the SSH URL is not github.com only (`TODO(host)`). [CLI_CONFIG](specs/CLI_CONFIG.md), [GITHUB_ACCESS](specs/GITHUB_ACCESS.md)

- Verdict: real
- Evidence: Same as A10 and F03.

### T74  docs/TODO.md:149  [Wishlist]

* **S: Let a `repo clone`'s fork have a repository name other than the upstream's** under `githubUser`. [CLI_CONFIG](specs/CLI_CONFIG.md)

- Verdict: real
- Evidence: repoclone.fork asks ForkExists(user, slug) with the upstream's name; no way to name another.
