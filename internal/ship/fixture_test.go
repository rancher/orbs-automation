package ship

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/ociregistry"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
)

// lineSpec is a line a project lists and the branch it is on; no branch is the
// default branch.
type lineSpec struct{ Line, Branch string }

// projectSpec is a project as the tests write it, before it is made into documents.
type projectSpec struct {
	Name, Repo, ManagedBy string
	Owned                 bool
	Title                 string     // pullRequests.title
	PushTo                string     // pullRequests.pushTo
	Labels, BotLabels     []string   // pullRequests.labels and botLabels
	Lines                 []lineSpec // the lines it lists, on a branch each; none is a single branch
	Stream                string     // a producer's stream, as the lines of its spec
	Scheme                string     // a producer's stream scheme; semver when empty
	Images                []string   // a producer's image artifacts
}

// stepSpec is a step of an edit.
type stepSpec struct{ Run, Since, Until, From string }

// link is a holding of the producer's image in a consumer, with the edit that
// follows it: what the tests write for a route.
type link struct {
	Consumer          projectSpec
	File, Key, Form   string // where it is and how it is written; the defaults are what the fixtures use
	Accept            string
	HoldsLines        string // holds.lines, as flow YAML
	Strategy          string // the strategy of an edit that follows the holding
	Pre, Post         []stepSpec
	PackageForTheEdit string
}

// fakeWorkspace is a workspace served from the specs: the documents are built
// from them, joined, and the real update engine is built from that, as the
// workspace does from the loaded config. The host's refs and files are fields.
type fakeWorkspace struct {
	t          *testing.T
	producer   projectSpec
	links      []link
	projectErr error
	refs       map[string]git.Refs // by repository slug
	refsErr    map[string]error
	files      map[string]string // "slug@ref:path"
	asked      []string
	cloneDir   string // the engineer's clone, for Ship
	cloneErr   error
	push       PushSettings

	graph    *chain.Graph
	graphKey string
}

var lineID = regexp.MustCompile(`v[0-9]+\.[0-9]+`)

// docs makes the config documents of the specs.
func (f *fakeWorkspace) docs() []string {
	var docs []string
	lineSet := map[string]bool{}
	for _, m := range lineID.FindAllString(f.producer.Stream, -1) {
		lineSet[m] = true
	}
	for _, l := range f.links {
		for _, ls := range l.Consumer.Lines {
			lineSet[ls.Line] = true
		}
	}
	var lines []string
	for l := range lineSet {
		lines = append(lines, l)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(lines)))
	if len(lines) > 0 {
		docs = append(docs, chaintest.Lines(lines...))
	}

	docs = append(docs, chaintest.Doc("Project", f.producer.Name, "", projectBody(f.producer)))
	stream := f.producer.Stream
	if stream == "" {
		stream = "  universal: true\n"
	}
	if f.producer.Scheme != "" {
		stream = "  scheme: " + f.producer.Scheme + "\n" + stream
	}
	docs = append(docs, chaintest.Doc("Stream", f.producer.Name, "{project: "+f.producer.Name+"}", stream))
	for i, image := range f.producer.Images {
		docs = append(docs, chaintest.Doc("Artifact", fmt.Sprintf("%s-image-%d", f.producer.Name, i), "{stream: "+f.producer.Name+"}", "  kind: image\n  name: "+image+"\n"))
	}

	seen := map[string]bool{}
	for i, l := range f.links {
		c := l.Consumer
		if !seen[c.Name] {
			seen[c.Name] = true
			docs = append(docs, chaintest.Doc("Project", c.Name, "", projectBody(c)))
		}
		name := fmt.Sprintf("%s-h%d", c.Name, i)
		file, key, form, accept := orDefault(l.File, "build.yaml"), orDefault(l.Key, "defaultSccOperatorImage"), orDefault(l.Form, "{image}:{tag}"), orDefault(l.Accept, "stable")
		holds := fmt.Sprintf("  holds: {kind: release, form: %q, accept: %s", form, accept)
		if l.HoldsLines != "" {
			holds += ", lines: " + l.HoldsLines
		}
		holds += "}\n"
		docs = append(docs, chaintest.Doc("Holding", name, "{project: "+c.Name+", artifact: "+f.producer.Name+"-image-0}",
			fmt.Sprintf("  at: {file: %s, key: %s}\n", file, key)+holds))
		if l.Strategy != "" || len(l.Pre)+len(l.Post) > 0 {
			spec := ""
			if l.Strategy != "" {
				spec += "  strategy: " + l.Strategy + "\n  with: {package: " + orDefault(l.PackageForTheEdit, "x") + "}\n"
			}
			spec += stepsYAML("preEdit", l.Pre) + stepsYAML("postEdit", l.Post)
			docs = append(docs, chaintest.Doc("Edit", name+"-edit", "{holding: "+name+"}", spec))
		}
	}
	return docs
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// projectBody is the spec of a project document.
func projectBody(p projectSpec) string {
	slug := orDefault(p.Repo, "acme/"+p.Name)
	repo := "{name: " + slug
	if p.Owned {
		repo += ", owned: true"
	}
	if p.ManagedBy != "" {
		repo += ", managedBy: " + p.ManagedBy
	}
	repo += "}"
	body := "  repo: " + repo + "\n"
	var pr []string
	if p.Title != "" {
		pr = append(pr, fmt.Sprintf("title: %q", p.Title))
	}
	if p.PushTo != "" {
		pr = append(pr, "pushTo: "+p.PushTo)
	}
	if len(p.Labels) > 0 {
		pr = append(pr, "labels: ["+strings.Join(p.Labels, ", ")+"]")
	}
	if len(p.BotLabels) > 0 {
		pr = append(pr, "botLabels: ["+strings.Join(p.BotLabels, ", ")+"]")
	}
	if len(pr) > 0 {
		body += "  pullRequests: {" + strings.Join(pr, ", ") + "}\n"
	}
	if len(p.Lines) > 0 {
		body += "  branches:\n    layout: perLine\n    lines:\n"
		var bodySb174 strings.Builder
		for _, l := range p.Lines {
			if l.Branch == "" {
				bodySb174.WriteString("      - {line: " + l.Line + "}\n")
			} else {
				bodySb174.WriteString("      - {line: " + l.Line + ", branch: " + l.Branch + "}\n")
			}
		}
		body += bodySb174.String()
	}
	return body
}

// stepsYAML writes the steps of a phase.
func stepsYAML(phase string, steps []stepSpec) string {
	if len(steps) == 0 {
		return ""
	}
	out := "  " + phase + ":\n"
	var outSb191 strings.Builder
	for _, s := range steps {
		outSb191.WriteString("    - {run: [" + strings.Join(strings.Fields(s.Run), ", ") + "]")
		if s.Since != "" {
			outSb191.WriteString(", since: " + s.Since)
		}
		if s.Until != "" {
			outSb191.WriteString(", until: " + s.Until)
		}
		if s.From != "" {
			outSb191.WriteString(", from: " + s.From)
		}
		outSb191.WriteString("}\n")
	}
	out += outSb191.String()
	return out
}

// config joins the documents, again when the specs have changed since, so the
// producer and the engine of one run are made from the same graph.
func (f *fakeWorkspace) config() *chain.Graph {
	docs := f.docs()
	if key := strings.Join(docs, "\x00"); f.graph == nil || key != f.graphKey {
		f.graph, f.graphKey = chaintest.Load(f.t, docs...), key
	}
	return f.graph
}

// Project returns the producer, whatever the key, as the loaded config has it.
func (f *fakeWorkspace) Project(key string) (*chain.Project, error) {
	f.asked = append(f.asked, "project "+key)
	if f.projectErr != nil {
		return nil, f.projectErr
	}
	return f.config().Project(f.producer.Name), nil
}

// Bump builds the real engine from the config, as the workspace does.
func (f *fakeWorkspace) Bump() (*bump.Engine, error) {
	return bump.NewEngine(bump.Builtin(), f.config())
}

// CloneDir and Remote answer from the fields, for the tests that make commits.
func (f *fakeWorkspace) CloneDir(string) (string, error) { return f.cloneDir, f.cloneErr }

func (f *fakeWorkspace) Remote(string) (string, error) { return "upstream", nil }

// PushSettings answers from the field.
func (f *fakeWorkspace) PushSettings(string) PushSettings { return f.push }

func (f *fakeWorkspace) UpstreamRefs(_ context.Context, slug string) (git.Refs, error) {
	f.asked = append(f.asked, "refs "+slug)
	return f.refs[slug], f.refsErr[slug]
}

func (f *fakeWorkspace) UpstreamFile(_ context.Context, slug, ref, path string) ([]byte, error) {
	f.asked = append(f.asked, "file "+slug+"@"+ref+":"+path)
	content, ok := f.files[slug+"@"+ref+":"+path]
	if !ok {
		return nil, errors.New("file not found")
	}
	return []byte(content), nil
}

type fakeProber struct {
	mu      sync.Mutex
	results map[string]ociregistry.Result
	calls   int
}

func (f *fakeProber) Exists(_ context.Context, host, image, tag string) ociregistry.Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if r, ok := f.results[host+" "+image+":"+tag]; ok {
		return r
	}
	return ociregistry.Result{Status: ociregistry.Found}
}

func tags(names ...string) []git.Tag {
	out := make([]git.Tag, len(names))
	for i, n := range names {
		out[i] = git.Tag{Name: n, Commit: "sha-" + n}
	}
	return out
}

// universal is a producer whose every release goes to every branch: scc-operator.
func universal() projectSpec {
	return projectSpec{Name: "scc-operator", Repo: "rancher/scc-operator", Owned: true, Images: []string{"rancher/scc-operator"}}
}

// lined is a producer with a series for each of two lines: kuberlr-kubectl.
func lined() projectSpec {
	return projectSpec{
		Name: "kuberlr-kubectl", Repo: "rancher/kuberlr-kubectl", Owned: true, Images: []string{"rancher/kuberlr-kubectl"},
		Stream: "  lines:\n    - {line: v2.16, series: \"9.x\"}\n    - {line: v2.15, series: \"8.x\"}\n",
	}
}

// riLink is the holding of scc-operator's image in rancher's build.yaml, as the
// config writes it: the consumer lists v2.15 and v2.14 on branches of their own.
func riLink() link {
	return link{
		Consumer: projectSpec{
			Name: "rancher", Repo: "rancher/rancher",
			Lines: []lineSpec{{"v2.15", "release/v2.15"}, {"v2.14", "release/v2.14"}},
		},
	}
}

func fixture(t *testing.T) *fakeWorkspace {
	t.Helper()
	return &fakeWorkspace{
		t:        t,
		producer: universal(),
		links:    []link{riLink()},
		refs: map[string]git.Refs{
			"rancher/scc-operator": {Tags: tags("v0.5.2", "v0.5.3", "v0.6.0-rc.1")},
			"rancher/rancher": {Branches: []git.Branch{
				{Name: "main", Commit: "sha-main"}, {Name: "release/v2.15", Commit: "sha-215"},
			}},
		},
		files: map[string]string{
			"rancher/rancher@sha-215:build.yaml":  "defaultSccOperatorImage: rancher/scc-operator:v0.5.2\n",
			"rancher/rancher@sha-main:build.yaml": "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n",
		},
	}
}

// withDevLine gives the consumer the line in development, v2.16, which has no
// branch of its own and so lives on the default branch.
func withDevLine(ws *fakeWorkspace) *fakeWorkspace {
	for i := range ws.links {
		c := &ws.links[i].Consumer
		c.Lines = append(slices.Clone(c.Lines), lineSpec{Line: "v2.16"})
	}
	return ws
}
