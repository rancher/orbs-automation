# branch-tags

**Status:** draft. Nothing is built yet: stubs exist for `gero repo branch-tags` and an empty `internal/gittags` package.

Port of the `.github/scripts/branch-tags.sh` bash script that many ORBS repos carry. The goal is to prove the script can live in `gero`, not to delete the original. The default behavior is a **faithful port**: same inputs, same output keys, case and order, same results. Every approved improvement is listed in section 6 and is opt-in. Git access uses go-git behind an interface ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md), IMPLEMENTATION §3).

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

What makes `gero repo branch-tags` a faithful port. Each point matches the script:

```
gero repo branch-tags
```

1. **Inputs.** When both `GITHUB_REF` and `GITHUB_SHA` are non-empty, the ref is `GITHUB_REF` and the commit is the first 7 characters of `GITHUB_SHA`. Otherwise the repository in the current directory is read, which must have a `.git` directory there. The command reads the environment and passes values down. The library never reads it.
2. **Branch rules.** Only `refs/heads/main` and `refs/heads/release/<version>` are supported, with the tags in the table in section 1. `<version>` is not validated, so `release/` gives `branch_tag=-head`.
3. **Previous tag.** The newest tag by `creatordate` whose name starts with the prefix, matched as a regex as the script does, so the `.` in `v2.x-head-` is a wildcard. No match is not an error: `prev_static_tag=` is empty.
4. **Previous means previous in sequence.** The newest matching tag counts even when it already points at the current commit. This is decided, not a quirk to fix: the question is what came before in the tag sequence, not what came before on a different commit.
5. **Local short commit.** Matches `git rev-parse --short HEAD`, which is 7 or more characters depending on repository size. How go-git reproduces this is an open question.
6. **Output.** Exactly three lowercase `key=value` lines on stdout, in the script's order: `branch_tag`, `branch_static_tag`, `prev_static_tag`. The key case differs from [CHECK_SEMVER](CHECK_SEMVER.md) (upper case) because each matches the script it replaces and workflows read those names.
7. **Errors.** An unsupported ref and a missing `.git` exit 1. A detached HEAD with no CI environment exits non-zero, as the script does (the script's 128 comes from git, which go-git will not reproduce exactly). Error message text is not part of the contract.
8. **Context:** either (read-only, no credentials).

## 3. Library

`internal/gittags` resolves the result from a ref, a commit and a tag source. The interface is defined in the package that consumes it ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 3):

```go
type Result struct {
    BranchTag       string
    BranchStaticTag string
    PrevStaticTag   string
}

// Tags lists tag names, newest first by creator date.
type Tags interface {
    NewestFirst() ([]string, error)
}

var ErrUnsupportedRef = errors.New("unsupported branch pattern")

func Resolve(ref, sha string, tags Tags) (Result, error)
```

* `Resolve` is pure apart from the `Tags` call. It takes `sha` in full and uses the first 7 characters. The opt-ins are options on the call, so the default is the faithful behavior.
* A second small piece reads the current ref and commit from a repository, and implements `Tags`, using go-git. It is the only code that imports go-git, so `Resolve` is tested with a hand-written fake.
* **Creator date.** git's `creatordate` is the tagger date for an annotated tag and the commit date for a lightweight tag. The go-git side has to reproduce that, plus git's tie-breaking for equal dates **(unverified)**. This is checked against real `git` in the parity test, since `branch-tags` depends on it.
* Shallow clones and detached HEAD with go-git also need confirming **(unverified)**, as noted in IMPLEMENTATION §3.

## 4. Test plan

Test first, per [TESTING](TESTING.md) and [`ai/red-green-commits.md`](../ai/red-green-commits.md).

**`Resolve`, table test with a fake `Tags`:**

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

A `Tags` error is returned, not swallowed.

**go-git adapter, against temporary repositories built in the test** (`testdata/` for any golden output):

* Annotated and lightweight tags order by `creatordate` as git does, including equal dates.
* Current ref and short commit on a branch, and on detached HEAD (error). Discovery from a subdirectory and in a worktree is asserted only with its improvement flag, since the faithful default requires a `.git` directory in the current directory.
* A shallow clone with and without tags.

**`cmd`, in-process (captured stdout, stderr, exit code):**

* `GITHUB_REF` and `GITHUB_SHA` set: exact default output, key order included. Only one of them set: falls back to the repository.
* Unsupported ref and not a repository: exit 1. Detached HEAD with no CI environment: non-zero exit.
* Each improvement flag changes only its own case, and leaves the default output untouched.

**Parity (optional, `test/integration/`):** build the same temporary repositories, run the original script and the built `gero`, and compare stdout. This is also the check for the unverified go-git ordering. All default-mode rows must match the script. The improvement rows from section 6 are asserted separately. The original script lives there as a fixture.

## 5. Open questions

* **Branch families as config.** Supported ref patterns default to `main` and `release/*`, and an optional config could override them, possibly in the repo-level `gero.yaml`. [ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md) currently allows only `WorkflowsConfig` in a target repo's `gero.yaml`, so this needs a decision there first. Unconfigured behavior stays the script's.
* **Flags.** Whether `--ref`, `--sha` and `--repo <path>` are worth adding to run the command without the CI environment. Undecided; the environment rule stays the default.
* **Default local short commit.** To be faithful, the default should match `git rev-parse --short`. go-git has no equivalent that lengthens on ambiguity, so either reproduce git's rule (minimum 7, longer when the prefix is ambiguous) or ask git through the `os/exec` fallback allowed in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md). Settled with the first adapter tests.
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
