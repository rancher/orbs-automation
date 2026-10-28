# CLI Config

**Status:** agreed. Items marked **(leaning)** are proposals. Nothing is built yet.

How a single invocation of `gero` gets its settings, and how it finds the central application config. Flags and the command tree are in [CLI_FRAMEWORK](CLI_FRAMEWORK.md).

## 1. Terms

Two kinds of config exist, and the docs keep them apart.

* **Application config** describes the world `gero` operates on: projects, Rancher lines, external projects, and per-repo workflow settings. It is the multi-document YAML in `config/` and a target repo's `gero.yaml` (IMPLEMENTATION §7). Versioned by `apiVersion`.
* **CLI config** describes how one invocation behaves: where to find the application config, which remote counts as upstream, output format, verbosity, host. It comes from flags, environment variables and an optional user file (section 2). It never holds application facts, and it never holds secrets.

Code mirrors the split: `internal/config` for application config, `internal/clicfg` for CLI config ([PROJECT_LAYOUT](PROJECT_LAYOUT.md)).

## 2. Sources and precedence

Highest wins:

1. **Flags**, detected by whether the flag was explicitly set (cobra's `Flags().Changed()`, read in `cmd/`) so a flag's default never masks a lower source. `internal/clicfg` does not import cobra ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 3), so `cmd/` passes it a framework-neutral flag source: the values, and which were set.
2. **Environment**, `GERO_*`.
3. **User file.**
4. **Built-in defaults.**

```
 flags  >  GERO_* env  >  user file  >  defaults
                 \           |           /
                  v          v          v
              internal/clicfg  ->  Settings (resolved, immutable)
                                        |
                                  command RunE
```

`GH_TOKEN` / `GITHUB_TOKEN` keep their meaning and are consumed by the GitHub access layer (IMPLEMENTATION §3), not by this loader.

## 3. User file and keys

### User file

* Path: `$XDG_CONFIG_HOME/gero/config.yaml`, else `~/.config/gero/config.yaml`. `GERO_CONFIG` overrides the path. We do not use `os.UserConfigDir` because it points at `~/Library/Application Support` on macOS. `~/.gero` was considered and rejected to match `gh`'s layout.
* Optional. A missing file is fine; a malformed file, or one with an unknown key, is a loud error naming the file and key.
* No secrets. Tokens stay with `gh` or the environment.
* **CI ignores it** unless `GERO_CONFIG` is set explicitly, so a runner never behaves differently because of a stray file.
* The main use is per-developer behavior that must not leak into the shared tool. The motivating case: locally `origin` is a developer's fork, while in CI `origin` is the real source, so `upstream` tells `gero` which remote to treat as the real source.

### Keys and environment

Candidate set, names not final **(leaning)**:

| File key | Env var | Flag | Meaning |
|---|---|---|---|
| `configDir` | `GERO_CONFIG_DIR` | `--config-dir` | Where the central application config is (section 4) |
| `upstream` | `GERO_UPSTREAM` | `--upstream` | Remote or repo treated as the real source |
| `output` | `GERO_OUTPUT` | `--output`, `--json` | `text` (default) or `json` |
| `host` | `GERO_HOST` | `--host` | GitHub host, if we ever target non-github.com (IMPLEMENTATION §3) |
| n/a | `GERO_LOG` | `-v` | Log level ([CLI_OUTPUT](CLI_OUTPUT.md) section 2) |
| n/a | `GERO_CONFIG` | n/a | Path of the user file |

Env and flag names derive from the file key by one rule, so the table is a check, not a source of truth. The loader is table-tested for precedence and for unknown keys ([TESTING](TESTING.md)).

## 4. Locating the central application config

Commands that read the central `config/` find it in this order:

1. `--config-dir`
2. `GERO_CONFIG_DIR`
3. `configDir` in the user file
4. `./config`, when the working directory is a checkout of the central repo

If none resolves, the command fails with a message listing these four places. There is no runtime fetch of config from another branch (IMPLEMENTATION §7, "Avoid"). In CI the central repo is checked out and `GERO_CONFIG_DIR` or `--config-dir` is set explicitly in the workflow.

## Open questions

* The shape of the framework-neutral flag source that `cmd/` hands to `internal/clicfg`, and how commands receive resolved flag values. Designed with the loader, together with [PROJECT_LAYOUT](PROJECT_LAYOUT.md)'s question on how flags reach `internal/cli/`.
* Final names for the user-file keys.
