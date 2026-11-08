package main

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// built is the output of build, run from the repository root as the command is.
func built(t *testing.T) map[string]map[string]any {
	t.Helper()
	t.Chdir("../..")
	files, err := build()
	require.NoError(t, err)
	docs := map[string]map[string]any{}
	for name, data := range files {
		var doc map[string]any
		require.NoError(t, json.Unmarshal(data, &doc), name)
		docs[name] = doc
	}
	return docs
}

// dig reads a value of a decoded JSON document by its path of keys, and fails the
// test when a key is missing.
func dig(t *testing.T, doc any, path ...string) any {
	t.Helper()
	for i, key := range path {
		m, ok := doc.(map[string]any)
		require.True(t, ok, "%v is not an object", path[:i])
		doc, ok = m[key]
		require.True(t, ok, "no %q at %v", key, path[:i])
	}
	return doc
}

func TestBuildMakesAFileForEachKindThePairAndTheUserFile(t *testing.T) {
	var names []string
	for name := range built(t) {
		names = append(names, name)
	}
	sort.Strings(names)
	assert.Equal(t, []string{
		"gero.v1alpha2.schema.json",
		"user-config.schema.json",
		"v1alpha2/artifact.schema.json",
		"v1alpha2/edit.schema.json",
		"v1alpha2/holding.schema.json",
		"v1alpha2/project.schema.json",
		"v1alpha2/rancherlines.schema.json",
		"v1alpha2/stream.schema.json",
	}, names)
}

func TestAKindSchemaPinsItsVersionAndKindAndRequiresTheEnvelope(t *testing.T) {
	doc := built(t)["v1alpha2/project.schema.json"]
	assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", doc["$schema"])
	assert.Equal(t, "#/$defs/Project", doc["$ref"])

	project := dig(t, doc, "$defs", "Project")
	assert.Equal(t, "gero/v1alpha2", dig(t, project, "properties", "apiVersion", "const"))
	assert.Equal(t, "Project", dig(t, project, "properties", "kind", "const"))
	assert.Equal(t, `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, dig(t, project, "properties", "name", "pattern"))
	assert.ElementsMatch(t, []any{"apiVersion", "kind", "name", "spec"}, dig(t, project, "required"))
	assert.Equal(t, false, dig(t, project, "additionalProperties"), "an unknown key is refused, as strict decoding does")
}

func TestFieldsAreNamedByTheirYAMLKeyAndAreNotRequired(t *testing.T) {
	spec := dig(t, built(t)["v1alpha2/project.schema.json"], "$defs", "ProjectSpec")
	assert.Contains(t, dig(t, spec, "properties"), "finalDestination")
	assert.Contains(t, dig(t, spec, "properties"), "pullRequests")
	assert.NotContains(t, dig(t, spec, "properties"), "FinalDestination")
	assert.NotContains(t, spec, "required")
}

func TestAFieldWithAFixedSetOfValuesListsThem(t *testing.T) {
	docs := built(t)
	assert.Equal(t, []any{"release", "commit", "branch"}, dig(t, docs["v1alpha2/holding.schema.json"], "$defs", "Holds", "properties", "kind", "enum"))
	assert.Equal(t, []any{"single", "perLine"}, dig(t, docs["v1alpha2/project.schema.json"], "$defs", "Branches", "properties", "layout", "enum"))
	assert.Equal(t, []any{"dockerhub", "prime-staging", "prime-production"}, dig(t, docs["v1alpha2/artifact.schema.json"], "$defs", "ArtifactSpec", "properties", "registries", "items", "enum"), "for a list, the values of an item")
}

func TestATypeThatDecodesItselfIsDescribedByTheFormsItReads(t *testing.T) {
	doc := built(t)["v1alpha2/project.schema.json"]
	branch := dig(t, doc, "$defs", "BranchLine", "properties", "branch")
	forms := dig(t, branch, "oneOf").([]any)
	require.Len(t, forms, 2)
	assert.Equal(t, "string", dig(t, forms[0], "type"))
	assert.Equal(t, "object", dig(t, forms[1], "type"))

	refs := dig(t, doc, "$defs", "Project", "properties", "refs")
	assert.Equal(t, "object", dig(t, refs, "type"))
	assert.NotNil(t, dig(t, refs, "additionalProperties", "oneOf"), "a name or a list of names")
}

func TestADateIsADateAndNotATimestamp(t *testing.T) {
	dates := dig(t, built(t)["v1alpha2/rancherlines.schema.json"], "$defs", "Dates", "properties", "ga")
	assert.Equal(t, "string", dig(t, dates, "type"))
	assert.Equal(t, "date", dig(t, dates, "format"))
}

func TestTheGoCommentsAreTheDescriptions(t *testing.T) {
	doc := built(t)["v1alpha2/project.schema.json"]
	assert.NotEmpty(t, dig(t, doc, "$defs", "Project", "description"))
	assert.NotEmpty(t, dig(t, doc, "$defs", "Repo", "properties", "name", "description"))
}

func TestThePairPicksTheSchemaOfAKindByItsKind(t *testing.T) {
	doc := built(t)["gero.v1alpha2.schema.json"]
	branches := dig(t, doc, "allOf").([]any)
	require.Len(t, branches, 6, "one for each kind")
	kinds := map[any]any{}
	for _, b := range branches {
		kinds[dig(t, b, "if", "properties", "kind", "const")] = dig(t, b, "then", "$ref")
	}
	assert.Equal(t, "#/$defs/Project", kinds["Project"])
	assert.Equal(t, "#/$defs/RancherLines", kinds["RancherLines"])
	assert.Contains(t, dig(t, doc, "$defs"), "Holds", "the definitions of every kind are in it")
}

func TestTheUserFileHasNoEnvelopeAndNamesItsValues(t *testing.T) {
	doc := built(t)["user-config.schema.json"]
	file := dig(t, doc, "$defs", "fileConfig")
	assert.Contains(t, dig(t, file, "properties"), "githubUser")
	assert.NotContains(t, dig(t, file, "properties"), "apiVersion")
	assert.Equal(t, []any{"text", "json", "env"}, dig(t, file, "properties", "output", "enum"))
	assert.Equal(t, []any{"auto", "always", "never"}, dig(t, file, "properties", "color", "enum"))
	assert.Equal(t, []any{"fork", "upstream"}, dig(t, file, "properties", "pushTo", "enum"))
}

func TestBuildIsTheSameEveryTime(t *testing.T) {
	t.Chdir("../..")
	first, err := build()
	require.NoError(t, err)
	second, err := build()
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestAHintOnAFieldThatDoesNotExistIsAnError(t *testing.T) {
	defs := jsonschema.Definitions{"Holds": {Properties: jsonschema.NewProperties()}}
	defs["Holds"].Properties.Set("kind", &jsonschema.Schema{Type: "string"})

	require.NoError(t, applyHint(defs, hint{def: "Holds", key: "kind", enum: []string{"a"}}))
	assert.Equal(t, []any{"a"}, defs["Holds"].Properties.Value("kind").Enum)

	assert.Error(t, applyHint(defs, hint{def: "Holds", key: "kynd", enum: []string{"a"}}), "a renamed field")
	assert.Error(t, applyHint(defs, hint{def: "Hold", key: "kind", enum: []string{"a"}}), "a renamed type")
}
