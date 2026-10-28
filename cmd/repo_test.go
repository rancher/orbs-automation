package cmd

import (
	"bytes"
	"strings"
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

// Wiring only: the logic is tested in internal/cli/branchtags.
func TestRepoBranchTagsWiring(t *testing.T) {
	t.Chdir(t.TempDir()) // not inside any repository, so no tags
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	t.Cleanup(func() { rootCmd.SetOut(nil); rootCmd.SetErr(nil); rootCmd.SetArgs(nil) })

	t.Run("CI inputs", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "refs/heads/release/v2.x")
		t.Setenv("GITHUB_SHA", "abcdef1234567890")
		out.Reset()
		rootCmd.SetArgs([]string{"repo", "branch-tags"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatal(err)
		}
		want := "branch_tag=v2.x-head\nbranch_static_tag=v2.x-head-abcdef1\nprev_static_tag=\n"
		if out.String() != want {
			t.Errorf("stdout = %q, want %q", out.String(), want)
		}
	})

	t.Run("unsupported ref: message on stdout only, error returned", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "refs/pull/12/merge")
		t.Setenv("GITHUB_SHA", "abcdef1")
		out.Reset()
		errOut.Reset()
		rootCmd.SetArgs([]string{"repo", "branch-tags"})
		if err := rootCmd.Execute(); err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(out.String(), "unsupported branch pattern") {
			t.Errorf("stdout = %q, want the message", out.String())
		}
		if errOut.Len() != 0 {
			t.Errorf("stderr = %q, want empty", errOut.String())
		}
	})

	t.Run("not a repository", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "")
		t.Setenv("GITHUB_SHA", "")
		out.Reset()
		rootCmd.SetArgs([]string{"repo", "branch-tags"})
		if err := rootCmd.Execute(); err == nil {
			t.Fatal("expected an error")
		}
	})
}
