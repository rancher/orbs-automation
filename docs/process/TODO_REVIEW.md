# TODO review sheet (working file)

The one-page way to go through [TODO_AUDIT](TODO_AUDIT.md). It is deleted with the audit when the new `docs/TODO.md` lands. Nothing here is decided until you say so; every "Proposed" cell is a recommendation.

## How to review

1. **Pass 1** is what I propose to close without a decision. Read it once; say which, if any, to keep.
2. **Pass 2** is the feature domains. Each has a proposed milestone, the work in it, and the decisions that shape it, each with a recommended answer. Reply by exception, such as "all as proposed except D3, DEC-9 and DEC-14", or write in the **Your call** cells.
3. A ★ marks work that is **not on the current `docs/TODO.md`**.

**Milestones**, mapped to [PHASES](../PHASES.md): **BV1** Before V1 (the v1.0.0 gate), **AV1** After V1 (the v1.x themes), **BV2** Before V2 (what the v2.0.0 gate needs: a real line cut, scripts retired), **WISH** Moonshot and wishlist (the dashboard, the QA bridge, and anything with no pull). Redraw any of these.

Sizes are S (hours), M (a day or two), L (design as well as code).

---

## Pass 1: closed without a decision

Say which, if any, should stay open.

| IDs | Why it closes |
|---|---|
| B19, C03, C21 | Built, and the note records a fact (`GERO_REPL`, the short commit, the unknown-line rule). |
| B30, A18, C23, C26, C31, C38, D18 | Duplicates of another item (B24, A12/C37, C20, C34, C40, C28, D13). |
| B20, B22, C32 | Settled in code (no `PS1`, no rc-file trick, branch in place or in a worktree). The spec keeps the reason; the list does not need it. |
| E03, E05, E09, B23, C19 | A note that answers itself: a cross-reference, the PHASES policy ("alongside, then switch"), the REPL already shipping, both sections shown by default. |
| F01 | A revisit marker for go-git, not work. Stays as a code comment only. |
| D12, T52, B31 | A stated limitation (no conditional requests until `ghreleases` has them); generated schemas already live in `generated/`. |

I would also fix these in place, with no list entry: the `TODO(release-state)` comments at `git/latest.go` and `ship/holdings.go` (they point at `git.LatestReleases`, and the per-series index belongs in `tagscheme`); `REPO_STATUS.md:51` naming a `TODO(releasetag)` marker that does not exist; the `BRANCH_TAGS` and `PROJECT_LAYOUT` questions the code has settled.

**Your call:** Agreed, and applied: the settled notes are removed from their specs, the stale comments and the `REPO_STATUS` marker line are fixed. Duplicates and `GERO_REPL`-style rationale stay in their specs and only drop out of the rebuilt TODO.

---

## Pass 2: feature domains

### Domain 1. Ship from CI (`gero ci:ship`)

**Proposed: BV1.** The v1.0.0 gate names it, and `scc-operator` and `shell` cannot drop their scripts without it. The bot identity is already built; only the CI command and its wiring are missing.

| IDs | Work | Size |
|---|---|---|
| C34, C35, T01 | Register `ci:ship` as CI-only and run `gero ship`'s bump loop as the bot (the bot identity, `GERO_BOT_USER` and `identity.ResolveIn`, is built and tested; no CI command calls it) | L |
| C36, T02 | A temporary clone as a `Workspace`, and a one-shot token credential for push and tag push | M |
| C37, T03 | Step summary and workflow run URL in the PR body | S |
| C28, T04 | Steps `from: producer` and `from: central` (steprun refuses both) | M |
| A22, A24, T32 | Call `RequireCrossRepoToken` and `WorkstationSigns` (neither has a caller) and print the preflight line | S |
| C39, T06 | Workflows in `scc-operator` and `shell` that install a pinned, signed `gero` and run `ci:ship` beside the script | M |
| A23, T07 | Check the `auth status` 403 path with a real installation token (needs a real run) | S |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-1 | C40: does `gero` make its own temp clone, or take a directory from the workflow? | Its own, since PHASES says "a temporary clone" and a workflow then needs no checkout | |
| DEC-2 | C41: does `ci:ship` fan out over lines like `gero ship`? | Yes, same loop | |
| DEC-3 | C42: when the deterministic branch already moved (push rejected) | Report it and exit non-zero; no retry | |
| DEC-4 | C43: do the step summary and the PR comments carry the same text? | Yes | |
| DEC-5 | D17, T05: what config a `ci:*` job without a checkout uses | The bundle of the installed release by default; a checkout still wins when `GERO_CENTRAL_CONFIG_DIR` is set | |
| DEC-6 | A26: do workstation signs under a forced CI do more than warn? | No, warn only (the spec says so) | |

**Domain milestone, your call:** ______

---

### Domain 2. Chain tracing: read what a repo holds, walk the hops

**Proposed: BV1 for the reads and the walk; AV1 for the stage check.** PHASES puts "reading a commit or branch holding" and the multi-hop walk in the v1.0.0 chain stream. The stage check is a v1.x check over the chain.

| IDs | Work | Size |
|---|---|---|
| D01, T09 | Read a commit or branch holding at a ref (the engine skips both kinds today) | M |
| D06 | Read the `lock.yaml` pin at a ref and compare it with the head of the stage's branch | M |
| A06, T10 | `dockerfile` format for the update engine (`ErrFormatNotBuilt`) | M |
| D02, C20, T11 | Cycle-safe multi-hop walk, tree output and `--depth` in `lines resolve` and `repo status` | L |
| D03, T13 | Check a held tag's stage against the release team's rule (**AV1**) | M |
| D05 | Confirm the pre-v2.16 key is `CHART_DEFAULT_BRANCH` in `package/Dockerfile` and find the file for v2.16 and later | S |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-7 | D07: is a `{since, until}` range enough to say which lines a holding covers? | Yes, keep it | |
| DEC-8 | D09: does the commit alone answer "is this chart version in this Assets tag"? | Commit alone is the cheap check; reading `release.yaml` at it is the precise one, later | |

For the release team, not for you (T08): how the Rancher key and `lock.yaml` pin are read, and how a BRO tag maps to a chart version (D08).

**Domain milestone, your call:** ______

---

### Domain 3. Charts automation (`chartsPackageBump`, chart-version tags)

**Proposed: BV1.** The v1.0.0 gate names BRO's tag becoming a charts bump. ★ E01 is not on the current list: `rancher-charts.yaml` already sets `scheme: chartVersion`, and `tagscheme.ForStream` answers "not built", so Charts cannot resolve a newest tag.

| IDs | Work | Size |
|---|---|---|
| E01 ★ | A `chartVersion` scheme in `tagscheme` (Valid, LineOf, Latest, IsStable, Compare), reading the part after `+up` | M |
| D04, A06, T12 | Real `chartsPackageBump` Apply (edits packages in `rancher/charts`) and BRO tag to chart version | L |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-9 | C33: how does a strategy report the value it wrote? | Add a field to `bump.Result` when the strategy is built | |
| DEC-10 | D10: does the charts bump differ by line? | Answer when D04 is built | |

**Domain milestone, your call:** ______

---

### Domain 4. Reconciler and tracking issues

**Proposed: AV1.** PHASES lists tracking issues and the reconciler under v1.x. Nothing is built, and every item waits on `ci:reconcile`.

| IDs | Work | Size |
|---|---|---|
| D24, A27, T43, T46 | State block and view of a tracking issue; `ci:reconcile` and its issue, check-run and release reads and writes | L |
| D22, T44 | Create `gero/stale` and `gero/superseded` labels on demand | M |
| D23, T45 | Honor `gero/hold` before work | M |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-11 | E07: how the reconciler is triggered | Cron only first; `repository_dispatch` later if latency hurts | |
| DEC-12 | D25: labels per class? | No; per project (already built) is enough | |
| DEC-13 | D26: label vocabulary | `gero/*` as the spec has it | |

**Domain milestone, your call:** ______

---

### Domain 5. Repo workflow management (WorkflowsConfig, `workflows sync` and `verify`, templates, `VERSION.md`)

**Proposed: AV1 for the type, `sync`, `verify` and `VERSION.md`; BV2 for "branch for X" (E13).** This is a scoped feature domain that is not built: both commands are stubs, there is no `WorkflowsConfig` Go type, the bundle has no template chunk, and nothing generates `VERSION.md`. PHASES lists rendered workflows and `VERSION.md` under v1.x and "branch for X" as a v2.0.0 gate item.

| IDs | Work | Size |
|---|---|---|
| A01, A02, T49 | `WorkflowsConfig` Go type, schema, and a loader that rejects any other kind in a repo's `gero.yaml` | M |
| D14, A12, T50 | `workflows sync` and `verify`, with template and generated chunks in the bundle | L |
| E12, T51 | Generate `VERSION.md` from `branches.lines` and `RancherLines` | M |
| E13 | "Branch for X": compose `repo branch`, `branches.lines` update, `VERSION.md` and workflow render (**BV2**) | L |
| D16 | Rename or widen `centralConfigDir` once templates ship | S |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-14 | E08: reusable workflows or composite actions, and which workflow first | Reusable workflows pinned by tag or SHA (PHASES); `ci.yml` already has `workflow_call`; start with the one `ci:ship` needs | |
| DEC-15 | E11: how a downstream repo pins and calls them | By `gero` tag or SHA, as PHASES says | |
| DEC-16 | A15: does `workflows verify` exit 3 on drift? | Yes | |
| DEC-17 | A09: does any config besides `WorkflowsConfig` move into target repos? | No | |
| DEC-18 | E12: the `VERSION.md` layout and whether older readers (frameworks-automation) need the old formats | **Not mine to answer**: needs the team and a look at those readers | |

**Domain milestone, your call:** ______

---

### Domain 6. Lines and lifecycle

**Proposed: AV1.** PHASES lists `gero lines sync` in v1.x.

| IDs | Work | Size |
|---|---|---|
| E10, T55 | `gero lines sync`: fetch the lifecycle page (`internal/lifecycle` parses it, nothing calls it), update dates, show the diff first | M |
| T56 ★ | `gero doctor` flags drift between the lifecycle dates and the page (needs T55) | S |
| A07, T37 | The `lifecycle-date` warning rule | S |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-19 | E10: is a new line added by hand or by a command? | By hand; the Kubernetes range is manual either way | |

**Domain milestone, your call:** ______

---

### Domain 7. Release status, tags and the task commands

**Proposed: AV1.** None of this is in the PHASES v1.0.0 gate; these are the "release-state" extras and the task commands.

| IDs | Work | Size |
|---|---|---|
| C16, F02, F05, T34 | Releases behind in `repo status`. The need is real but the per-series index belongs in `tagscheme`, not `git.LatestReleases` | M |
| C17, T35 | Open bump PR tier in `repo status`, reusing `pulls.Service` and `ParseMarker` | M |
| C18, F04, T36 | Images a release publishes in `repo status`, from `registry.CheckProject` | S |
| T31 | Find a bump PR by marker, not by the newest 100 open PRs (`pulls.DefaultMax`) | S |
| T57 | Enforce `requiresPrerelease` for tags made by hand or a workflow (PHASES: v1.x) | M |
| C27, B28 | Move the rest of ship's steps (closing superseded PRs, plan, publish) into `internal/task`; commands for push, PR and bump-commit only when needed | M |
| A25, T33 | Preflight line in `repo branch` and `repo tag` | S |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-20 | C10, C11, T62: which steps of `ship` are used alone, as commands (push-branch, open-PR, bump-commit, close-superseded) | None until a second caller needs them | |
| DEC-21 | C12, T63: `repo branch --from <ref>` | Add when asked; WISH | |
| DEC-22 | C13: annotated or signed tags | Stay lightweight | |
| DEC-23 | C22: a "where does shell's v2.15 go" command | Drop; `ship` covers it | |
| DEC-24 | C14, C15, C25: `@` shorthand, sharing `resolvePushTarget`, a config knob for a step's commit | Defer all three until something needs them | |
| DEC-35 | C29, C30: should the default PR title identify the tooling, and should steps pick a toolchain (they use the machine's own) | Neutral title stays; no toolchain selection | |
| DEC-36 | D19, D20, D21: the image check (`repo images exist`) is HEAD-only, existence-only, with a fixed 20 s timeout and no retry | Leave as is; revisit if a registry ignores HEAD or flakes | |

**Domain milestone, your call:** ______

---

### Domain 8. Checks over the chain (readiness, line consistency)

**Proposed: AV1.** PHASES lists them under v1.x, and both need the walk from Domain 2.

| IDs | Work | Size |
|---|---|---|
| T47 | Stable readiness: would a stable Rancher build be valid with what Rancher holds? | L |
| T48 | Line consistency: does each hop hold a version of its own line? | M |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-25 | E02: how a view names a component with no tracing config | Decide when the view is designed | |

**Domain milestone, your call:** ______

---

### Domain 9. Config engine, schema and validation

**Proposed: mixed.** The v1.0.0 gate needs the names settled as `gero/v1` and the real configs validated in CI (BV1). The rest is AV1.

| IDs | Work | Milestone |
|---|---|---|
| A08, A11, E06, T17 | Freeze kind and field names, then rename `gero/v1alpha2` to `gero/v1` | BV1 |
| T22 ★ | A Go test that validates every file of `config/` against the JSON Schemas (needs a validator module) | BV1 |
| A05, T14 | `config validate` and `doctor` build the update engine and report its errors | BV1 |
| A03, T38 | Warn on holding cycles longer than two projects | AV1 |
| A04 ★ | `config show --resolved` | AV1 |
| T69, T70 | Publish the schemas at a stable URL, then wire them into editors | WISH |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-26 | A08, A11: when to freeze names | Once WorkflowsConfig (Domain 5) and the Charts shapes (Domain 3) are done, since both add kinds | |

**Domain milestone, your call:** ______

---

### Domain 10. CLI framework hygiene

**Proposed: the exit-code helper BV1 (the gate names it); the rest AV1.**

| IDs | Work | Milestone |
|---|---|---|
| A13, B02, T18 | Exit-code helper: usage and validation errors exit 2 | BV1 |
| A17, B27, T40 | One `ui.Confirm` for the four `[y/N]` prompts | AV1 |
| A19, T41 | One logger through the context | AV1 |
| B29, T42 | Narrow `ship.Git` (~22 methods) and `ship.Remote` (13) | AV1 |
| A14, T27 | `--json` for `config validate`, `--output env` for `auth status` | AV1 |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-27 | B04: run doctor checks in parallel | No; drop unless it gets slow | |

**Domain milestone, your call:** ______

---

### Domain 11. Doctor

**Proposed: AV1.**

| IDs | Work | Size |
|---|---|---|
| B03, T58–T61 | Online checks: PR-create permission, tracker access, push access per fork, a clone far behind its upstream | M-L |
| B09, T30 | Check that the shell block is installed and current | S-M |
| B01, T28 | `config resolve repos -v` logs scanned directories, remotes and why each matched | M |
| T26 | Warn when a newer `gero` exists (**WISH** unless you want it) | S |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-28 | B05: a paste-safe `doctor` mode | WISH | |
| DEC-29 | B14, T26: an online newer-release check | Yes, behind `--online` | |

**Domain milestone, your call:** ______

---

### Domain 12. Shell and REPL

**Proposed: WISH, except `gero cd` completion (AV1).**

| IDs | Work | Milestone |
|---|---|---|
| B08, T29 | Complete the project argument of `gero cd` | AV1 |
| B15 ★, B16 ★, B18 ★ | REPL `help`, a one-line unknown-command hint, `!<cmd>` | WISH |
| B21 ★ | Opt-in prompt prefix when `GERO_REPL` is set | WISH |
| B06 ★, B07 ★ | Nushell, elvish, Windows PowerShell 5; tests that run the scripts in real fish and pwsh | WISH |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-30 | B17: more REPL prompt placeholders | Only if asked | |

**Domain milestone, your call:** ______

---

### Domain 13. Self-update and release engineering

**Proposed: confirming a real release is BV1 (the gate says so); the rest WISH.**

| IDs | Work | Milestone |
|---|---|---|
| T23 | Confirm a release end to end: `cosign` in the CI image, bundles on the release, `doctor self-update` installs it (needs a real run) | BV1 |
| T24 | Confirm a pushed tag starts the release workflow of a real repo (needs a real run) | BV1 |
| B10 ★ | A test or CI step that verifies a real `.sigstore.json` bundle | AV1 |
| B11, T71 | `self-update --version` to install or roll back | WISH |
| B12, T72 | Give `self-update` a `trusted_root.json` by file | WISH |
| B13 ★ | Windows build | WISH, or reword as a non-goal |

**Domain milestone, your call:** ______

---

### Domain 14. Testing and integration

**Proposed: the integration tier and the hostile-environment job BV1; the rest later.**

| IDs | Work | Milestone |
|---|---|---|
| B24, T19 | `test/integration` module that runs the built binary, plus a CI job | BV1 |
| B26, T21 | One CI run of `go test ./...` with a hostile environment | BV1 |
| C24, T20 | Integration test that pushes to a scratch repo (needs the tier) | AV1 |
| B25, T64 | Golden files for command output | WISH |
| A20 ★ | A pty harness for the REPL and prompts (needs a dependency decision) | WISH |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-31 | A21: a fake GitHub server library or hand-rolled `httptest` handlers | Hand-rolled, as the unit tests already do; decide when T19 is built | |

**Domain milestone, your call:** ______

---

### Domain 15. Central bundle

**Proposed: AV1, all small.**

| IDs | Work | Size |
|---|---|---|
| D13, T53 | A `bundleRefresh` setting wired to `Resolver.Refresh` | S |
| D15 | Is 60 unauthenticated requests an hour enough in CI? (the failure is reported and the cache is the fallback) | S |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-32 | D11, T54: should `auto` prefer a newer bundle to a local copy? | No; `--config-source` already picks one | |

**Domain milestone, your call:** ______

---

### Domain 16. GitHub hosts beyond github.com

**Proposed: WISH.** All SUSE repos are on github.com. The code partly tolerates other hosts, which is the root of the question.

| IDs | Work | Size |
|---|---|---|
| A10, F03, T73 | A `host` setting (`GERO_HOST`, `--host`) used by credentials and `repo clone`'s SSH URL | M |
| A28 ★ | API base URLs for `*.ghe.com` and `github.localhost` | S |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-33 | A29: supported, or only tolerated | Only tolerated, which is why the work is WISH | |

**Domain milestone, your call:** ______

---

### Domain 17. `build-tags` and `semver check` flags

**Proposed: WISH.** The default behaviour is a faithful port and is built; every improvement is opt-in and nothing needs one.

| IDs | Work |
|---|---|
| C01–C09, T39 | Open questions only: `--ref`/`--sha`/`--repo`, one flag per improvement vs a group, stderr opt-in, `env-full`, `--strict`, a grammar mode |

| # | Decision | Proposed | Your call |
|---|---|---|---|
| DEC-34 | Do any of the C01–C09 improvements get built? | None, until a repo needs one; close the questions as "not planned" | |

**Domain milestone, your call:** ______

---

### Domain 18. Process docs and spec upkeep

**Proposed: BV1.** The gate says the process docs are reviewed by the release team.

| IDs | Work | Size |
|---|---|---|
| T16 | Process docs reviewed by the release team; today every one is `draft` or `staging` | M, no code |
| T15 | Check the config against the repos (charts' stable branch names, the `ob-team-charts` package) | S, no code |
| T25 | Correct the specs that no longer match the code | S, no code |
| E04 | Who adds a line in `rancher-assets` (the config says `managedBy: release-team`) | for the release team |

**Domain milestone, your call:** ______

---

### Domain 19. Dashboard, QA bridge, workspace inference, go-git

**Proposed: WISH.** No spec exists for the first two, and the dashboard's own command name is open.

| IDs | Work |
|---|---|
| A16, T66 | A spec for the dashboard (`dashboard` or `triage`, A16) |
| T65 | A spec for the QA bridge (PHASES: v3.0.0) |
| T67 | Infer upstream, fork, `githubUser` and release line from the working directory |
| T74 | Let a clone's fork have a repository name other than the upstream's |
| F01, T68 | A go-git client for the work that runs `git` |

**Domain milestone, your call:** ______

---

## Counts

Every item of the audit is in a Pass 1 row or one of the 19 domains above, and the four "needs a real run" checks (A23, T07, T23, T24) sit in Domains 1 and 13. If you find an item that appears nowhere, say so.
