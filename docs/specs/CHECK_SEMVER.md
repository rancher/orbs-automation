# check-semver

The command is `gero semver check`. `check` is the lenient script port described here. Strict SemVer operations, if they come, are separate verbs under `gero semver` and do not change `check` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md) section 2).

**Status:** implemented.

Port of the `.github/scripts/check-semver` bash script that many ORBS repos carry. The goal is to prove the script can live in `gero`, not to delete the original. The behavior is a **faithful port**: same input, same output keys and order, same results. Tag classification as a shared library is also called for by the [release rules](../IMPLEMENTATION.md#release-rules).

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

## 2. Faithful port

What makes `gero semver check` a faithful port. Each point matches the script exactly:

1. **Input.** The argument, else `GITHUB_REF_NAME`. An empty argument falls through to the variable. Extra arguments are ignored.
2. **Prefix.** One leading lowercase `v` is stripped. Uppercase `V` is not, and `vv1.2.3` loses one `v`. The SemVer FAQ says `v1.2.3` is not itself a semantic version and calls a `v` prefix a common convention, so it gives no rule for `V`; the script's behavior stands.
3. **`HAS_PRERELEASE`.** The same regex, `^[0-9]+\.[0-9]+\.[0-9]+-`, applied after stripping.
4. **`HAS_BUILD_META`.** True when the tag contains a `+` anywhere.
5. **Output.** Exactly two `KEY=value` lines on stdout, `HAS_PRERELEASE` first, values `true` or `false`, so a workflow can append them to `$GITHUB_OUTPUT`. Nothing else is printed.
6. **No validation.** Any text is accepted, SemVer or not. One-off QA builds, hotfixes and similar non-SemVer tags are legitimate input and exit 0.
7. **Missing tag.** Neither argument nor `GITHUB_REF_NAME`: an error on stderr and exit 1.

```
gero semver check [tag]
```

* **Context:** either (read-only, no credentials).
* The command reads the environment and passes the string down. The library never reads it.
* **Layering** ([PROJECT_LAYOUT](PROJECT_LAYOUT.md)). `cmd/semver.go` holds the cobra command and a short `RunE`. The logic is in `internal/cli/checksemver`, with no cobra: `ResolveTag(args, getenv) (string, error)` picks the tag (argument, else `GITHUB_REF_NAME`, else `ErrNoTag`), `Classify(tag) semver.Info` wraps the library call, and `Render(info semver.Info) string` formats it as the two `KEY=value` lines. `cmd/` cannot import `internal/platform/semver`, so `checksemver.Classify` is the only place the command calls the library, and `Render` is a pure formatter over `Info`.

## 3. Library

`internal/platform/semver` has two halves that stay apart. `Classify`, below, is the lenient script port and accepts any text. The rest (`Parse`, `Version`, `Compare`, `Series`, `NextStable` and `NextPrerelease`) reads a tag with the SemVer grammar, for ordering, series and choosing the next tag, and is what `repo tag`, `repo status` and the release rules use. `Classify` is a pure function, shaped like:

```go
type Info struct {
    Input         string
    Version       string // Input with one leading "v" stripped
    HasPrerelease bool
    HasBuildMeta  bool

    // Always zero: the SemVer grammar is read by Parse, not by Classify.
    IsSemver            bool
    Major, Minor, Patch uint64
    Prerelease          string
    BuildMeta           string
}

func Classify(tag string) Info
```

* The call reproduces the script (section 2). `Parse` reads the SemVer 2.0.0 grammar.
* No I/O, no environment, no error return: non-SemVer is a result, not a failure.
* Implemented with the stdlib `regexp`, with no dependency. The script's regex must be reproduced exactly, which a strict SemVer library would not do.
* It is the one place the script's classification lives. Self-update uses it to tell a pre-release `gero`, and the commands that order or choose tags use `Parse` instead of reimplementing the shell check.

## 4. Test plan

Test first, per [TESTING](TESTING.md) and [`contributing/red-green-commits.md`](../contributing/red-green-commits.md).

**`internal/platform/semver`, table test of `Classify`.** The results are the script's, including the ones the SemVer grammar would reject (in bold):

| Input | Pre | Build |
|---|---|---|
| `1.2.3` | false | false |
| `v1.2.3` | false | false |
| `1.2.3-rc.1` | true | false |
| `v1.2.3-beta` | true | false |
| `1.2.3+build.1` | false | true |
| `1.2.3-rc.1+build.1` | true | true |
| `1.2.3+build-5` | false | true |
| `1.2.3-hotfix-1` | true | false |
| `1.2.3-` | **true** | false |
| `01.2.3-rc` | **true** | false |
| `1.2.3-rc..1` | **true** | false |
| `hotfix+urgent` | false | **true** |
| `1.2-rc.1` | false | false |
| `1.2.3.4-rc` | false | false |
| `V1.2.3-rc` | false | false |
| `vv1.2.3-rc` | false | false |
| `qa-build-42` | false | false |
| `` (empty) | false | false |

**`internal/cli/checksemver`, plain table tests (a fake `getenv`, no cobra, no `t.Setenv`):**

* `ResolveTag`: no argument with `GITHUB_REF_NAME` set uses it. Both set: argument wins. Empty argument with the variable set: uses the variable. Extra arguments are ignored. Neither set: `ErrNoTag`.
* `Render`: given an `Info`, exactly the two lines, key order included, and nothing else. The extra `Info` fields never reach the output. A non-SemVer tag gets the two lines too.

**`cmd`, wiring only, through the root command (captured stdout, error):**

* Argument given: the two lines on stdout, and extra arguments are accepted.
* Neither argument nor `GITHUB_REF_NAME`: empty stdout and an error, which `Execute` turns into exit 1.

## 5. Open questions

None yet.
