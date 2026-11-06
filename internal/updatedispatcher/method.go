package updatedispatcher

import "github.com/rancher/orbs-automation/internal/config"

// ParamDef is a param a method offers a producer, as the consumer's config
// declares it. It is the only declaration of a param: no strategy has its own.
type ParamDef struct {
	Key         string
	Description string
	Default     string // what it is when the producer passes nothing
	Required    bool   // the producer must pass it
}

// ActionMethod is a consumer's update method as the domain sees it, without the
// YAML tags of the config structs it is made from.
type ActionMethod struct {
	Name          string
	StrategyName  string
	With          map[string]string // fixed by the consumer, for the strategy's Configure
	AllowedParams []ParamDef        // in the order the config lists them
	Accept        string            // config.AcceptStable or config.AcceptAll
	PreEdit       []config.Step     // steps are plain data, so the config type is used as it is
	PostEdit      []config.Step
}

// Param returns the offered param of a key.
func (m ActionMethod) Param(key string) (ParamDef, bool) {
	for _, p := range m.AllowedParams {
		if p.Key == key {
			return p, true
		}
	}
	return ParamDef{}, false
}

// TranslateConfig turns a method as the config structs have it into the domain
// model. A param is required by the rule config owns, AllowedParam.IsRequired.
func TranslateConfig(raw config.UpdateMethod) ActionMethod {
	params := make([]ParamDef, len(raw.AllowedParams))
	for i, p := range raw.AllowedParams {
		params[i] = ParamDef{Key: p.Key, Description: p.Description, Default: p.Default, Required: p.IsRequired()}
	}
	return ActionMethod{
		Name:          raw.Name,
		StrategyName:  raw.Strategy,
		With:          raw.With,
		AllowedParams: params,
		Accept:        raw.Accept.When,
		PreEdit:       raw.PreEdit,
		PostEdit:      raw.PostEdit,
	}
}
