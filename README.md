# ORBS Automation

Release and process automation for the Rancher ORBS team.

The aim: a release of our products moves through the whole chain of repositories the same way every time, with every step checked and recorded, and automated wherever it is safe to be, by one tested tool in place of copies of shell scripts that drift apart.

> **Status: pre-1.0.** Under active development. Run `gero --help` for what exists today.

## What's here

### `gero`, the command line tool

`gero` sits next to `gh`, reuses your `gh` login or `GH_TOKEN`, and works on a workstation or in CI.

- **Ships a release downstream.** `gero ship` opens version-bump pull requests from your own clone, one for each branch that needs one, and closes the ones a newer release replaces.
- **Says where releases stand.** `gero repo status` shows a project's releases and what each project that takes from it holds, and `gero lines resolve` shows how the Rancher release lines connect across projects.
- **Handles the pieces by hand.** `gero repo branch`, `gero repo tag` and `gero repo clone`.
- **Sets up and checks your machine.** `gero init`, `gero doctor` and `gero shell install`.
- **Has a prompt.** `gero repl` runs the same commands a line at a time.

Output is styled in a terminal, and `--no-color` or `NO_COLOR` turns that off.

### The `gero` app config

`config/` describes the world `gero` works on, as YAML documents: which projects we automate, what each one releases (streams and artifacts), where each repository holds a release of another (holdings), the steps that run around a bump (edits), and the Rancher release lines with their lifecycle dates. `gero config validate` checks it, and JSON Schemas in `generated/jsonschema/` let an editor complete and check the files.

Each `gero` release also carries a compiled copy of this config, so a workstation needs no checkout of this repo.

### Release tracking issues (planned)

Issues kept up to date automatically, so everyone can see where a release stands. Not built yet.

## Try it

Install the latest release of `gero` for macOS or Linux (amd64 or arm64) from the [releases page](https://github.com/rancher/orbs-automation/releases). Each release has an archive named `gero_<os>_<arch>.tar.gz` and a `checksums.txt`:

```sh
# pick your platform: darwin or linux, and amd64 or arm64
gh release download --repo rancher/orbs-automation --pattern 'gero_linux_amd64.tar.gz' --pattern checksums.txt
sha256sum --ignore-missing -c checksums.txt     # on macOS: shasum -a 256 --ignore-missing -c checksums.txt
tar xzf gero_linux_amd64.tar.gz
install -m 0755 gero ~/.local/bin/gero          # or any directory on your PATH
```

`gh release download` takes the latest stable release. Put a tag first (`gh release download v0.1.0 --repo ...`) for a pre-release or an older one. Without `gh`, download the same two files from the release page. Releases are signed with Sigstore, and `gero doctor self-update` checks the signatures before it replaces the binary, so after the first install it keeps itself current.

Then:

```sh
gero init          # writes your user file, and asks for what it needs
gero doctor        # checks your setup and says how to fix what is wrong
gero --help
gero repl          # an interactive prompt over the same commands
```

To build from source instead, with Go installed: `go build -o gero .` in a checkout. From a checkout `gero` also finds `./config` itself.

## Contributing

How we write code, tests, commits and docs is in [docs/contributing](docs/contributing/README.md).
