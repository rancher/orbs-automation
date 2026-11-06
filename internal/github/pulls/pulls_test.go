package pulls

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/rancher/orbs-automation/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// reply is what the stub answers one request with.
type reply struct {
	status int
	body   string
	link   string // a Link header, for the next page
}

// stub answers requests from a function and records them, with their bodies.
type stub struct {
	handle func(r *http.Request) reply
	reqs   []*http.Request
	bodies []string
}

func (s *stub) option() gogithub.ClientOptionsFunc {
	return gogithub.WithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body string
		if r.Body != nil {
			data, _ := io.ReadAll(r.Body)
			body = string(data)
		}
		s.reqs, s.bodies = append(s.reqs, r), append(s.bodies, body)
		rep := s.handle(r)
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		if rep.link != "" {
			rec.Header().Set("Link", rep.link)
		}
		rec.WriteHeader(rep.status)
		_, _ = rec.WriteString(rep.body)
		return rec.Result(), nil
	}))
}

// req is the first request, which the test requires there to have been.
func (s *stub) req(t *testing.T) *http.Request {
	t.Helper()
	require.NotEmpty(t, s.reqs, "a request was made")
	return s.reqs[0]
}

// body is the body of the first request.
func (s *stub) body(t *testing.T) string {
	t.Helper()
	require.NotEmpty(t, s.bodies, "a request was made")
	return s.bodies[0]
}

func always(status int, body string) *stub {
	return &stub{handle: func(*http.Request) reply { return reply{status: status, body: body} }}
}

func service(t *testing.T, s *stub) *Service {
	t.Helper()
	svc, err := New(github.Credentials{Host: "github.com", Token: "tok"}, s.option())
	require.NoError(t, err)
	return svc
}

const prJSON = `{"number":7,"html_url":"https://github.com/rancher/rancher/pull/7","title":"[v2.15] Update scc-operator",` +
	`"body":"body <!-- gero -->","draft":false,"user":{"login":"dan"},"base":{"ref":"release/v2.15"},` +
	`"head":{"ref":"bot/scc-operator-v0.5.3-v2.15","user":{"login":"danpock"}},"labels":[{"name":"status/auto-created"},{"name":"team/orbs"}]}`

func TestOpenForBaseListsTheOpenPRsOfABaseNewestFirst(t *testing.T) {
	s := always(200, "["+prJSON+"]")

	got, err := service(t, s).OpenForBase(bg, "rancher/rancher", "release/v2.15", 0)

	require.NoError(t, err)
	require.Len(t, s.reqs, 1, "one request: a page is enough unless asked for more")
	q := s.req(t).URL.Query()
	assert.Equal(t, "/repos/rancher/rancher/pulls", s.req(t).URL.Path)
	assert.Equal(t, "open", q.Get("state"))
	assert.Equal(t, "release/v2.15", q.Get("base"))
	assert.Equal(t, "created", q.Get("sort"))
	assert.Equal(t, "desc", q.Get("direction"))
	assert.Equal(t, "100", q.Get("per_page"))
	assert.False(t, got.Truncated)
	require.Len(t, got.PRs, 1)
	assert.Equal(t, PR{
		Number: 7, URL: "https://github.com/rancher/rancher/pull/7", Title: "[v2.15] Update scc-operator",
		Body: "body <!-- gero -->", Author: "dan", Base: "release/v2.15", HeadRef: "bot/scc-operator-v0.5.3-v2.15",
		HeadOwner: "danpock", Labels: []string{"status/auto-created", "team/orbs"},
	}, got.PRs[0])
}

func page(from, to int) string {
	var items []string
	for n := from; n <= to; n++ {
		items = append(items, strings.Replace(prJSON, `"number":7`, `"number":`+strconv.Itoa(n), 1))
	}
	return "[" + strings.Join(items, ",") + "]"
}

func TestOpenForBaseStopsAtMaxAndSaysThereWasMore(t *testing.T) {
	s := &stub{handle: func(*http.Request) reply {
		return reply{status: 200, body: page(1, 3), link: `<https://api.github.com/repos/rancher/rancher/pulls?page=2>; rel="next"`}
	}}

	got, err := service(t, s).OpenForBase(bg, "rancher/rancher", "main", 3)

	require.NoError(t, err)
	assert.Len(t, got.PRs, 3)
	assert.True(t, got.Truncated, "another page exists that was not read")
	assert.Len(t, s.reqs, 1, "and it was not requested")
}

func TestOpenForBaseFollowsPagesUpToMax(t *testing.T) {
	s := &stub{handle: func(r *http.Request) reply {
		if r.URL.Query().Get("page") == "2" {
			return reply{status: 200, body: page(4, 5)}
		}
		return reply{status: 200, body: page(1, 3), link: `<https://api.github.com/repos/rancher/rancher/pulls?page=2>; rel="next"`}
	}}

	got, err := service(t, s).OpenForBase(bg, "rancher/rancher", "main", 10)

	require.NoError(t, err)
	assert.Len(t, got.PRs, 5)
	assert.False(t, got.Truncated)
	assert.Len(t, s.reqs, 2)
}

func TestOpenForBaseTrimsToMaxWhenAPageHasMore(t *testing.T) {
	s := always(200, page(1, 5))

	got, err := service(t, s).OpenForBase(bg, "rancher/rancher", "main", 2)

	require.NoError(t, err)
	require.Len(t, got.PRs, 2)
	assert.Equal(t, 1, got.PRs[0].Number, "the newest are kept")
	assert.True(t, got.Truncated)
}

func TestOpenForBaseErrorNamesTheRepositoryAndBase(t *testing.T) {
	_, err := service(t, always(404, `{"message":"Not Found"}`)).OpenForBase(bg, "rancher/nope", "main", 0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rancher/nope")
	assert.Contains(t, err.Error(), "main")
}

func TestOpenForHeadAsksForThatHeadOnly(t *testing.T) {
	s := always(200, "["+prJSON+"]")

	got, err := service(t, s).OpenForHead(bg, "rancher/rancher", "danpock:bot/scc-operator-v0.5.3-v2.15")

	require.NoError(t, err)
	require.Len(t, s.reqs, 1)
	q := s.req(t).URL.Query()
	assert.Equal(t, "danpock:bot/scc-operator-v0.5.3-v2.15", q.Get("head"))
	assert.Equal(t, "open", q.Get("state"))
	require.Len(t, got, 1)
	assert.Equal(t, 7, got[0].Number)
}

func TestCreateOpensThePRFromAHeadToABase(t *testing.T) {
	s := always(201, prJSON)

	got, err := service(t, s).Create(bg, "rancher/rancher", NewPR{
		Title: "[v2.15] Update scc-operator", Body: "the body", Head: "danpock:bot/x", Base: "release/v2.15",
	})

	require.NoError(t, err)
	require.Len(t, s.reqs, 1)
	assert.Equal(t, http.MethodPost, s.req(t).Method)
	assert.Equal(t, "/repos/rancher/rancher/pulls", s.req(t).URL.Path)
	for _, want := range []string{`"title":"[v2.15] Update scc-operator"`, `"body":"the body"`, `"head":"danpock:bot/x"`, `"base":"release/v2.15"`} {
		assert.Contains(t, s.body(t), want)
	}
	assert.Equal(t, 7, got.Number)
	assert.Equal(t, "https://github.com/rancher/rancher/pull/7", got.URL)
}

func TestCreateADraft(t *testing.T) {
	s := always(201, prJSON)

	_, err := service(t, s).Create(bg, "rancher/rancher", NewPR{Title: "t", Head: "h", Base: "b", Draft: true})

	require.NoError(t, err)
	assert.Contains(t, s.body(t), `"draft":true`)
}

func TestCreateRefusedSaysWhy(t *testing.T) {
	s := always(422, `{"message":"Validation Failed","errors":[{"message":"A pull request already exists for danpock:bot/x."}]}`)

	_, err := service(t, s).Create(bg, "rancher/rancher", NewPR{Title: "t", Head: "danpock:bot/x", Base: "main"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rancher/rancher")
	assert.Contains(t, err.Error(), "already exists")
}

func TestCommentPostsToTheItem(t *testing.T) {
	s := always(201, `{"id":1}`)

	err := service(t, s).Comment(bg, "rancher/rancher", 7, "Superseded by #9")

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, s.req(t).Method)
	assert.Equal(t, "/repos/rancher/rancher/issues/7/comments", s.req(t).URL.Path)
	assert.Contains(t, s.body(t), `"body":"Superseded by #9"`)
}

func TestCloseClosesWithoutMerging(t *testing.T) {
	s := always(200, `{"number":7,"state":"closed"}`)

	err := service(t, s).Close(bg, "rancher/rancher", 7)

	require.NoError(t, err)
	assert.Equal(t, http.MethodPatch, s.req(t).Method)
	assert.Equal(t, "/repos/rancher/rancher/issues/7", s.req(t).URL.Path, "the issues endpoint closes issues and PRs alike")
	assert.Contains(t, s.body(t), `"state":"closed"`)
}

func TestRepoLabelsListsEveryPage(t *testing.T) {
	s := &stub{handle: func(r *http.Request) reply {
		if r.URL.Query().Get("page") == "2" {
			return reply{status: 200, body: `[{"name":"team/orbs"}]`}
		}
		return reply{status: 200, body: `[{"name":"status/auto-created"},{"name":"kind/bug"}]`,
			link: `<https://api.github.com/repos/rancher/rancher/labels?page=2>; rel="next"`}
	}}

	got, err := service(t, s).RepoLabels(bg, "rancher/rancher")

	require.NoError(t, err)
	assert.Equal(t, []string{"status/auto-created", "kind/bug", "team/orbs"}, got)
	assert.Equal(t, "/repos/rancher/rancher/labels", s.req(t).URL.Path)
	assert.Equal(t, "100", s.req(t).URL.Query().Get("per_page"))
}

func TestAddLabelsAddsThemToTheItem(t *testing.T) {
	s := always(200, `[{"name":"status/auto-created"}]`)

	err := service(t, s).AddLabels(bg, "rancher/rancher", 7, []string{"status/auto-created", "team/orbs"})

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, s.req(t).Method)
	assert.Equal(t, "/repos/rancher/rancher/issues/7/labels", s.req(t).URL.Path)
	assert.Contains(t, s.body(t), `["status/auto-created","team/orbs"]`)
}

func TestAddNoLabelsMakesNoRequest(t *testing.T) {
	s := always(200, `[]`)

	require.NoError(t, service(t, s).AddLabels(bg, "rancher/rancher", 7, nil))

	assert.Empty(t, s.reqs)
}

func TestARepositoryThatIsNotOwnerSlashNameIsAnErrorAndMakesNoRequest(t *testing.T) {
	s := always(200, `[]`)
	svc := service(t, s)

	_, err := svc.OpenForBase(bg, "rancher", "main", 0)
	require.Error(t, err)
	_, err = svc.Create(bg, "a/b/c", NewPR{})
	require.Error(t, err)
	require.Error(t, svc.Comment(bg, "", 1, "x"))
	require.Error(t, svc.Close(bg, "x/", 1))
	_, err = svc.RepoLabels(bg, "/y")
	require.Error(t, err)

	assert.Empty(t, s.reqs)
}

func TestMeIsTheLoginTheTokenBelongsTo(t *testing.T) {
	s := always(200, `{"login":"danpock"}`)

	me, err := service(t, s).Me(bg)

	require.NoError(t, err)
	assert.Equal(t, "danpock", me)
	assert.Equal(t, "/user", s.req(t).URL.Path)
}

func TestMeAsTheBotIsTheBotWithoutAskingGitHub(t *testing.T) {
	s := always(200, `{"login":"someone-else"}`)

	me, err := service(t, s).AsBot("rancher-scc-bot[bot]").Me(bg)

	require.NoError(t, err)
	assert.Equal(t, "rancher-scc-bot[bot]", me, "an installation token has no user to ask about")
	assert.Empty(t, s.reqs)
}

func TestMeWithABadTokenIsAnError(t *testing.T) {
	_, err := service(t, always(401, `{"message":"Bad credentials"}`)).Me(bg)

	require.ErrorIs(t, err, github.ErrBadCredentials)
}
