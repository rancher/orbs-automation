# Contributing

How we work on this repo, for anyone, a person or an assistant. The rules are short because the tools enforce most of them.

## Code and tests

* **Write the failing test first, honestly.** Stub the API so everything compiles, make the test fail on its assertion, then fix it. The cycle is in [red-green-commits](red-green-commits.md), and [TESTING](../specs/TESTING.md) says how the tests are built.
* **Tests read nothing from the machine they run on:** no `GERO_*`, token, `HOME`, `XDG_*` or `CI`, and never the network.
* **Check mutants with `gremlins`, never by hand.** It mutates a temporary copy, is optional and installed globally by whoever wants it, and is not in `go.mod` ([TESTING](../specs/TESTING.md)). Do not edit code to see whether a test notices.
* **Respect the doors.** Each outside module has one package that may import it, and layers import downward only. `golangci-lint` fails on a breach ([PROJECT_LAYOUT](../specs/PROJECT_LAYOUT.md#6-enforcement), [DEPENDENCY_POLICY](../specs/DEPENDENCY_POLICY.md)). A new module needs an entry in the policy first.

## Commits

* **Small, one step each.** A stub, a failing test and its fix are three commits, and a docs change that goes with code is its own commit.
* **Stage the paths you mean.** `git add <path>`, never `git add -A` or `git add .`, because someone's work in progress may be in the tree.
* **Say what changed and why,** with the package or area first and a plain sentence after: `bump: goBump runs go mod tidy in each go.mod its holdings changed, once`. Look at `git log` for the shape.

## Docs

* Follow the rules in the [docs README](../README.md): the present tense, no build logs, one home for each fact, and nothing about the state of the world that goes stale, such as what has been released or how large something measured. Say what the tool does and what is not built.
* When the code and a spec disagree, the code is right and the spec is either stale or describes a feature that is not built. Say which, in the same change. [`design/`](../design/) comes before a spec that disagrees with it.
* A docs change that corrects a statement checks it against the code first.

## Assistants

An assistant follows the same rules: it writes the test first, stages explicit paths, commits in small steps and checks a claim in a doc against the code before it writes it. It uses the IDE's search, refactoring and formatting where it has them in preference to a script that edits many files.
