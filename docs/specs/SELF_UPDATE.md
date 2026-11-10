# Self-update

**Status:** implemented.

`gero doctor self-update` replaces the running `gero` with the newest GitHub release. It is for an engineer's workstation: it is `workstation-only` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)), refused in CI and in `gero repl` and hidden from help there. The mechanics are `github.com/mallardduck/binup`, behind `internal/platform/selfupdate` ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)); the flow and its text are `internal/cli/selfupdate`.

## The flow

Check, guard, ask, swap, in that order, as `binup` lays it out.

1. **Check.** Ask GitHub for the newest release and compare it with the running version, which is injected at build time ([`cmd/version`](../../cmd/version/version.go)). Nothing is downloaded. Which releases count: a pre-release `gero` considers pre-releases (GitHub's "latest" is always stable, so it would have nowhere to go); a stable one considers only stable releases. `--pre-release` lets a stable one consider pre-releases too, and `--pre-release=false` keeps a pre-release to stable ones. A development build (`dev`) is never offered an update, and is told so. `--check` stops here, and so does `--dry-run`, which also names what it would install.
2. **Guard.** A binary that a package manager owns (an `GERO_INSTALL_MODE` set by the package, or a Homebrew or apt path) or that the user cannot write is not touched: the command says a release exists and where to update instead, and exits 0. This comes before the question, so there is never a "install it? [y/N]" that cannot be acted on.
3. **Ask.** One question, "Update gero X to Y? [y/N]", as every write command asks once ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md)). `--yes`, or `GERO_NON_INTERACTIVE`, answers yes. Without a terminal and without the flag it fails before downloading, naming the flag. A pre-release is said to be one.
4. **Swap.** Download, verify, and replace the file atomically; if the swap fails the old binary is put back.

## What proves the release is ours

The level is the highest `binup` offers, and there is no flag that lowers it.

* A checksum proves a download is intact, not that it is authentic: whoever can replace the release can replace `checksums.txt` too. So each release carries keyless [Sigstore](https://sigstore.dev) bundles, `<file>.sigstore.json`, for **the checksum manifest and for every archive**.
* Both are verified. The manifest's signature makes its hashes trustworthy, the archive must match its hash, and the archive's own signature authenticates it directly, so neither is trusted on the strength of the other.
* The signer is **pinned**: the certificate must name `release.yaml` of `rancher/orbs-automation`, run for a `v` tag, issued by GitHub Actions' OIDC (`selfupdate.SignerPattern` and `SignerIssuer`). A valid signature from a fork, another workflow, a branch or another repository proves nothing and is refused. The pattern is anchored at both ends, which `sigstore.NewVerifier` also insists on. The verifier and the pinned identity live in `internal/platform/releasesign`, which the fetch of the central config bundle shares ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)): one definition of "ours".
* It **fails closed.** A release without the bundles fails with `ErrSignatureMissing`, a bundle that does not verify with `ErrSignatureInvalid`; the binary on disk is left as it was. Nothing falls back to the checksum alone.
* The public-good Sigstore root is fetched over TUF unless one is supplied, so verifying needs the network beyond GitHub.

The same GitHub build-provenance attestation the release workflow makes (`actions/attest-build-provenance`) is not checked by `binup`; it is for people who verify by hand with `gh attestation verify`.

## What the release must publish

The release workflow uses the `cosign` and `gh` that the container image provides, and after GoReleaser has built the draft it signs each archive, `checksums.txt` and the config bundle with `cosign sign-blob --yes --bundle=<file>.sigstore.json`, uploads the bundles to the draft, and fails the release before it is published if one is missing. The signing is in the workflow and not in `.goreleaser.yaml` because GoReleaser has no way to skip it on a snapshot, and a snapshot run (locally, or the dry run in CI) would then ask for a Sigstore login. The assets are `gero_<os>_<arch>.tar.gz`, `checksums.txt` and `gero-config-bundle.yaml`, and a `.sigstore.json` beside each. The workflow grants `id-token: write`, which keyless signing needs. The archive name has no version in it (`{name}_{os}_{arch}.{ext}`), so `selfupdate.AssetTemplate` says that; a change to `name_template` has to change both.

## Cost

Sigstore verification is the largest dependency gero has, by size of binary. It is accepted for this command alone, because a wrong answer here installs attacker code on an engineer's machine.

## Not built yet

* Tests of the signature itself against a real release: the tests drive the real `binup` and `ghreleases` code and the real signer pattern, with a stand-in verifier for the signature, in memory and with no network.
* `gero doctor --online` warning that a newer release exists.
* `--version`, to install a named release or roll back: built only if someone asks.
* An offline path (a `trusted_root.json` supplied by file), though `selfupdate.Config.TrustedRoot` takes one: built only if someone asks.
* Windows: gero is released for macOS and Linux only, and a build is made only if someone asks.

## Open questions

None yet.
