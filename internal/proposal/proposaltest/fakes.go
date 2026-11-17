package proposaltest

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
)

// Log is the calls made to the fakes, in order, as one line each.
type Log struct{ Calls []string }

func (l *Log) Add(format string, args ...any) {
	l.Calls = append(l.Calls, fmt.Sprintf(format, args...))
}

// Index is the position of the first call that starts with prefix, or -1.
func (l *Log) Index(prefix string) int {
	for i, c := range l.Calls {
		if strings.HasPrefix(c, prefix) {
			return i
		}
	}
	return -1
}

func (l *Log) Has(prefix string) bool {
	for _, c := range l.Calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

// Count is how many calls start with prefix.
func (l *Log) Count(prefix string) int {
	n := 0
	for _, c := range l.Calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

// FakeRemote is the GitHub of a test: what it finds is set in its fields, and every call it
// gets is logged.
type FakeRemote struct {
	Log       *Log
	Labels    []string
	LabelsErr error
	AddErr    error
	CreateErr error
	CommentOn map[int]error
	CloseOn   map[int]error
	Open      pulls.Open            // what OpenForBase finds
	OpenBy    map[string]pulls.Open // what it finds for a base, when that is set
	OpenErr   error
	ByHead    map[string][]pulls.PR // what OpenForHead finds for a head
	HeadErr   error
	MeErr     error

	Behind      int
	BehindErr   error
	UpdateErr   error
	PRComments  []pulls.Comment
	CommentsErr error
	EditErr     error
	// UpdatedTitle and UpdatedBody are what the last UpdateBody was given.
	UpdatedTitle, UpdatedBody string
}

func (f *FakeRemote) BehindBy(_ context.Context, _, base, head string) (int, error) {
	f.Log.Add("behind %s %s", base, head)
	return f.Behind, f.BehindErr
}

func (f *FakeRemote) UpdateBody(_ context.Context, _ string, n int, title, body string) error {
	f.Log.Add("update #%d title=%q body=%q", n, title, body)
	f.UpdatedTitle, f.UpdatedBody = title, body
	return f.UpdateErr
}

func (f *FakeRemote) Comments(_ context.Context, _ string, n, _ int) ([]pulls.Comment, error) {
	f.Log.Add("comments #%d", n)
	return f.PRComments, f.CommentsErr
}

func (f *FakeRemote) EditComment(_ context.Context, _ string, id int64, body string) error {
	f.Log.Add("edit comment %d %s", id, body)
	return f.EditErr
}

func (f *FakeRemote) Me(context.Context) (string, error) {
	f.Log.Add("me")
	return "danpock", f.MeErr
}

func (f *FakeRemote) OpenForBase(_ context.Context, slug, base string, _ int) (pulls.Open, error) {
	f.Log.Add("open %s %s", slug, base)
	if found, ok := f.OpenBy[base]; ok {
		return found, f.OpenErr
	}
	return f.Open, f.OpenErr
}

func (f *FakeRemote) OpenForHead(_ context.Context, slug, head string) ([]pulls.PR, error) {
	f.Log.Add("open head %s %s", slug, head)
	return f.ByHead[head], f.HeadErr
}

func (f *FakeRemote) Create(_ context.Context, slug string, in pulls.NewPR) (pulls.PR, error) {
	f.Log.Add("create %s head=%s base=%s title=%q", slug, in.Head, in.Base, in.Title)
	if f.CreateErr != nil {
		return pulls.PR{}, f.CreateErr
	}
	return pulls.PR{Number: 104, URL: "https://github.com/" + slug + "/pull/104"}, nil
}

func (f *FakeRemote) Comment(_ context.Context, _ string, n int, body string) error {
	f.Log.Add("comment #%d %s", n, body)
	return f.CommentOn[n]
}

func (f *FakeRemote) Close(_ context.Context, _ string, n int) error {
	f.Log.Add("close #%d", n)
	return f.CloseOn[n]
}

func (f *FakeRemote) RepoLabels(context.Context, string) ([]string, error) {
	f.Log.Add("labels")
	return f.Labels, f.LabelsErr
}

func (f *FakeRemote) AddLabels(_ context.Context, _ string, n int, labels []string) error {
	f.Log.Add("add labels #%d %s", n, strings.Join(labels, ","))
	return f.AddErr
}
