package cmd

import (
	"bytes"
	"testing"
)

// Wiring only: the logic is tested in internal/cli/checksemver.
func TestRepoCheckSemverWiring(t *testing.T) {
	t.Setenv("GITHUB_REF_NAME", "")
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	t.Cleanup(func() { rootCmd.SetOut(nil); rootCmd.SetArgs(nil) })

	rootCmd.SetArgs([]string{"repo", "check-semver", "v1.2.3-rc.1", "ignored"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if want := "HAS_PRERELEASE=true\nHAS_BUILD_META=false\n"; out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}

	rootCmd.SetArgs([]string{"repo", "check-semver"})
	out.Reset()
	if err := rootCmd.Execute(); err == nil {
		t.Error("expected an error with no tag")
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}
