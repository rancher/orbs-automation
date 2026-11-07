package updatedispatcher

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
)

// Method is a consumer's method with its strategy configured.
type Method struct {
	Project string // the consumer's config name
	ActionMethod
	Updater Updater
}

// Engine holds every update method of the loaded config, each with its
// strategy configured. Building it is where the mistakes that depend on a
// strategy are found, which is also what gero doctor needs.
type Engine struct {
	projects []config.Project
	methods  map[methodKey]*Method
}

type methodKey struct{ project, method string }

// NewEngine builds the engine for a loaded config, looking each method's
// strategy up in the registry and configuring it, and checking that the method
// offers every input the strategy requires and that every edge's params fit the
// method it names. It reports every problem it finds, each naming the method or
// edge, as one joined error. The engine is returned either way, holding the
// methods that were fine.
func NewEngine(strategies *StrategyRegistry, cfg config.Config) (*Engine, error) {
	e := &Engine{projects: cfg.Projects, methods: map[methodKey]*Method{}}
	var errs []error
	for _, p := range cfg.Projects {
		for _, raw := range p.UpdateMethods {
			m, err := buildMethod(strategies, p.Name, raw)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			e.methods[methodKey{p.Name, raw.Name}] = m
		}
	}
	for _, p := range cfg.Projects {
		for i, edge := range p.Downstream {
			route, ok := e.route(edge)
			if !ok {
				continue // a missing consumer or method is config's reference check
			}
			if _, err := route.Method.Bind(edge); err != nil {
				errs = append(errs, fmt.Errorf("Project %q: downstream[%d]: %w", p.Name, i, err))
			}
		}
	}
	return e, errors.Join(errs...)
}

func buildMethod(strategies *StrategyRegistry, project string, raw config.UpdateMethod) (*Method, error) {
	at := fmt.Sprintf("Project %q: method %q", project, raw.Name)
	m := TranslateConfig(raw)
	strategy, err := strategies.Get(m.StrategyName)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", at, err)
	}
	var errs []error
	for _, need := range strategy.Requires() {
		if _, ok := m.Param(need); !ok {
			errs = append(errs, fmt.Errorf("%s: strategy %s needs the input %q: offer it under allowedParams", at, m.StrategyName, need))
		}
	}
	updater, err := strategy.Configure(m.With)
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", at, err))
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &Method{Project: project, ActionMethod: m, Updater: updater}, nil
}

// Method returns a consumer's method by project config name and method name.
func (e *Engine) Method(project, name string) (*Method, bool) {
	m, ok := e.methods[methodKey{project, name}]
	return m, ok
}

// Route is one way out of a producer: the downstream edge, the consumer it
// names, and the consumer's method it invokes.
type Route struct {
	Edge     config.Edge
	Consumer config.Project
	Method   *Method
}

// RoutesFrom returns the producer's routes in config order: each downstream edge
// whose consumer exists and whose method was built.
func (e *Engine) RoutesFrom(producer config.Project) []Route {
	var out []Route
	for _, edge := range producer.Downstream {
		if r, ok := e.route(edge); ok {
			out = append(out, r)
		}
	}
	return out
}

// IncomingRoute is a way into a consumer: the producer whose edge names it, and
// the route that edge is.
type IncomingRoute struct {
	Producer config.Project
	Route
}

// RoutesInto returns the routes into the consumer, in the config order of the
// producers and of their edges: the reverse view of RoutesFrom, for what a project
// takes and not what it ships. An edge whose method was not built is left out, as
// RoutesFrom leaves it out.
func (e *Engine) RoutesInto(consumer config.Project) []IncomingRoute {
	var out []IncomingRoute
	for _, producer := range e.projects {
		for _, edge := range producer.Downstream {
			if r, ok := e.route(edge); ok && r.Consumer.Name == consumer.Name {
				out = append(out, IncomingRoute{Producer: producer, Route: r})
			}
		}
	}
	return out
}

func (e *Engine) route(edge config.Edge) (Route, bool) {
	for _, p := range e.projects {
		if p.Name != edge.To && !strings.EqualFold(p.Repo.Name, edge.To) {
			continue
		}
		m, ok := e.Method(p.Name, edge.Method)
		return Route{Edge: edge, Consumer: p, Method: m}, ok
	}
	return Route{}, false
}

// Bind checks an edge's params against what the method offers and returns the
// inputs of the call: unknown params and missing required ones are errors, and
// the defaults of the params the edge leaves out are filled in.
func (m *Method) Bind(edge config.Edge) (Inputs, error) {
	at := fmt.Sprintf("method %q of %s", m.Name, m.Project)
	in := Inputs{}
	var errs []error
	for _, def := range m.AllowedParams {
		if def.Default != "" {
			in[def.Key] = def.Default
		}
	}
	keys := make([]string, 0, len(edge.Params))
	for key := range edge.Params {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if _, ok := m.Param(key); !ok {
			errs = append(errs, fmt.Errorf("param %q is not allowed by %s: it allows %s", key, at, m.allowed()))
			continue
		}
		in[key] = edge.Params[key]
	}
	for _, def := range m.AllowedParams {
		if _, ok := in[def.Key]; !ok && def.Required {
			errs = append(errs, fmt.Errorf("param %q is required by %s", def.Key, at))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return in, nil
}

func (m *Method) allowed() string {
	if len(m.AllowedParams) == 0 {
		return "no params"
	}
	keys := make([]string, len(m.AllowedParams))
	for i, p := range m.AllowedParams {
		keys[i] = p.Key
	}
	return strings.Join(keys, ", ")
}
