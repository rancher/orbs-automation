// Package cigate decides whether a CI-only command may run: it needs the CI
// context and a token (docs/specs/CLI_FRAMEWORK.md, EXECUTION_CONTEXT.md). The
// cobra layer reads the command's annotation and calls Require.
package cigate

import (
	"fmt"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/github"
)

// Auth is where the gate looks for a token. Tests replace it.
var Auth github.Auth = github.GoGh{}

// Require reports why the command at commandPath may not run as a CI-only
// command, or nil when it may. The CI context is any of GITHUB_ACTIONS=true,
// CI=true or GERO_CONTEXT=ci, so a developer can run a CI job locally to debug
// it, with their own token.
func Require(commandPath string, s clicfg.Settings) error {
	if !s.CI {
		return fmt.Errorf("%s needs the CI context: run it in GitHub Actions, or set GERO_CONTEXT=ci to run it locally", commandPath)
	}
	if _, err := github.Resolve(Auth, ""); err != nil {
		return fmt.Errorf("%s: %w", commandPath, err)
	}
	return nil
}

// RequireWorkstation reports why the command at commandPath may not run as a
// workstation-only command, or nil when it may. It does not run in CI, and not
// inside gero repl, which says so with inRepl.
func RequireWorkstation(commandPath string, s clicfg.Settings, inRepl bool) error {
	return nil
}

// RequireCrossRepoToken refuses the workflow's default GITHUB_TOKEN before a
// command at commandPath writes to a repository other than the one the workflow
// runs in: that token is scoped to its own repository, so GitHub would answer
// 403 after the work was done. It says to pass a cross-repo token as GH_TOKEN,
// which wins over GITHUB_TOKEN. Call it for every CI-only command that writes
// elsewhere.
func RequireCrossRepoToken(commandPath string, s clicfg.Settings) error {
	if s.DefaultToken {
		return fmt.Errorf("%s writes to another repository, but GH_TOKEN is unset or equal to GITHUB_TOKEN, which only reaches the repository the workflow runs in: set GH_TOKEN to a token for the target repository", commandPath)
	}
	return nil
}
