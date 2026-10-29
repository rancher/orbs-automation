package doctor

import (
	"context"
	"fmt"
)

// versionCheck reports which gero is running and on what. It always passes: it
// exists so the output of gero doctor says what it was run with.
type versionCheck struct{ build Build }

// NewVersionCheck returns the check named "version".
func NewVersionCheck(b Build) Check { return versionCheck{build: b} }

func (versionCheck) Name() string     { return "version" }
func (versionCheck) Category() string { return "environment" }
func (versionCheck) Description() string {
	return "Reports the gero version and the platform it runs on"
}
func (versionCheck) Online() bool    { return false }
func (versionCheck) Needs() []string { return nil }

func (c versionCheck) Run(context.Context) Result {
	return Result{Status: StatusOK, Message: fmt.Sprintf("gero %s on %s/%s", c.build.Version, c.build.OS, c.build.Arch)}
}
