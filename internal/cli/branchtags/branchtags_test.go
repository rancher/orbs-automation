package branchtags

import (
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/git/repo"
)

type fakeRepo struct {
	ref, short string
	headErr    error
	tags       []string
	tagsErr    error
}

func (f fakeRepo) Head() (string, string, error)  { return f.ref, f.short, f.headErr }
func (f fakeRepo) NewestFirst() ([]string, error) { return f.tags, f.tagsErr }

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

// openers returns an opener that serves repo (or err), recording its calls.
func serve(repo Repository, err error, calls *int) opener {
	return func(string) (Repository, error) {
		*calls++
		return repo, err
	}
}

func TestResolveFromCI(t *testing.T) {
	repo := fakeRepo{ref: "refs/heads/ignored", short: "zzzzzzz", tags: []string{"main-1111111", "v1.0.0"}}
	vars := map[string]string{"GITHUB_REF": "refs/heads/main", "GITHUB_SHA": "abcdef1234567890"}

	var strict, discover int
	got, err := resolve(env(vars), ".", serve(repo, nil, &strict), serve(repo, nil, &discover))
	if err != nil {
		t.Fatal(err)
	}
	want := git.BranchTags{BranchTag: "head", BranchStaticTag: "main-abcdef1", PrevStaticTag: "main-1111111"}
	if got != want {
		t.Errorf("Resolve = %+v, want %+v", got, want)
	}
	if strict != 0 {
		t.Error("CI inputs must not require a .git directory in the working directory")
	}
}

func TestResolveFromCIWithoutRepository(t *testing.T) {
	vars := map[string]string{"GITHUB_REF": "refs/heads/release/v2.x", "GITHUB_SHA": "abcdef1234567"}
	var n int
	got, err := resolve(env(vars), ".", nil, serve(nil, repo.ErrNotRepository, &n))
	if err != nil {
		t.Fatalf("a missing repository is not an error in CI mode: %v", err)
	}
	want := git.BranchTags{BranchTag: "v2.x-head", BranchStaticTag: "v2.x-head-abcdef1"}
	if got != want {
		t.Errorf("Resolve = %+v, want %+v", got, want)
	}
}

func TestResolveFromCIOtherOpenErrorIsReturned(t *testing.T) {
	boom := errors.New("boom")
	vars := map[string]string{"GITHUB_REF": "refs/heads/main", "GITHUB_SHA": "abcdef1"}
	var n int
	if _, err := resolve(env(vars), ".", nil, serve(nil, boom, &n)); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}

func TestResolveFromRepository(t *testing.T) {
	repo := fakeRepo{ref: "refs/heads/release/v2.x", short: "abcdef12", tags: []string{"v2.x-head-1111111"}}
	for name, vars := range map[string]map[string]string{
		"no CI variables": nil,
		"only the ref":    {"GITHUB_REF": "refs/heads/main"},
		"only the sha":    {"GITHUB_SHA": "ffffffffffff"},
		"empty values":    {"GITHUB_REF": "", "GITHUB_SHA": ""},
	} {
		t.Run(name, func(t *testing.T) {
			var n int
			got, err := resolve(env(vars), ".", serve(repo, nil, &n), nil)
			if err != nil {
				t.Fatal(err)
			}
			// The local abbreviation is used as given, even past 7 characters.
			want := git.BranchTags{BranchTag: "v2.x-head", BranchStaticTag: "v2.x-head-abcdef12", PrevStaticTag: "v2.x-head-1111111"}
			if got != want {
				t.Errorf("Resolve = %+v, want %+v", got, want)
			}
		})
	}
}

func TestResolveErrors(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name string
		vars map[string]string
		open opener
		want error
	}{
		{"not a repository", nil, func(string) (Repository, error) { return nil, repo.ErrNotRepository }, repo.ErrNotRepository},
		{"detached HEAD", nil, func(string) (Repository, error) { return fakeRepo{headErr: repo.ErrDetachedHead}, nil }, repo.ErrDetachedHead},
		{"unsupported local branch", nil, func(string) (Repository, error) { return fakeRepo{ref: "refs/heads/feature/x", short: "abcdef1"}, nil }, git.ErrUnsupportedRef},
		{"unsupported CI ref", map[string]string{"GITHUB_REF": "refs/pull/12/merge", "GITHUB_SHA": "abcdef1"}, nil, git.ErrUnsupportedRef},
		{"tag listing fails", nil, func(string) (Repository, error) {
			return fakeRepo{ref: "refs/heads/main", short: "abcdef1", tagsErr: boom}, nil
		}, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discover := func(string) (Repository, error) { return nil, repo.ErrNotRepository }
			if _, err := resolve(env(tt.vars), ".", tt.open, discover); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		res  git.BranchTags
		want string
	}{
		{"all set", git.BranchTags{BranchTag: "head", BranchStaticTag: "main-abcdef1", PrevStaticTag: "main-1111111"},
			"branch_tag=head\nbranch_static_tag=main-abcdef1\nprev_static_tag=main-1111111\n"},
		{"empty previous tag keeps its line", git.BranchTags{BranchTag: "head", BranchStaticTag: "main-abcdef1"},
			"branch_tag=head\nbranch_static_tag=main-abcdef1\nprev_static_tag=\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Render(tt.res); got != tt.want {
				t.Errorf("Render = %q, want %q", got, tt.want)
			}
		})
	}
}
