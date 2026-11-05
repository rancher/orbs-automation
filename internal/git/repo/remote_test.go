package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ref(name, hash string) *plumbing.Reference {
	return plumbing.NewHashReference(plumbing.ReferenceName(name), plumbing.NewHash(hash))
}

const (
	hashA = "1111111111111111111111111111111111111111"
	hashB = "2222222222222222222222222222222222222222"
	hashC = "3333333333333333333333333333333333333333"
	hashD = "4444444444444444444444444444444444444444"
)

// fakeList answers each call from the next entry and records the auth it got.
type fakeList struct {
	results []fakeResult
	auths   []transport.AuthMethod
}

type fakeResult struct {
	refs []*plumbing.Reference
	err  error
}

func (f *fakeList) list(_ context.Context, _ string, auth transport.AuthMethod) ([]*plumbing.Reference, error) {
	f.auths = append(f.auths, auth)
	r := f.results[len(f.auths)-1]
	return r.refs, r.err
}

func TestListTagsResolvesAnnotatedTagsToTheirCommit(t *testing.T) {
	f := &fakeList{results: []fakeResult{{refs: []*plumbing.Reference{
		ref("refs/heads/main", hashD),
		ref("refs/tags/v1.0.0", hashA),    // lightweight: the commit itself
		ref("refs/tags/v1.1.0", hashB),    // annotated: the tag object
		ref("refs/tags/v1.1.0^{}", hashC), // and the commit it peels to
	}}}}

	got, err := listTags(context.Background(), f.list, "https://example/o/r.git", "")

	require.NoError(t, err)
	assert.Equal(t, []git.Tag{{Name: "v1.0.0", Commit: hashA}, {Name: "v1.1.0", Commit: hashC}}, got)
}

func TestListTagsIsSortedByName(t *testing.T) {
	f := &fakeList{results: []fakeResult{{refs: []*plumbing.Reference{
		ref("refs/tags/v2", hashA), ref("refs/tags/v1", hashB), ref("refs/tags/v10", hashC),
	}}}}
	got, err := listTags(context.Background(), f.list, "u", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"v1", "v10", "v2"}, []string{got[0].Name, got[1].Name, got[2].Name})
}

func TestListTagsAsksAnonymouslyFirst(t *testing.T) {
	f := &fakeList{results: []fakeResult{{}}}
	_, err := listTags(context.Background(), f.list, "u", "secret")
	require.NoError(t, err)
	require.Len(t, f.auths, 1, "a public repository costs one anonymous request")
	assert.Nil(t, f.auths[0], "the token is not sent unless it is asked for")
}

func TestListTagsRetriesWithTheTokenWhenAuthenticationIsRequired(t *testing.T) {
	f := &fakeList{results: []fakeResult{
		{err: transport.ErrAuthenticationRequired},
		{refs: []*plumbing.Reference{ref("refs/tags/v1", hashA)}},
	}}

	got, err := listTags(context.Background(), f.list, "u", "secret")

	require.NoError(t, err)
	assert.Equal(t, []git.Tag{{Name: "v1", Commit: hashA}}, got)
	require.Len(t, f.auths, 2)
	basic, ok := f.auths[1].(*http.BasicAuth)
	require.True(t, ok, "the retry authenticates")
	assert.Equal(t, "secret", basic.Password)
}

func TestListTagsWithoutATokenSaysCredentialsAreNeeded(t *testing.T) {
	f := &fakeList{results: []fakeResult{{err: transport.ErrAuthenticationRequired}}}
	_, err := listTags(context.Background(), f.list, "u", "")
	assert.ErrorIs(t, err, transport.ErrAuthenticationRequired)
	assert.ErrorContains(t, err, "token")
	assert.Len(t, f.auths, 1)
}

func TestListTagsDoesNotRetryOtherErrors(t *testing.T) {
	boom := errors.New("network down")
	f := &fakeList{results: []fakeResult{{err: boom}}}
	_, err := listTags(context.Background(), f.list, "u", "secret")
	assert.ErrorIs(t, err, boom)
	assert.Len(t, f.auths, 1)
}
