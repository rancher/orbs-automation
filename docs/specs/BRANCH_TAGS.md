# branch-tags

**Status:** implemented for the faithful port (sections 1 to 4). Not built yet: the opt-in improvements in section 6 and the optional parity test. Section 5 has open questions.

Port of the `.github/scripts/branch-tags.sh` bash script that many ORBS repos carry. The command is `gero repo build-tags`: it names the tags of a build of the head of a branch (the rolling tag, the fixed tag for the commit, and the previous fixed tag, which a release tool such as goreleaser can use as the start of its changelog). Its first name was `current-tags`, which still works as an alias, and the script keeps its name. The goal is to prove the script can live in `gero`, not to delete the original. The default behavior is a **faithful port**: same inputs, same output keys, case and order, same results. Every approved improvement is listed in section 6 and is opt-in. Git access uses go-git behind an interface ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md), [GITHUB_ACCESS](GITHUB_ACCESS.md)).

## 1. What the script does today

**Inputs.** The current ref and commit, taken from one of two places:

1. **GitHub Actions:** when both `GITHUB_REF` and `GITHUB_SHA` are non-empty, the ref is `GITHUB_REF` and the commit is the first 7 characters of `GITHUB_SHA`.
2. **Local repo:** otherwise the script requires a `.git` directory in the current directory, then uses `git symbolic-ref HEAD` for the ref and `git rev-parse --short HEAD` for the commit.

**Branch rules.** Only two ref shapes are supported:

| Ref | `branch_tag` | `branch_static_tag` | Previous tag prefix |
|---|---|---|---|
| `refs/heads/main` | `head` | `main-<sha>` | `main-` |
| `refs/heads/release/<version>` | `<version>-head` | `<version>-head-<sha>` | `<version>-head-` |

`<version>` is whatever follows `refs/heads/release/`, expected to look like `v2.x`. It is not validated.

**Previous tag.** The newest tag, by `git tag --sort=-creatordate`, whose name starts with the prefix. Empty when none match.

**Output.** Three lines on stdout, in this order, exit 0:

```
branch_tag=<value>
branch_static_tag=<value>
prev_static_tag=<value or empty>
```

**Errors.** Any other ref prints `Unsupported branch pattern. Expected 'main' or 'release/*'.` and exits 1. A missing `.git` prints a message and exits 1. Both messages go to **stdout**, not stderr.

**Quirks worth knowing:**

* Detached HEAD (no `GITHUB_REF`) makes `git symbolic-ref` fail, and `set -e` ends the script with git's exit code (128) and git's own stderr message.
* Tag refs (`refs/tags/v1.0.0`) and pull request refs (`refs/pull/12/merge`) are unsupported, so they exit 1.
* `.git` must be a directory in the current directory. Subdirectories, worktrees and submodules (where `.git` is a file) fail.
* The previous-tag prefix is passed to `grep` as a **regex**, so the `.` in `v2.x-head-` matches any character.
* The previous tag is the newest matching tag, **including one on the current commit** if it was already tagged. It is not "the one before this".
* An empty version (`refs/heads/release/`) gives `branch_tag=-head`, and `release/a/b` gives version `a/b`.
* The local short commit comes from git's `--short`, which grows past 7 characters in large or ambiguous repos. The CI commit is always exactly 7.
* A tag with no match is not an error, because the pipeline has no `pipefail`. It only shows up as an empty `prev_static_tag=`.
* Tag history only exists if the checkout fetched tags. A shallow checkout without them gives an empty previous tag.

## 2. Faithful port (the default)

What makes `gero repo build-tags` a faithful port. Each point matches the script:

```
gero repo build-tags
```

1. **Inputs.** When both `GITHUB_REF` and `GITHUB_SHA` are non-empty, the ref is `GITHUB_REF` and the commit is the first 7 characters of `GITHUB_SHA`. Otherwise the repository in the current directory is read, which must have a `.git` directory there. The command reads the environment and passes values down. The library never reads it.
2. **Branch rules.** Only `refs/heads/main` and `refs/heads/release/<version>` are supported, with the tags in the table in section 1. `<version>` is not validated, so `release/` gives `branch_tag=-head`.
3. **Previous tag.** The newest tag by `creatordate` whose name starts with the prefix, matched as a regex as the script does, so the `.` in `v2.x-head-` is a wildcard. No match is not an error: `prev_static_tag=` is empty.
4. **Previous means previous in sequence.** The newest matching tag counts even when it already points at the current commit. This is decided, not a quirk to fix: the question is what came before in the tag sequence, not what came before on a different commit.
5. **Local short commit.** Matches `git rev-parse --short HEAD`, which is 7 or more characters depending on repository size. go-git has no equivalent, so this one call goes through the `git` escape hatch ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)); see section 3.
6. **Output.** Exactly three lowercase `key=value` lines on stdout, in the script's order: `branch_tag`, `branch_static_tag`, `prev_static_tag`. The key case differs from [CHECK_SEMVER](CHECK_SEMVER.md) (upper case) because each matches the script it replaces and workflows read those names.
7. **Errors.** An unsupported ref and a missing `.git` exit 1. A detached HEAD with no CI environment exits non-zero, as the script does (the script's 128 comes from git, which go-git will not reproduce exactly; `gero` exits 1). The message goes to **stdout**, as in the script, and only once. Error message text is not part of the contract.
8. **Tags in CI.** With `GITHUB_REF` and `GITHUB_SHA` set the script only runs `git tag`, so no `.git` directory is required in the current directory: the repository is found from the current directory upward, and no repository at all gives an empty `prev_static_tag=`, not an error.
9. **Context:** either (read-only, no credentials).

## 3. Library

`internal/platform/git` (pure logic, no go-git and no exec) resolves the result from a ref, an abbreviated commit and a tag source. The interface is defined in the package that consumes it ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 3):

```go
type BranchTags struct {
    BranchTag       string
    BranchStaticTag string
    PrevStaticTag   string
}

// Tags lists tag names, newest first by creator date.
type Tags interface {
    NewestFirst() ([]string, error)
}

var ErrUnsupportedRef = errors.New("unsupported branch pattern")

// Short abbreviates a CI commit to its first 7 characters.
func Short(sha string) string

// ResolveBranchTags uses commit exactly as given.
func ResolveBranchTags(ref, commit string, tags Tags) (BranchTags, error)
```

* `ResolveBranchTags` is pure apart from the `Tags` call. It uses `commit` **as given**, and the caller abbreviates: `Short` (7 characters) for the CI `GITHUB_SHA`, git's own `--short` for a local repository, which can be longer. If `ResolveBranchTags` cut to 7 itself, the default local output would silently become the "Fixed 7-character short commit" improvement. The opt-ins are options on the call, so the default is the faithful behavior.
* `internal/platform/git/repo` reads the current ref and commit from a repository and implements `Tags`, using go-git: `Open` (strict: requires `.git` as a directory in the given directory), `Discover` (from the directory upward, used for listing tags in CI), `Head` and `NewestFirst`. It is the only code that imports go-git, so `ResolveBranchTags` is tested with a hand-written fake.
* `internal/cli/branchtags` chooses the inputs (CI variables, else the repository), defines the `Repository` interface it consumes, and renders the three lines. It is tested with a fake `Repository`.
* **Short commit uses `git`.** `Head` calls `gitcli.ShortCommit`, which runs `git rev-parse --short HEAD`. This is the escape hatch, recorded in `internal/platform/git/gitcli`, which runs git through `internal/platform/execx`, the only package that imports `os/exec`: git's length grows with the object count (7 minimum, 8 or more from roughly 16k objects) and lengthens on ambiguity, and go-git has no equivalent. Reproducing the rule by hand would be guesswork, and the faithful default has to match git.
* **Creator date.** git's `creatordate` is the tagger date for an annotated tag and the commit date for a lightweight tag. `NewestFirst` reproduces that, and breaks equal dates by ref name ascending, as git does. The adapter tests compare the order with `git tag --sort=-creatordate` whenever a `git` binary is present, and it matched on every case tried (verified for lightweight, annotated, mixed and equal-date tags).
* Shallow clones (with and without tags) and detached HEAD work with go-git, and are covered by the adapter tests. Shallow clones are built with real `git`, so that test skips when it is missing.

## 4. Test plan

Test first, per [TESTING](TESTING.md) and [`ai/red-green-commits.md`](../ai/red-green-commits.md).

**`ResolveBranchTags`, table test with a fake `Tags`:**

| Ref | Sha | Tags (newest first) | Result |
|---|---|---|---|
| `refs/heads/main` | `abcdef1234567` | none | `head`, `main-abcdef1`, `` |
| `refs/heads/main` | `abcdef1234567` | `main-1111111`, `v1.0.0`, `main-0000000` | `head`, `main-abcdef1`, `main-1111111` |
| `refs/heads/release/v2.x` | `abcdef1234567` | `v2.x-head-2222222`, `main-1111111` | `v2.x-head`, `v2.x-head-abcdef1`, `v2.x-head-2222222` |
| `refs/heads/release/v2.x` | `abcdef1234567` | `v2Ax-head-2222222` | default: previous is `v2Ax-head-2222222` (regex `.`); with the opt-in, empty |
| `refs/heads/release/v2.x` | `abcdef1234567` | `v2.x-head-abcdef1` | previous is the current commit's own tag (decided) |
| `refs/heads/feature/x` | any | any | `ErrUnsupportedRef` |
| `refs/tags/v1.0.0` | any | any | `ErrUnsupportedRef` |
| `refs/pull/12/merge` | any | any | `ErrUnsupportedRef` |
| `refs/heads/release/` | `abcdef1234567` | none | default: `-head`, `-head-abcdef1`, ``; with the opt-in, `ErrUnsupportedRef` |
| `refs/heads/main` | `abc` (short) | none | static tag uses what is there, no panic |

The `Sha` column is the commit as `ResolveBranchTags` receives it (already abbreviated). `Short` is tested separately: `abcdef1234567` gives `abcdef1`, and anything shorter is returned whole. A longer local commit such as `abcdef12` is used as given.

A `Tags` error is returned, not swallowed.

**go-git adapter, against temporary repositories built in the test** (`testdata/` for any golden output):

* Annotated and lightweight tags order by `creatordate` as git does, including equal dates.
* Current ref and short commit on a branch, and on detached HEAD (error). Discovery from a subdirectory and in a worktree is asserted only with its improvement flag, since the faithful default requires a `.git` directory in the current directory.
* A shallow clone with and without tags.

**`internal/cli/branchtags`, with a fake `Repository`:** CI inputs (no `.git` required, no repository means no tags), local inputs (also when only one of `GITHUB_REF` and `GITHUB_SHA` is set, or both are empty), a longer local commit used as given, every error path (not a repository, detached HEAD, unsupported ref, tag listing failure), and `Render` keeping the empty `prev_static_tag=` line.

**`cmd`, wiring only, through the root command:**

* CI variables set: the three lines on stdout.
* Unsupported ref: the message on stdout only, nothing on stderr, and an error (exit 1). Not a repository: an error.
* Each improvement flag changes only its own case, and leaves the default output untouched.

**Parity (optional, `test/integration/`):** build the same temporary repositories, run the original script and the built `gero`, and compare stdout. This is also the check for the unverified go-git ordering. All default-mode rows must match the script. The improvement rows from section 6 are asserted separately. The original script lives there as a fixture.

## 5. Open questions

* **Branch families as config.** Supported ref patterns default to `main` and `release/*`, and an optional config could override them, possibly in the repo-level `gero.yaml`. [ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md) currently allows only `WorkflowsConfig` in a target repo's `gero.yaml`, so this needs a decision there first. Unconfigured behavior stays the script's.
* **Flags.** Whether `--ref`, `--sha` and `--repo <path>` are worth adding to run the command without the CI environment. Undecided; the environment rule stays the default.
* **Default local short commit.** Decided: ask git through the escape hatch (section 3). Revisit only if a go-git abbreviation that matches git appears, since the subprocess inherits the environment and git config.
* **Opt-in flag names and grouping.** One flag per improvement in section 6, or a single flag that turns on the related ones.
* **Error reporting.** The stderr row in section 6 touches a global rule ([CLI_OUTPUT](CLI_OUTPUT.md): diagnostics to stderr). Whether it still needs a per-command opt-in.

## 6. Breaking changes that will improve the script after port

Approved improvements. None of them is part of the faithful port. During the port each is opt-in behind a flag, so the default output never changes. Making any of them the default later is a deliberate breaking change, planned with a release, since workflows read these results. The same section exists in [CHECK_SEMVER](CHECK_SEMVER.md).

| Improvement | Script and default | Improved | Breaking if made default |
|---|---|---|---|
| **Literal prefix match** | Regex, so `.` in `v2.x-head-` matches any character | Literal prefix | Yes: the previous tag can change |
| **Fixed 7-character short commit** | Local: git `--short`, 7 or more characters | Always the first 7 characters, as in CI, so tag names do not depend on repository size | Yes: the static tag can change locally |
| **Reject empty release version** | `release/` gives `branch_tag=-head` | Rejected as an unsupported ref | Yes: output becomes an error |
| **Unsupported ref and detached HEAD exit code** | Exit 1, and git's 128 for detached HEAD | Exit 2 (validation error) with a message naming the ref, and a hint to set `GITHUB_REF` for detached HEAD | Yes: scripts checking exact codes |
| **Diagnostics to stderr** | The script prints its messages to stdout | Messages on stderr, data only on stdout ([CLI_OUTPUT](CLI_OUTPUT.md)) | Yes for anyone reading messages from stdout |
| **Discover repository upward** | `.git` must be a directory in the current directory | Finds the repository from the current directory upward, so subdirectories and worktrees work | No: only turns failures into results |
| **`--json`** | Not available | `{"branchTag", "branchStaticTag", "prevStaticTag"}` | No: new format, default unchanged |
| **Branch families** | `main` and `release/*` only | Configurable patterns with those defaults | No: only changes behavior when configured |
