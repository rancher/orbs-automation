package v1alpha2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRefIsTheOneNameOfAKey(t *testing.T) {
	h := &Holding{TypeMeta: TypeMeta{Kind: KindHolding}, ObjectMeta: ObjectMeta{Refs: Refs{"project": {"p"}, "artifact": {"a", "b"}}}}

	assert.Equal(t, "p", Ref(h, "project"))
	assert.Empty(t, Ref(h, "artifact"), "several names are not one")
	assert.Empty(t, Ref(h, "from"), "a key not given")
}

func TestRefListIsTheNamesOfAKey(t *testing.T) {
	e := &Edit{TypeMeta: TypeMeta{Kind: KindEdit}, ObjectMeta: ObjectMeta{Refs: Refs{"holding": {"x", "y"}}}}

	assert.Equal(t, []string{"x", "y"}, RefList(e, "holding"))
}

func TestAKeyThatIsNotTheKindsOrNotOfThatShapeIsAMistake(t *testing.T) {
	h := &Holding{TypeMeta: TypeMeta{Kind: KindHolding}}
	e := &Edit{TypeMeta: TypeMeta{Kind: KindEdit}}

	assert.Panics(t, func() { Ref(h, "misspelt") })
	assert.Panics(t, func() { RefList(h, "misspelt") })
	assert.Panics(t, func() { Ref(e, "holding") }, "a list is read with RefList")
	assert.Panics(t, func() { RefList(h, "project") }, "one name is read with Ref")
}

func TestEveryKeyOfRefKeysIsReadByItsShape(t *testing.T) {
	kinds := map[string]Object{
		KindStream:   &Stream{TypeMeta: TypeMeta{Kind: KindStream}},
		KindArtifact: &Artifact{TypeMeta: TypeMeta{Kind: KindArtifact}},
		KindHolding:  &Holding{TypeMeta: TypeMeta{Kind: KindHolding}},
		KindEdit:     &Edit{TypeMeta: TypeMeta{Kind: KindEdit}},
	}
	for kind, keys := range refKeys {
		for key, k := range keys {
			o := kinds[kind]
			if k.list {
				assert.NotPanics(t, func() { RefList(o, key) }, "%s refs.%s", kind, key)
			} else {
				assert.NotPanics(t, func() { Ref(o, key) }, "%s refs.%s", kind, key)
			}
		}
	}
}
