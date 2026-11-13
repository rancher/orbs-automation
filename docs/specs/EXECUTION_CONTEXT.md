# Execution Context

**Status:** implemented for the detection of the context, CI-only gating, commit identity and the write-command contract of `gero ship`. The rest is under [Not built yet](#not-built-yet). **(leaning)** marks a proposal, and **(unverified)** needs a small spike.

How one `gero` command behaves on an engineer's workstation and in CI, and who it acts as in each. [CLI_FRAMEWORK](CLI_FRAMEWORK.md) decides which commands are CI-only. This spec covers what every command that runs in both has in common, above all the ones that write. The first of those is [PUSH_TO_REPO](PUSH_TO_REPO.md): an engineer runs it locally and the PRs carry their name, and CI runs it and the PRs carry the bot's name. The same command and the same code do both.

## Principle

**A write command may be two commands.** `gero ship` is the workstation command and `gero ci:ship` the CI one ([CI_SHIP](CI_SHIP.md)), over shared backends, so the setup that only CI needs stays out of the local command's loop. The rest of this spec is what the two have in common, and the principle below still holds inside each backend.

**One code path, two actors.** A command never branches on "am I in CI" to decide what to do. The context chooses who acts and where inputs come from. It does not choose what the command does. Everything the context changes is listed once, below, so a reader can see all of it in one place.

## Detecting the context

One function in `internal/gero/clicfg` decides, once, and the result is `Settings.CI`. Nothing else reads `CI` or `GITHUB_ACTIONS`.

* CI is `GITHUB_ACTIONS=true`, or `CI=true`.
* `GERO_CONTEXT=ci|workstation` forces the answer and wins over both. It lets a developer reproduce a CI failure locally, lets a test pin the context, and lets a developer who exports `CI=true` for another tool keep their user file. Any other value is a usage error naming the variable.
* **Workstation signs under a forced CI.** When `GERO_CONTEXT=ci` forces CI, `gero` also looks for signs that this is a person's machine: none of `GITHUB_ACTIONS` and the other runner variables (`GITHUB_RUN_ID`, `GITHUB_REPOSITORY`, `RUNNER_*`), a TTY on stdin and stdout, and a user file at the default path. Any one is reported, all are listed in the preflight line of a write command, and none changes what the command does: they only warn. A TTY is itself one of the signs, so refusing on it would block scripted local debugging. They exist so a bot-shaped write from a laptop is never a surprise. Like the rest of detection they are read in `internal/gero/clicfg`.
* CI-only gating needs the CI context above (any of `GITHUB_ACTIONS=true`, `CI=true`, `GERO_CONTEXT=ci`) **and** a resolvable token ([GITHUB_ACCESS](GITHUB_ACCESS.md)), enforced by the root `PersistentPreRunE` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)). It deliberately does not require `GITHUB_ACTIONS` itself, so a developer can run a CI job locally with `GERO_CONTEXT=ci` and their own token to debug it.

## What differs by context

| Behavior | Workstation | CI | Owner |
|---|---|---|---|
| User file | Loaded | Ignored unless `GERO_CONFIG` is set | [CLI_CONFIG](CLI_CONFIG.md) |
| `upstream` default | Remote matching the slug, else the only remote | `origin` | [WORKSPACE](WORKSPACE.md) |
| Log handler | Text | JSON | [CLI_OUTPUT](CLI_OUTPUT.md) |
| Interaction | Prompts when stdin and stdout are a TTY, unless `--non-interactive` | Never prompts; a write needs `--non-interactive` | [CLI_OUTPUT](CLI_OUTPUT.md) |
| `gero repl` | Runs, with a TTY | Refused ([REPL](REPL.md)) | [REPL](REPL.md) |
| Ref and commit inputs | The repository | `GITHUB_REF`, `GITHUB_SHA` | [BRANCH_TAGS](BRANCH_TAGS.md) |
| Token | `GH_TOKEN`, then the `gh` login | `GH_TOKEN` from the workflow | [GITHUB_ACCESS](GITHUB_ACCESS.md) |
| Actor identity | The engineer | The bot | this spec |
| Push target and credentials | The engineer's fork | The upstream, with the workflow's token | [PUSH_TO_REPO](PUSH_TO_REPO.md) |
| Branch prefix | The engineer's `branchPrefix` | `bot/` | [PUSH_TO_REPO](PUSH_TO_REPO.md) |
| Labels | The consumer's `pullRequests.labels` | `labels` and `botLabels`, such as `origin/bot` | [METADATA](METADATA.md#label-classes) |
| Run record in the PR | The engineer's login | The workflow run URL | [PUSH_TO_REPO](PUSH_TO_REPO.md) |
| Commit signing | Optional, opt-in | None | this spec |

A new context-dependent behavior is added to this table in the same change that introduces it. A behavior that is not in the table does not depend on the context.

## The actor

The actor is who the commits and the PRs are attributed to. It is two things that must agree, and they come from different places.

| | Workstation | CI |
|---|---|---|
| **PR author** | The owner of the token, which is the engineer's `gh` login | The owner of the token, which is the bot |
| **Commit author and committer** | The engineer's git config (`user.name`, `user.email`) | The bot identity from settings |

* **The PR author is decided by token selection and nothing else.** `gero` never sets an author on the API call. Using the engineer's token is what puts their name on the PR, and no code is needed for it.
* **A write to another repository checks the token it will use.** The default `GITHUB_TOKEN` is scoped to the repository the workflow runs in, so a cross-repo write with it is a configuration mistake, and `gero` refuses it before any write with a message naming `GH_TOKEN`, instead of letting GitHub return a 403. The check applies to every CI-only command that writes elsewhere. It treats the token as the default one when `GH_TOKEN` is unset or equal to `GITHUB_TOKEN`. Asking GitHub which repositories the token reaches (`GET /installation/repositories`) and requiring the target to be among them is a stronger test **(unverified)**, since both the default and an App installation token are `ghs_` tokens and the prefix cannot tell them apart.
* **`gero` never mints a token.** A workflow that needs to write to another repository creates a cross-repo token itself (for example a GitHub App installation token scoped to the target repository) and passes it as `GH_TOKEN`, which wins over `GITHUB_TOKEN` ([GITHUB_ACCESS](GITHUB_ACCESS.md)). The default `GITHUB_TOKEN` is scoped to the repository the workflow runs in and cannot push to `rancher/rancher`.
* **The bot identity is an input, not something derived from the token.** An installation token does not reveal the bot's login. The workflow supplies it as an environment variable, `GERO_BOT_USER` (for example `rancher-scc-bot[bot]`), alongside the token, both loaded from Vault by workflow steps. `gero` builds the commit identity from it the way the bash does today: `name` is the login and `email` is `<numeric id>+<login>@users.noreply.github.com`, where the id comes from `GET /users/<login>`, which is public.
* **`gero` is the plain-English name.** Docs, comments, issues and PR text call the actor `gero`, and credit "ORBS Team Automation" as the team behind it. There are two forms, fixed in the tool and never read from config or the environment. The HTTP `User-Agent` is a real product string, `gero/<version> (+https://github.com/rancher/orbs-automation; ORBS Team Automation)` (`internal/platform/github.UserAgent`, version set once from the build). The short credit, `gero (ORBS Team Automation)` (`actor.Credit`), is only for the "Created with: ..." line in PR and issue templates that people read, and is never sent on the wire. Both are labels: authorship still comes from the token and `GERO_BOT_USER`.
* **Commit identity resolution.** On a workstation `gero` reads the engineer's git configuration, repository then global then system, and fails with a message naming `user.name` and `user.email` when either is missing. It never invents one. In CI `GERO_BOT_USER` is required for any command that commits, and the command fails fast without it.
* **A mismatch is not hidden.** If the workstation token and the git identity disagree (a token for one account, commits for another) the preflight line shows both.
* **Who commits is decided by whoever makes the commit.** `gero ship` commits through the `git` binary, which reads the identity from the clone's own configuration, so on a workstation `gero` only reads it, to say it ([The write-command contract](#write-command-contract)) and to fail before any write when it is missing, and does not pass it on. The read is made in the clone the commits go in (`identity.ResolveIn`), since a `user.email` set in that clone alone is the one a commit there carries. A commit made through go-git has no git configuration to lean on and has to be given the identity explicitly (`actor.Identity` is what it is given), and so does the bot's commit in CI, whose fresh clone has none: `ship` passes the bot's identity to every commit when it runs as the bot, with git's `user.name` and `user.email` set for that one commit.

`gero auth status` has to work with a bot token. An installation token cannot call `GET /user` **(unverified)**, so the "login GitHub reports" falls back to saying "installation token" and does not fail.

## Push credentials and signing

**Push uses the `git` binary, through `internal/platform/git/gitcli`.** A push needs what go-git does not carry: the engineer's SSH agent and `~/.ssh/config`, credential helpers and hooks on a workstation, and a token over HTTPS in CI. It is never forced, with prompts off (`GIT_TERMINAL_PROMPT=0`) so a missing credential fails at once. `execx` scrubs the environment, so a push passes on only the variables that reach the agent and the helpers, and never a token. Which remote a push goes to is in [PUSH_TO_REPO](PUSH_TO_REPO.md).

The CI path, the token as a one-shot credential over HTTPS and never written to a remote URL or to disk, is not built and belongs to `gero ci:ship`.

**Signing is off by default and opt-in for humans.** Signed commits are not required by the target repositories. An engineer who wants them sets `signCommits: true` in the user file (or `GERO_SIGN_COMMITS=true`, which beats the file). `gero ship` signs every commit it makes when it is on.

* The signature is made by `git` with the engineer's own `gpg.*` and `user.signingkey` configuration, so `gero` holds no key and never reads one. With signing on, the commit is created through `gitcli`, not go-git.
* If signing is requested and fails (no key, agent locked), the command fails. It never falls back to an unsigned commit, because the engineer asked for the opposite.
* In CI the setting is ignored, as every user-file key is ([CLI_CONFIG](CLI_CONFIG.md)). Bot commits are not signed and not verified.

`signCommits` fits the user file because it states how one engineer's commits are made, not what the tool decides.

## Write-command contract

Every command that writes to git or GitHub follows these rules in both contexts. They are what lets a command be tested once and trusted twice. How a command that pushes branches and opens PRs applies them is in [PUSH_TO_REPO](PUSH_TO_REPO.md).

1. **Preflight line.** Before the first write, the command prints who it acts as and where it writes: token login (or "installation token"), commit identity, push target, signing on or off. `--dry-run` prints the same line. `gero ship` prints the identity, signing and the PR author's login as one line, `acting as Dan Pock <dan@example.com>, commits not signed, PRs by danpock`, after the PR preview and before the work, and the push target in the question it asks and in the closing summary. A missing identity fails the run before anything is written.
2. **`--dry-run` parity.** A dry run computes and prints everything, including the branch names and PR titles, and changes nothing remote. It takes the same path as a real run up to the first write.
3. **Never lose the engineer's work.** A checkout with uncommitted changes is never touched: the edit happens in a temporary worktree of the same repository. A clean checkout may be switched to the work branch and is put back on the branch it started on. A dry run always uses a worktree, and a `gero` run leaves the clones it found as it found them, including after a failure ([PUSH_TO_REPO](PUSH_TO_REPO.md#preparing-the-branch)). In CI there is no engineer's checkout: `gero ci:ship` works in the checkouts the workflow provides ([CI_SHIP](CI_SHIP.md)).
4. **Per-target results.** A fan-out reports each target and tolerates partial failure ([IMPLEMENTATION](../IMPLEMENTATION.md#repo-shapes-and-downstream-bumps)), so exit code and output do not depend on the context.
5. **One confirmation per command, and a flag to skip it.** Before a command starts writing it prints the preflight line and asks once, "proceed?", in both contexts. `--non-interactive` (alias `--yes`, env `GERO_NON_INTERACTIVE`) answers yes and never prompts, which is how a workflow, or a script, runs the same command. Without the flag and without a TTY the command fails before any write, with a message naming the flag, so a forgotten flag in CI is a loud failure and not a hang or a silent yes. A forced CI context that shows workstation signs (see [Detecting the context](#detecting-the-context)) adds a warning line naming them to the preflight line. `--dry-run` writes nothing and so never asks. `--dry-run` stays opt-in: making it the default would mean every real run needs `--dry-run=false`, so the confirmation is the safety instead.

## Testing

Domain code never reads the environment, so a test drives it with a `Settings` value and fakes ([TESTING](TESTING.md)). For every write command the table-driven test runs the same scenario twice, once as the workstation actor and once as the bot, and asserts only what differs: the commit identity, the push target and the PR author recorded by the fake API.

## Not built yet

* The cross-repo token check (`cigate.RequireCrossRepoToken`) and the workstation-signs detection (`clicfg.WorkstationSigns`) exist, and no command calls them yet.
* The `auth status` fallback treats a 403 from `GET /user` as an installation token, which is unverified against a real one.
* The preflight line that names workstation signs under a forced CI context.
* `gero repo branch` and `gero repo tag` follow the one-confirmation and `--dry-run` rules but do not print the preflight line, which only `gero ship` prints.
* The push of `gero ci:ship` without an engineer's credentials.

## Open questions

None yet.
