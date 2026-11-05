package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every field of the config types is one of:
//
//	plain             used as written; an omitted value is the zero value
//	default           filled at load by loader.WithDefaults when omitted
//	resolved:<T.M>    left raw, and read through the method T.M, which gives the
//	                  value to act on; the test checks the method exists
//	layered:<rule>    a CLI setting can replace it; the rule is applied by a
//	                  Workspace method, which this package cannot check
//	open:<question>   not decided yet; the question is where it is recorded
//
// Adding a field fails this test until it is classed here, so the decision
// about raw versus effective is made when the field is added, not after a
// caller has read the raw value (docs/specs/WORKSPACE.md, "Raw and effective
// values").
var fieldClasses = map[string]string{
	"Project.Name":          "plain",
	"Project.Repo":          "plain",
	"Project.Release":       "plain",
	"Project.PullRequests":  "plain",
	"Project.Images":        "resolved:Project.EffectiveImages",
	"Project.UpdateMethods": "plain",
	"Project.Downstream":    "plain",
	"Project.Sources":       "plain",

	"Repo.Name":          "plain",
	"Repo.Owned":         "plain",
	"Repo.DefaultBranch": "default",

	"Release.Branches":           "default",
	"Release.TagPattern":         "default",
	"Release.RequiresPrerelease": "plain",
	"Release.Lines":              "resolved:Project.IsUniversal",

	"ProjectLine.Line":    "plain",
	"ProjectLine.Version": "resolved:ProjectLine.Series",
	"ProjectLine.Branch":  "resolved:Project.Branch",

	"UpdateMethod.Name":          "plain",
	"UpdateMethod.Strategy":      "plain",
	"UpdateMethod.With":          "plain",
	"UpdateMethod.AllowedParams": "plain",
	"UpdateMethod.Accept":        "plain",
	"UpdateMethod.PreEdit":       "plain",
	"UpdateMethod.PostEdit":      "plain",

	"AllowedParam.Key":         "plain",
	"AllowedParam.Description": "plain",
	"AllowedParam.Default":     "plain",
	"AllowedParam.Required":    "resolved:AllowedParam.IsRequired",

	"Edge.To":       "plain",
	"Edge.Method":   "plain",
	"Edge.Params":   "plain",
	"Edge.Lines":    "resolved:Edge.AcceptLines",
	"Edge.PreEdit":  "plain",
	"Edge.PostEdit": "plain",

	"Step.Run":   "plain",
	"Step.Since": "resolved:Step.AppliesTo",
	"Step.Until": "resolved:Step.AppliesTo",
	"Step.From":  "plain",

	"Image.Name":       "plain",
	"Image.Registries": "resolved:Project.EffectiveImages",
	"Image.Since":      "resolved:Image.AppliesTo",

	"PullRequests.Title":        "plain",
	"PullRequests.BranchPrefix": "layered:the branchPrefix user setting replaces it whole",
	"PullRequests.Labels":       "plain",

	"Source.From":   "plain",
	"Source.Accept": "plain",
	"Source.Update": "plain",

	"Accept.When":  "default",
	"Accept.Lines": "resolved:Source.AcceptLines",

	"Update.Strategy": "plain",
	"Update.Params":   "plain",
	"Update.PostEdit": "plain",

	"RancherLines.Name":  "plain",
	"RancherLines.Lines": "plain",

	"Line.ID":         "plain",
	"Line.Phase":      "resolved:Line.PhaseAt",
	"Line.Dates":      "plain",
	"Line.Kubernetes": "plain",

	"Dates.GA":  "plain",
	"Dates.EOM": "plain",
	"Dates.EOL": "plain",

	"KubernetesRange.Min": "plain",
	"KubernetesRange.Max": "plain",

	"External.Name":          "plain",
	"External.Repo":          "plain",
	"External.Relationships": "plain",

	"Relationship.Project": "plain",
	"Relationship.Action":  "plain",
}

func TestEveryConfigFieldIsClassified(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeFor[Project](), reflect.TypeFor[Image](), reflect.TypeFor[UpdateMethod](), reflect.TypeFor[AllowedParam](), reflect.TypeFor[Edge](), reflect.TypeFor[Step](), reflect.TypeFor[Repo](), reflect.TypeFor[Release](),
		reflect.TypeFor[ProjectLine](), reflect.TypeFor[PullRequests](), reflect.TypeFor[Source](),
		reflect.TypeFor[Accept](), reflect.TypeFor[Update](), reflect.TypeFor[RancherLines](),
		reflect.TypeFor[Line](), reflect.TypeFor[Dates](), reflect.TypeFor[KubernetesRange](),
		reflect.TypeFor[External](), reflect.TypeFor[Relationship](),
	}
	seen := map[string]bool{}
	for _, typ := range types {
		for i := range typ.NumField() {
			key := typ.Name() + "." + typ.Field(i).Name
			seen[key] = true
			class, ok := fieldClasses[key]
			if !assert.True(t, ok, "%s is not classed: add it to fieldClasses as plain, default, resolved:<Type.Method> or open:<question>", key) {
				continue
			}
			if method, isResolved := strings.CutPrefix(class, "resolved:"); isResolved {
				assert.True(t, hasMethod(types, method), "%s names %s, which does not exist", key, method)
			}
		}
	}
	for key := range fieldClasses {
		assert.True(t, seen[key], "%s is classed but is not a field of a config type", key)
	}
}

func hasMethod(types []reflect.Type, name string) bool {
	typeName, method, _ := strings.Cut(name, ".")
	for _, typ := range types {
		if typ.Name() == typeName {
			_, ok := typ.MethodByName(method)
			return ok
		}
	}
	return false
}
