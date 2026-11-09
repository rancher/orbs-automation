# Docs

New here? Read [VISION](VISION.md), then [ARCHITECTURE_DESIGN](ARCHITECTURE_DESIGN.md), then run `gero --help`. The rest is reference.

Each kind of document answers one question, and each kind changes at a different speed. That is the whole organizing idea.

| Where | Answers | Changes |
|---|---|---|
| [VISION](VISION.md) | Why does `gero` exist? | Rarely |
| [ARCHITECTURE_DESIGN](ARCHITECTURE_DESIGN.md) | What is the product shape? | When the design changes |
| [IMPLEMENTATION](IMPLEMENTATION.md) | What does the tool do, and what did we decide about it? | When a decision is made or reversed |
| [PHASES](PHASES.md) | What gets built, in what order? | When the plan changes |
| [design/](design/CHAIN_MODEL.md) | What are the config model, the layers and the engines, and what did we decide? Where a spec disagrees, the newer `design/` account wins | While the design moves, then it is folded into specs and `doc.go` files |
| [process/](process/README.md) | How is a Rancher release made, and how does each project take part? Drafts, not yet reviewed by the release team | When the process changes |
| [specs/](specs/README.md) | How is one part built, and why that way? | While the part is being built, then it settles |
| [projects/](projects/README.md), [references/](references/frameworks-automation.md) | What do we know about a repo we automate, or a tool we learned from? Notes from reading the repos | When the repo changes |
| [contributing/](contributing/red-green-commits.md) | How do we work on this repo (contributor guidance)? | Rarely |
| [`../config/README.md`](../config/README.md) | What do the real config files say, and what are the defaults? | With the config |

For what is built today, run `gero --help`, read the code and read the tests. Docs do not keep a build log.

## Rules that keep the docs readable

Docs that record progress get rewritten on every commit and end up describing nobody's current understanding.

1. **Write in the present tense, as if the design were finished.** A spec says what `gero` does, not what it did last week. Do not narrate history ("this replaces the earlier plan", "now also"). Git has the history.
2. **No build logs.** Do not list what is built in a status line or in prose. A spec carries a status (below), with a short "except" clause when part of it is not built, and a **Not built yet** list at the end. An item leaves that list when it is built. Nothing else in the file changes.
3. **One home per fact.** Link to it, do not copy it. A rule that is enforced by lint belongs in `.golangci.yml` comments, and the spec states the principle and links the config. The design of a package belongs in its `doc.go`, and the spec links the package.
4. **Cite by name, not by number.** Link to a spec or a heading by its title (`[GITHUB_ACCESS](specs/GITHUB_ACCESS.md)`). Section numbers drift when someone inserts a heading, which is why the core docs have none. A few older specs keep numbered headings; do not add numbers to new ones.
5. **Keep a spec in step with the code.** Update it in the same change that builds the part, and say in **Not built yet** what is left. While the design is still moving, keep the thinking in the issue, the PR or a scratch note, and write the spec when the shape settles.
6. **Rewrite instead of patching.** When a section has been edited three times, replace it with a fresh account of the current design. If it no longer fits its file, move it.
7. **Remove what the code now says.** A spec for something `implemented` keeps only the rationale the code cannot carry: why this and not the alternative, and the constraints that are not obvious from reading it.

## Spec status

`draft` is a proposal, `agreed` is decided and not built, and `implemented` means the code exists and matches. Inside a document, **(leaning)** marks a proposal not yet agreed and **(unverified)** marks something that needs a small spike before we rely on it.

## Where a new fact goes

* A decision about what the tool does for users: [IMPLEMENTATION](IMPLEMENTATION.md).
* How a part of the CLI or a package works: a spec, or the package's `doc.go` if it is small enough.
* Something we learned about a specific repo: `projects/`.
* A tool or project we took ideas from: `references/`.
* A question nobody has answered yet: the **Open questions** section of the document it affects, and nowhere else.
