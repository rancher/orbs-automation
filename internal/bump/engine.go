package bump

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/gomodedit"
	"github.com/rancher/orbs-automation/internal/platform/templates"
)

// ErrNotBuilt is returned by a run that needs a strategy gero names and does not
// have yet.
var ErrNotBuilt = errors.New("strategy declared and not built")

// ErrNoBranches is returned when no consumer branch can be worked out for a
// release: the consumer has none to follow it to.
var ErrNoBranches = errors.New("no consumer branch for the release")

// StrategyInput is what a strategy may use to make its change: where the release
// goes, and which one it is.
type StrategyInput struct {
	Edit     string
	Producer *chain.Project
	Consumer *chain.Project
	Release  Release
	Line     string
	Branch   string
	// Dir is the directory the tree is, for a strategy that runs a program in it.
	Dir string
	// Runner runs a program in a directory, as a step does, so a strategy runs
	// programs the way the steps of an edit do and no other way.
	Runner StepRunner
	// Changes is what the holdings of the edit wrote for this target, before the
	// strategy runs.
	Changes []Change
}

// Applier is a strategy configured for one edit: the extra change a named rule
// makes after the holdings are written.
type Applier interface {
	Apply(ctx context.Context, dst WritableTree, in StrategyInput) (Result, error)
}

// Strategy is code that claims a name, which an edit's `strategy` says. It is
// defined here and nowhere else.
type Strategy interface {
	Name() string
	// Configure checks the edit's `with` values and returns the Applier they
	// configure. It is where a typo in `with` is found, when the config loads.
	Configure(with map[string]string) (Applier, error)
}

// Engine knows the routes of the config and each edit's strategy, configured.
// Building it is where the mistakes that depend on a strategy or a format are
// found, which is also what gero doctor needs.
type Engine struct {
	graph    *chain.Graph
	appliers map[*chain.Edit]Applier
	unusable map[*chain.Holding]bool // a holding that cannot be written, or whose edit cannot be built
}

// NewEngine builds the engine for a config: it configures the strategy of each
// edit from the registry and checks that every release holding is in a format
// gero can write. It reports every problem it finds, each naming the edit or the
// holding, as one joined error. The engine is returned either way, holding the
// routes that are fine.
func NewEngine(strategies *StrategyRegistry, g *chain.Graph) (*Engine, error) {
	e := &Engine{graph: g, appliers: map[*chain.Edit]Applier{}, unusable: map[*chain.Holding]bool{}}
	var errs []error
	seen := map[*chain.Edit]bool{}
	for _, h := range g.Holdings() {
		if h.Kind() != chain.HoldsRelease {
			continue
		}
		for _, at := range h.Locations() {
			if _, built := formats[at.Format]; !built {
				errs = append(errs, fmt.Errorf("%w: Holding %q is in %s, which gero cannot read or write yet", ErrFormatNotBuilt, h.Name(), at.Format))
				e.unusable[h] = true
				break
			}
		}
		for _, ed := range h.Edits() {
			if !seen[ed] {
				seen[ed] = true
				if err := e.configure(strategies, ed); err != nil {
					errs = append(errs, err)
				}
			}
			if _, bad := e.appliers[ed]; ed.Strategy() != "" && !bad {
				e.unusable[h] = true
			}
		}
	}
	for _, v := range g.Virtual() {
		if e.unusable[v.PinnedBy()] {
			e.unusable[v] = true // it is written with the Holding it comes from, and with its edits
		}
	}
	return e, errors.Join(errs...)
}

// configure looks the edit's strategy up and configures it with the edit's values.
func (e *Engine) configure(strategies *StrategyRegistry, ed *chain.Edit) error {
	if ed.Strategy() == "" {
		return nil
	}
	strategy, err := strategies.Get(ed.Strategy())
	if err != nil {
		return fmt.Errorf("Edit %q: %w", ed.Name(), err)
	}
	applier, err := strategy.Configure(ed.With())
	if err != nil {
		return fmt.Errorf("Edit %q: %w", ed.Name(), err)
	}
	e.appliers[ed] = applier
	return nil
}

// Route is one way out of a producer: the release holdings a consumer has of it, as
// the chain says (chain.Route), and what the engine adds to run them.
type Route struct {
	chain.Route
	// SkipProducerSteps leaves out the steps that run from the producer's repo, for
	// a producer the active team does not own: gero runs nothing out of a repo no
	// team of its own owns. They are reported by SkippedSteps.
	SkipProducerSteps bool
	// GoReleases names the newest patch of a Go minor, for a go.mod that pins no
	// more than go 1.26 (the Edit's {goToolchain}). The caller gives it, as it gives the
	// runner, so the engine reads no host. Nil means no list: such a go.mod is an error.
	GoReleases GoReleases
	engine     *Engine
}

// GoReleases is the list of Go releases, as far as the engine needs it.
type GoReleases interface {
	// NewestPatch returns the toolchain of the newest stable patch of a minor given as
	// 1.26, such as go1.26.10.
	NewestPatch(ctx context.Context, minor string) (string, error)
}

// EnvOf is the environment the steps and strategies of the target's edits run in:
// the edits' env values rendered for the target, each variable read from the tree
// only when a value uses it. An entry that renders empty is left out. Two edits that
// set one name to different values are an error.
func (r Route) EnvOf(ctx context.Context, tree Tree, t Target) (map[string]string, error) {
	vars := map[string]string{"line": t.Line, "branch": t.Branch}
	haveToolchain := false
	out, setBy := map[string]string{}, map[string]string{}
	for _, ed := range chain.EditsOf(r.holdingsOf(t)) {
		env := ed.Env()
		for _, name := range slices.Sorted(maps.Keys(env)) {
			uses, err := templates.Placeholders(env[name])
			if err != nil {
				return nil, fmt.Errorf("Edit %q: env %s: %w", ed.Name(), name, err)
			}
			if slices.Contains(uses, "goToolchain") && !haveToolchain {
				if vars["goToolchain"], err = r.goToolchain(ctx, tree); err != nil {
					return nil, fmt.Errorf("Edit %q: env %s: %w", ed.Name(), name, err)
				}
				haveToolchain = true
			}
			value, err := templates.Render(env[name], vars)
			if err != nil {
				return nil, fmt.Errorf("Edit %q: env %s: %w", ed.Name(), name, err)
			}
			if value == "" {
				continue // nothing to say: a variable set to empty is not one left unset
			}
			if prev, ok := out[name]; ok && prev != value {
				return nil, fmt.Errorf("Edits %q and %q set %s to different values for %s: %q and %q", setBy[name], ed.Name(), name, t.Branch, prev, value)
			}
			out[name], setBy[name] = value, ed.Name()
		}
	}
	return out, nil
}

var (
	goPatch      = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)        // 1.26.3
	goPrerelease = regexp.MustCompile(`^[0-9]+\.[0-9]+(rc|beta)[0-9]+$`) // 1.27rc1
	goMinor      = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)                // 1.26
)

// goToolchain is the Go toolchain the tree pins, from the go.mod at its root: the
// toolchain line, else the go line when it names a patch, else the newest patch of
// its minor from the list of releases. It is empty when there is no go.mod or no go
// line, and an error when the file cannot be read or parsed, or a minor has no list
// to find its patch in.
func (r Route) goToolchain(ctx context.Context, tree Tree) (string, error) {
	data, err := tree.ReadFile(ctx, "go.mod")
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	goVersion, toolchain, err := gomodedit.Directives(data)
	if err != nil {
		return "", err
	}
	switch {
	case toolchain != "" && toolchain != "default":
		return toolchain, nil
	case goPatch.MatchString(goVersion) || goPrerelease.MatchString(goVersion):
		return "go" + goVersion, nil
	case !goMinor.MatchString(goVersion):
		return "", nil // no go line, or one that is not a version
	case r.GoReleases == nil:
		return "", fmt.Errorf("go.mod says go %s with no toolchain line, and there is no list of Go releases to find its patch in: add a toolchain line to it", goVersion)
	}
	newest, err := r.GoReleases.NewestPatch(ctx, goVersion)
	if err != nil {
		return "", fmt.Errorf("go.mod says go %s with no toolchain line, and its newest patch could not be found (%w): add a toolchain line to it", goVersion, err)
	}
	return newest, nil
}

// envAdding runs steps with an environment added to the one each step has, which a
// step's own values win over.
type envAdding struct {
	StepRunner
	env map[string]string
}

func (a envAdding) Run(ctx context.Context, step chain.Step, env StepEnv) error {
	merged := maps.Clone(a.env)
	maps.Copy(merged, env.Env)
	env.Env = merged
	return a.StepRunner.Run(ctx, step, env)
}

// withEnv is the runner with the edits' env added to every run of it, so a strategy
// that runs a program, such as goBump's go mod tidy, runs it in the same environment
// as the steps.
func withEnv(runner StepRunner, env map[string]string) StepRunner {
	if len(env) == 0 || runner == nil {
		return runner
	}
	return envAdding{StepRunner: runner, env: env}
}

// usable is the routes of the chain whose holdings can all be used, as engine routes:
// a holding that cannot be used leaves out the route it is in, and no other.
func (e *Engine) usable(routes []chain.Route) []Route {
	var out []Route
	for _, r := range routes {
		if !slices.ContainsFunc(r.Holdings, func(h *chain.Holding) bool { return e.unusable[h] }) {
			out = append(out, Route{Route: r, engine: e})
		}
	}
	return out
}

// RoutesFrom returns the routes out of a producer, one for each consumer and
// stream, in the order of the holdings, leaving out a route whose holdings or edits are not
// usable.
func (e *Engine) RoutesFrom(producer *chain.Project) []Route {
	return e.usable(producer.RoutesFrom())
}

// RoutesInto returns the routes into a consumer, one for each producer and stream, in the
// order of the holdings.
func (e *Engine) RoutesInto(consumer *chain.Project) []Route {
	return e.usable(consumer.RoutesInto())
}

// holdingsOf is the holdings a target writes: its own, or the route's when it
// names none.
func (r Route) holdingsOf(t Target) []*chain.Holding {
	if len(t.Holdings) > 0 {
		return t.Holdings
	}
	return r.Holdings
}

// changesOf is what the holdings of an edit wrote, in the order the edit lists them.
func changesOf(ed *chain.Edit, written map[*chain.Holding][]Change) []Change {
	var out []Change
	for _, h := range ed.Holdings() {
		out = append(out, written[h]...)
	}
	return out
}

// Target is a branch of the consumer a release goes to, and the holdings written
// there.
type Target struct {
	Branch   string
	Line     string // the Rancher line of the branch, empty for a branch of none
	Holdings []*chain.Holding
	// Pinned is the tag of each virtual Holding among Holdings: the release of the
	// pinned module that the pinning release requires, which the caller derived from
	// its file. The engine reads no host. A virtual Holding with no entry is an error.
	Pinned map[*chain.Holding]string
}

// releaseOf is the release a holding of the target holds: the tag of the run for a
// holding of the route, and for a companion, a virtual one of another producer's
// Artifact that follows the route's holding, the tag that was derived for it. A
// virtual holding of the route's own producer is shipped as any other: when lasso is
// shipped, what its pins make in a consumer is written with the tag of the run.
func (t Target) releaseOf(producer *chain.Project, h *chain.Holding, tag, commit string) (Release, error) {
	if !h.Virtual() || h.Producer() == producer {
		return ReleaseOf(h, tag, commit)
	}
	pinned, ok := t.Pinned[h]
	if !ok {
		return Release{}, fmt.Errorf("Holding %q is virtual and no release was derived for it", h.Name())
	}
	return ReleaseOf(h, pinned, "")
}

// Companions returns the virtual Holdings that follow the target's holdings: what
// the pins of their Artifacts make in the consumer, for the target's line, each once,
// in the order of the holdings they come from. A run that ships the release also
// writes these, with the releases the pins resolve; the caller gives them to the
// target in Holdings and Pinned.
func (r Route) Companions(t Target) []*chain.Holding {
	var out []*chain.Holding
	for _, h := range r.holdingsOf(t) {
		for _, v := range h.Pinned() {
			if (t.Line == "" || v.AppliesTo(t.Line)) && !slices.Contains(out, v) {
				out = append(out, v)
			}
		}
	}
	return out
}

// TargetsFor returns the targets a release goes to, in order. line is the
// producer's line the release is in, or empty when it is in none. A producer whose
// stream is universal sends it to every release branch of the consumer, narrowed
// to a holding's stage; one with lines sends it to the branch of its line.
func (r Route) TargetsFor(line string) ([]Target, error) {
	var targets []Target
	add := func(branch string, h *chain.Holding) {
		i := slices.IndexFunc(targets, func(t Target) bool { return t.Branch == branch })
		if i < 0 {
			targets = append(targets, Target{Branch: branch, Line: r.Consumer.LineOfBranch(branch)})
			i = len(targets) - 1
		}
		targets[i].Holdings = append(targets[i].Holdings, h)
	}
	noLine := false
	for _, h := range r.Holdings {
		stage := h.Stage()
		if h.Artifact().Stream().Universal() {
			for _, branch := range r.Consumer.BranchesOfStage(stage) {
				if l := r.Consumer.LineOfBranch(branch); l == "" || h.HoldsLine(l) {
					add(branch, h)
				}
			}
			continue
		}
		if line == "" {
			noLine = true
			continue
		}
		if !h.AppliesTo(line) {
			continue
		}
		if branch, ok := r.Consumer.BranchOfLine(line, stage); ok {
			add(branch, h)
		}
	}
	if len(targets) > 0 {
		return targets, nil
	}
	if noLine {
		return nil, fmt.Errorf("%w: the release is in none of the release lines of %s, so there is no branch to follow it to",
			ErrNoBranches, r.Producer.Slug())
	}
	if line != "" {
		return nil, fmt.Errorf("%w: %s has no branch for the release line %s, which the release is in", ErrNoBranches, r.Consumer.Slug(), line)
	}
	return nil, fmt.Errorf("%w: %s has no release branches", ErrNoBranches, r.Consumer.Slug())
}

// Phase is where in the lifecycle a step runs.
type Phase string

// The phases, in the order they run around the write.
const (
	PreEdit  Phase = "preEdit"
	PostEdit Phase = "postEdit"
)

// StepEnv is what a step runs with: the directory of the clone, and the
// environment the engine builds for it.
type StepEnv struct {
	Dir string
	Env map[string]string
}

// StepRunner runs one step. The engine decides which steps run and in what order;
// a runner owns how.
type StepRunner interface {
	Run(ctx context.Context, step chain.Step, env StepEnv) error
}

// StepRef is a step with the phase, the edit and the position it has.
type StepRef struct {
	Phase Phase
	Edit  string
	Index int
	Step  chain.Step
}

// Steps returns the steps that run for a target, in the order they run: the
// edits' pre-edit steps before the write and their post-edit steps after it. Only
// steps whose range holds the target's line run; a branch in no line gets every
// step that has no range. A route that skips the producer's steps leaves out the
// ones that run from the producer's repo (SkippedSteps).
func (r Route) Steps(t Target) (pre, post []StepRef) {
	ran, _ := r.split(t)
	for _, ref := range ran {
		if ref.Phase == PreEdit {
			pre = append(pre, ref)
		} else {
			post = append(post, ref)
		}
	}
	return pre, post
}

// SkippedSteps returns the steps Steps leaves out of the run for a target because
// they run from the producer's repo and the route skips those, in the order they
// would have run: the pre-edit steps, then the post-edit steps.
func (r Route) SkippedSteps(t Target) []StepRef {
	_, skipped := r.split(t)
	return skipped
}

// split is the steps of a target that apply to its line, divided into the ones that
// run and the ones the route skips, each in the order the steps would run: every
// pre-edit step of the edits, then every post-edit step.
func (r Route) split(t Target) (ran, skipped []StepRef) {
	var pre, post []StepRef
	collect := func(phase Phase, name string, steps []chain.Step) []StepRef {
		var out []StepRef
		for i, st := range steps {
			if st.AppliesTo(t.Line) {
				out = append(out, StepRef{Phase: phase, Edit: name, Index: i, Step: st})
			}
		}
		return out
	}
	for _, ed := range chain.EditsOf(r.holdingsOf(t)) {
		pre = append(pre, collect(PreEdit, ed.Name(), ed.PreEdit())...)
		post = append(post, collect(PostEdit, ed.Name(), ed.PostEdit())...)
	}
	for _, ref := range append(pre, post...) {
		if r.SkipProducerSteps && ref.Step.FromProducer() {
			skipped = append(skipped, ref)
		} else {
			ran = append(ran, ref)
		}
	}
	return ran, skipped
}

// StepError is a step that failed, naming where it was.
type StepError struct {
	Phase Phase
	Edit  string
	Index int
	Run   []string
	Err   error
}

func (e *StepError) Error() string {
	return fmt.Sprintf("%s[%d] of Edit %q: %s: %v", e.Phase, e.Index, e.Edit, strings.Join(e.Run, " "), e.Err)
}

func (e *StepError) Unwrap() error { return e.Err }

// UnitKind says what a Unit is.
type UnitKind string

// The kinds of unit.
const (
	UnitStep     UnitKind = "step"
	UnitEdit     UnitKind = "edit" // the holdings written
	UnitStrategy UnitKind = "strategy"
)

// Unit is one piece of the lifecycle that may leave changes behind.
type Unit struct {
	Kind     UnitKind
	Phase    Phase    // for a step: where it runs
	Edit     string   // for a step or a strategy: the edit it is of
	Strategy string   // for a strategy: its name
	Index    int      // for a step: its position in that edit's list
	Run      []string // for a step: its command
	Changes  []Change // for the write and a strategy: what it changed
}

// Checkpoint is called after each unit has finished, so the caller can act on
// what it left behind, such as committing it. An error stops the run.
type Checkpoint func(ctx context.Context, u Unit) error

// Ready says whether the route can be run: nothing it needs is declared and not
// built.
func (r Route) Ready() error { return r.readyFor(r.Holdings) }

func (r Route) readyFor(holdings []*chain.Holding) error {
	for _, ed := range chain.EditsOf(holdings) {
		if nb, ok := r.engine.appliers[ed].(notBuilt); ok {
			return fmt.Errorf("Edit %q: %w", ed.Name(), nb.NotBuilt())
		}
	}
	return nil
}

// HoldingCheck is what one holding of a target holds.
type HoldingCheck struct {
	Holding *chain.Holding
	Check
}

// Check reads what each holding of the target holds in src and how it compares
// with the release of tag. It never writes.
func (r Route) Check(ctx context.Context, src Tree, t Target, tag, commit string) ([]HoldingCheck, error) {
	var out []HoldingCheck
	for _, h := range r.holdingsOf(t) {
		rel, err := t.releaseOf(r.Producer, h, tag, commit)
		if err != nil {
			return nil, err
		}
		chk, err := CheckHolding(ctx, src, h, t.Line, t.Branch, rel)
		if err != nil {
			return nil, fmt.Errorf("Holding %q: %w", h.Name(), err)
		}
		out = append(out, HoldingCheck{Holding: h, Check: chk})
	}
	return out, nil
}

// Run runs the lifecycle of the target in dir: the pre-edit steps, the write of
// each holding, the strategies, then the post-edit steps, calling after (when it
// is not nil) once each has finished. A step that fails stops the run, as a
// *StepError; if it is a post-edit step the write has been made, and Result says
// what changed. A strategy that is declared and not built stops the run before
// anything is done.
func (r Route) Run(ctx context.Context, tree WritableTree, runner StepRunner, t Target, tag, commit, dir string, after Checkpoint) (Result, error) {
	holdings := r.holdingsOf(t)
	if err := r.readyFor(holdings); err != nil {
		return Result{}, err
	}
	pre, post := r.Steps(t)
	var pattern string // the tag pattern of the stream the holdings' artifacts come from
	if len(holdings) > 0 {
		pattern = tagPatternOf(holdings[0])
	}
	edits, err := r.EnvOf(ctx, tree, t) // before anything is written: a toolchain it cannot name stops the run
	if err != nil {
		return Result{}, err
	}
	env := StepEnv{Dir: dir, Env: map[string]string{}}
	maps.Copy(env.Env, edits)
	maps.Copy(env.Env, map[string]string{
		"GERO_TAG": tag, "GERO_VERSION": newRelease(tag, pattern, "", "").Version, "GERO_TARGET_BRANCH": t.Branch,
		"GERO_SOURCE_REPO": r.Producer.Slug(),
	})
	checkpoint := func(u Unit) error {
		if after == nil {
			return nil
		}
		return after(ctx, u)
	}
	step := func(ref StepRef) error {
		if err := runner.Run(ctx, ref.Step, env); err != nil {
			return &StepError{Phase: ref.Phase, Edit: ref.Edit, Index: ref.Index, Run: ref.Step.Run, Err: err}
		}
		return checkpoint(Unit{Kind: UnitStep, Phase: ref.Phase, Edit: ref.Edit, Index: ref.Index, Run: ref.Step.Run})
	}
	for _, ref := range pre {
		if err := step(ref); err != nil {
			return Result{}, err
		}
	}
	var res Result
	written := map[*chain.Holding][]Change{} // what each holding wrote
	for _, h := range holdings {
		rel, err := t.releaseOf(r.Producer, h, tag, commit)
		if err != nil {
			return res, err
		}
		got, err := ApplyHolding(ctx, tree, h, t.Line, t.Branch, rel)
		if err != nil {
			return res, fmt.Errorf("Holding %q: %w", h.Name(), err)
		}
		res.Changed = res.Changed || got.Changed
		res.Changes = append(res.Changes, got.Changes...)
		written[h] = got.Changes
	}
	if err := checkpoint(Unit{Kind: UnitEdit, Changes: res.Changes}); err != nil {
		return res, err
	}
	for _, ed := range chain.EditsOf(holdings) {
		applier := r.engine.appliers[ed]
		if applier == nil {
			continue
		}
		got, err := applier.Apply(ctx, tree, StrategyInput{
			Edit: ed.Name(), Producer: r.Producer, Consumer: r.Consumer,
			Release: newRelease(tag, tagPatternOf(holdings[0]), r.Producer.Slug(), commit),
			Line:    t.Line, Branch: t.Branch,
			Dir: dir, Runner: withEnv(runner, edits), Changes: changesOf(ed, written),
		})
		if err != nil {
			return res, fmt.Errorf("Edit %q: %w", ed.Name(), err)
		}
		res.Changed = res.Changed || got.Changed
		res.Changes = append(res.Changes, got.Changes...)
		if err := checkpoint(Unit{Kind: UnitStrategy, Edit: ed.Name(), Strategy: ed.Strategy(), Changes: got.Changes}); err != nil {
			return res, err
		}
	}
	for _, ref := range post {
		if err := step(ref); err != nil {
			return res, err
		}
	}
	return res, nil
}

// TargetsOn returns the targets of a line of the consumer, in order: for each
// holding that applies to the line, the consumer's branch of the line for the
// holding's stage. It is how a run to named lines of the consumer finds its
// branches. It is an error when no holding of the route has a branch for the line.
func (r Route) TargetsOn(line string) ([]Target, error) {
	targets := r.targetsOnLine(line, r.Holdings)
	if len(targets) == 0 {
		return nil, fmt.Errorf("%w: %s has no branch for the release line %s that a holding of %s is for", ErrNoBranches, r.Consumer.Slug(), line, r.Producer.Slug())
	}
	return targets, nil
}

// targetsOnLine groups the holdings that apply to the consumer's line by the
// branch each is on for its stage.
func (r Route) targetsOnLine(line string, holdings []*chain.Holding) []Target {
	var targets []Target
	for _, h := range holdings {
		if !h.AppliesTo(line) {
			continue
		}
		branch, ok := r.Consumer.BranchOfLine(line, h.Stage())
		if !ok {
			continue
		}
		i := slices.IndexFunc(targets, func(t Target) bool { return t.Branch == branch })
		if i < 0 {
			targets = append(targets, Target{Branch: branch, Line: r.Consumer.LineOfBranch(branch)})
			i = len(targets) - 1
		}
		targets[i].Holdings = append(targets[i].Holdings, h)
	}
	return targets
}
