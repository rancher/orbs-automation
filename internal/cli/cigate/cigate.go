// Package cigate decides whether a CI-only command may run: it needs the CI
// context and a token (docs/specs/CLI_FRAMEWORK.md, EXECUTION_CONTEXT.md). The
// cobra layer reads the command's annotation and calls Require.
package cigate

import (
	"fmt"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/github"
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
