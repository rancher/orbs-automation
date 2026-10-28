# check-semver

**Status:** agreed. Items marked **(leaning)** are proposals. Nothing is built yet: stubs exist for `gero repo check-semver` and an empty `internal/semver` package.

Port of the `.github/scripts/check-semver` bash script that many ORBS repos carry. The goal is to prove the script can live in `gero`, not to delete the original. The default behavior is a **faithful port**: same input, same output keys and order, same results. Every approved improvement is listed in section 6 and is opt-in. Tag classification as a shared library is also called for by [IMPLEMENTATION §4](../IMPLEMENTATION.md).

## 1. What the script does today

Input is one tag, from the first argument or else `GITHUB_REF_NAME`. The script strips one leading `v` and prints two lines, in this order, exit 0:

```
HAS_PRERELEASE=<true|false>
HAS_BUILD_META=<true|false>
```

* `HAS_PRERELEASE` is true when the tag matches `^[0-9]+\.[0-9]+\.[0-9]+-`.
* `HAS_BUILD_META` is true when the tag contains a `+` anywhere.
* With no tag it prints an error to stderr and exits 1.
* It never validates the input: `foo` prints `false` for both and exits 0.

Quirks of that regex: `1.2.3-` counts as a prerelease, `01.2.3-rc` is accepted despite the leading zero, `V1.2.3` is not stripped and `vv1.2.3` loses only one `v`, and `+` anywhere counts as build metadata even in text that is not a version.

## 2. Faithful port (the default)

What makes `gero repo check-semver` a faithful port. Each point matches the script exactly:

1. **Input.** The argument, else `GITHUB_REF_NAME`. An empty argument falls through to the variable. Extra arguments are ignored.
2. **Prefix.** One leading lowercase `v` is stripped. Uppercase `V` is not, and `vv1.2.3` loses one `v`. The SemVer FAQ says `v1.2.3` is not itself a semantic version and calls a `v` prefix a common convention, so it gives no rule for `V`; the script's behavior stands.
3. **`HAS_PRERELEASE`.** The same regex, `^[0-9]+\.[0-9]+\.[0-9]+-`, applied after stripping.
4. **`HAS_BUILD_META`.** True when the tag contains a `+` anywhere.
5. **Output.** Exactly two `KEY=value` lines on stdout, `HAS_PRERELEASE` first, values `true` or `false`, so a workflow can append them to `$GITHUB_OUTPUT`. Nothing else is printed.
6. **No validation.** Any text is accepted, SemVer or not. One-off QA builds, hotfixes and similar non-SemVer tags are legitimate input and exit 0.
7. **Missing tag.** Neither argument nor `GITHUB_REF_NAME`: an error on stderr and exit 1.

```
gero repo check-semver [tag]
```

* **Context:** either (read-only, no credentials).
* The command reads the environment and passes the string down. The library never reads it.

## 3. Library

`internal/semver` exposes a pure function, shaped like:

```go
type Info struct {
    Input         string
    Version       string // Input with one leading "v" stripped
    HasPrerelease bool
    HasBuildMeta  bool

    // Set only with WithGrammar (section 6).
    IsSemver            bool
    Major, Minor, Patch uint64
    Prerelease          string
    BuildMeta           string
}

func Classify(tag string, opts ...Option) Info
```

* The default call reproduces the script (section 2). `WithGrammar()` switches to the SemVer 2.0.0 grammar (section 6).
* No I/O, no environment, no error return: non-SemVer is a result, not a failure.
* Implemented with the stdlib `regexp`, with no dependency **(leaning)**. The script's regex must be reproduced exactly, which a strict SemVer library would not do. A library can be adopted later for the grammar mode if [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md) allows it and the tests still hold.
* It is the one place tag classification lives. `lines resolve`, `reconcile` and the prerelease policy call it instead of reimplementing the shell check.

## 4. Test plan

Test first, per [TESTING](TESTING.md) and [`ai/red-green-commits.md`](../ai/red-green-commits.md).

**`internal/semver`, table test of `Classify`.** The first two result columns are the default (the script's results). The last three are the result with `WithGrammar()`. Rows where the two differ are the improvements from section 6:

| Input | Pre | Build | | IsSemver | Pre | Build |
|---|---|---|---|---|---|---|
| `1.2.3` | false | false | | true | false | false |
| `v1.2.3` | false | false | | true | false | false |
| `1.2.3-rc.1` | true | false | | true | true | false |
| `v1.2.3-beta` | true | false | | true | true | false |
| `1.2.3+build.1` | false | true | | true | false | true |
| `1.2.3-rc.1+build.1` | true | true | | true | true | true |
| `1.2.3+build-5` | false | true | | true | false | true |
| `1.2.3-hotfix-1` | true | false | | true | true | false |
| `1.2.3-` | **true** | false | | false | **false** | false |
| `01.2.3-rc` | **true** | false | | false | **false** | false |
| `1.2.3-rc..1` | **true** | false | | false | **false** | false |
| `hotfix+urgent` | false | **true** | | false | false | **false** |
| `1.2-rc.1` | false | false | | false | false | false |
| `1.2.3.4-rc` | false | false | | false | false | false |
| `V1.2.3-rc` | false | false | | false | false | false |
| `vv1.2.3-rc` | false | false | | false | false | false |
| `qa-build-42` | false | false | | false | false | false |
| `` (empty) | false | false | | false | false | false |

Also assert `Major`, `Minor`, `Patch`, `Prerelease` and `BuildMeta` on the grammar-mode SemVer rows.

**`cmd`, in-process (captured stdout, stderr, exit code):**

* Argument given: exactly the two default lines, key order included, and nothing else.
* No argument, `GITHUB_REF_NAME` set: uses it. Both set: argument wins. Empty argument with the variable set: uses the variable.
* Neither set: empty stdout, message on stderr, exit 1.
* Non-SemVer tag: the two lines, exit 0.
* Each improvement flag changes only its own behavior, and leaves the default output untouched.

**Parity (`test/integration/`):** run the original script and the built `gero` over every row above in default mode and compare stdout and exit code. All rows must match, including the ones that are quirks. The original script lives there as a fixture, not in the root module.

## 5. Open questions

* **Opt-in flag names and grouping.** One flag per improvement in section 6, or one flag (for example for the SemVer grammar) that turns on the related ones together.
* **Extra env format name.** `--output env-full` or `env-extra`. Decided with the CLI output work, since it adds a value to `--output`.
* **Strict mode.** Whether `--strict` is worth adding (section 6).
* **Non-SemVer `+`.** Whether the grammar mode should keep the script's "any `+`" meaning for non-SemVer text. Deferred until a real workflow depends on it.

## 6. Breaking changes that will improve the script after port

Approved improvements. None of them is part of the faithful port. During the port each is opt-in behind a flag, so the default output never changes. Making any of them the default later is a deliberate breaking change, planned with a release, since workflows read these results. The same section exists in [BRANCH_TAGS](BRANCH_TAGS.md).

| Improvement | Script and default | Improved | Breaking if made default |
|---|---|---|---|
| **SemVer grammar** | Regex only. `1.2.3-`, `01.2.3-rc` and `1.2.3-rc..1` count as prereleases | Input valid under SemVer 2.0.0 is classified by the real grammar, and anything else is non-SemVer. Non-SemVer tags stay legal | Yes: `HAS_PRERELEASE` flips for those inputs |
| **Non-SemVer `+`** | `hotfix+urgent` has `HAS_BUILD_META=true` | Build metadata is a SemVer concept, so non-SemVer text reports `false` | Yes: `HAS_BUILD_META` flips |
| **`IS_SEMVER` key** | Two keys only | A third env line, `IS_SEMVER=true\|false`, after the two original keys | Yes for strict consumers of the file, otherwise additive |
| **Missing-tag exit code** | Exit 1 | Exit 2, a usage error ([CLI_FRAMEWORK](CLI_FRAMEWORK.md) section 4) | Yes: scripts checking for exactly 1 |
| **`--json`** | Not available | Full detail: `input`, `version`, `isSemver`, `hasPrerelease`, `hasBuildMeta`, `major`, `minor`, `patch`, `prerelease`, `buildMeta` (`major`, `minor`, `patch` omitted when not SemVer) | No: new format, default unchanged |
| **Extra env keys** | Not available | Opt-in env format with `VERSION`, `MAJOR`, `MINOR`, `PATCH`, `PRERELEASE`, `BUILD_META` | No: new format |
| **`--strict`** (open) | Never fails on bad input | Exit 2 on non-SemVer input, for release-only workflows | No: purely additive |

`--json` field names are stable once released ([CLI_OUTPUT](CLI_OUTPUT.md) section 1). `--output env` is the explicit name of the default format.
