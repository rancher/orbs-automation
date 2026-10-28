package git

import (
	"errors"
	"testing"
)

type fakeTags struct {
	names []string
	err   error
}

func (f fakeTags) NewestFirst() ([]string, error) { return f.names, f.err }

func TestResolveBranchTags(t *testing.T) {
	const sha = "abcdef1"
	tests := []struct {
		name   string
		ref    string
		commit string
		tags   []string
		want   BranchTags
	}{
		{"main, no tags", "refs/heads/main", sha, nil,
			BranchTags{"head", "main-abcdef1", ""}},
		{"main, newest matching tag wins", "refs/heads/main", sha,
			[]string{"main-1111111", "v1.0.0", "main-0000000"},
			BranchTags{"head", "main-abcdef1", "main-1111111"}},
		{"main, non-matching tags skipped", "refs/heads/main", sha,
			[]string{"v1.0.0", "main-0000000"},
			BranchTags{"head", "main-abcdef1", "main-0000000"}},
		{"release", "refs/heads/release/v2.x", sha,
			[]string{"v2.x-head-2222222", "main-1111111"},
			BranchTags{"v2.x-head", "v2.x-head-abcdef1", "v2.x-head-2222222"}},
		{"release prefix is a regex, dot is a wildcard", "refs/heads/release/v2.x", sha,
			[]string{"v2Ax-head-2222222"},
			BranchTags{"v2.x-head", "v2.x-head-abcdef1", "v2Ax-head-2222222"}},
		{"previous may be the current commit's own tag", "refs/heads/release/v2.x", sha,
			[]string{"v2.x-head-abcdef1"},
			BranchTags{"v2.x-head", "v2.x-head-abcdef1", "v2.x-head-abcdef1"}},
		{"empty release version is accepted", "refs/heads/release/", sha, nil,
			BranchTags{"-head", "-head-abcdef1", ""}},
		{"nested release version", "refs/heads/release/a/b", sha, nil,
			BranchTags{"a/b-head", "a/b-head-abcdef1", ""}},
		{"short commit used as given", "refs/heads/main", "abc", nil,
			BranchTags{"head", "main-abc", ""}},
		{"longer local commit used as given", "refs/heads/main", "abcdef12", nil,
			BranchTags{"head", "main-abcdef12", ""}},
		{"invalid regex prefix matches nothing", "refs/heads/release/v[", sha,
			[]string{"v[-head-1"},
			BranchTags{"v[-head", "v[-head-abcdef1", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveBranchTags(tt.ref, tt.commit, fakeTags{names: tt.tags})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolveBranchTags = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResolveBranchTagsUnsupportedRef(t *testing.T) {
	for _, ref := range []string{
		"refs/heads/feature/x",
		"refs/heads/mainline",
		"refs/heads/release",
		"refs/tags/v1.0.0",
		"refs/pull/12/merge",
		"main",
		"",
	} {
		t.Run(ref, func(t *testing.T) {
			got, err := ResolveBranchTags(ref, "abcdef1", fakeTags{names: []string{"main-1111111"}})
			if !errors.Is(err, ErrUnsupportedRef) {
				t.Fatalf("err = %v, want ErrUnsupportedRef", err)
			}
			if got != (BranchTags{}) {
				t.Errorf("BranchTags = %+v, want zero", got)
			}
		})
	}
}

func TestResolveBranchTagsTagsError(t *testing.T) {
	boom := errors.New("boom")
	_, err := ResolveBranchTags("refs/heads/main", "abcdef1", fakeTags{err: boom})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap the Tags error", err)
	}
}

func TestShort(t *testing.T) {
	for in, want := range map[string]string{
		"abcdef1234567": "abcdef1",
		"abcdef1":       "abcdef1",
		"abc":           "abc",
		"":              "",
	} {
		if got := Short(in); got != want {
			t.Errorf("Short(%q) = %q, want %q", in, got, want)
		}
	}
}
