package gitcli

import (
	"context"
	"strings"
)

// ConfigBool is what git itself would use for the boolean key in the clone in
// dir (system, global and local config, includes and all), false for a key that
// is not set. A value that is not a boolean is an error, as it is for git.
func ConfigBool(ctx context.Context, dir, key string) (bool, error) {
	out, err := inDir(ctx, dir, "config", "--type=bool", "--get", key)
	if exitCode(err) == 1 {
		return false, nil // git config --get exits 1 for a key that is not set
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

// SignsByDefault says whether git, as configured in the clone in dir, signs the
// commits and the tags that gero makes when it is not told either way: commits
// when commit.gpgSign is set, and tags, which gero always annotates, when
// tag.gpgSign or tag.forceSignAnnotated is.
func SignsByDefault(ctx context.Context, dir string) (commits, tags bool, err error) {
	if commits, err = ConfigBool(ctx, dir, "commit.gpgSign"); err != nil {
		return false, false, err
	}
	for _, key := range []string{"tag.gpgSign", "tag.forceSignAnnotated"} {
		on, err := ConfigBool(ctx, dir, key)
		if err != nil {
			return false, false, err
		}
		tags = tags || on
	}
	return commits, tags, nil
}
