package v1alpha2

import "fmt"

// Ref is the name a document gives at a key of its refs, for a key that takes one
// name; it is "" when the document gives none or several, which the rules across
// documents report. It is how a name is read from a document: the key must be one
// refKeys has for the kind, or it panics, so a misspelt key is found by any test
// that reaches it and not by a ref that is silently never there.
func Ref(o Object, key string) string {
	if refKeyOf(o, key).list {
		panic(fmt.Sprintf("v1alpha2: refs.%s of a %s takes a list; use RefList", key, o.GetKind()))
	}
	if names := o.GetRefs()[key]; len(names) == 1 {
		return names[0]
	}
	return ""
}

// RefList is the names a document gives at a key of its refs that takes a list,
// in the order written.
func RefList(o Object, key string) []string {
	if !refKeyOf(o, key).list {
		panic(fmt.Sprintf("v1alpha2: refs.%s of a %s takes one name; use Ref", key, o.GetKind()))
	}
	return o.GetRefs()[key]
}

// refNames is the names a document gives at a key of either shape.
func refNames(o Object, key string) []string {
	refKeyOf(o, key)
	return o.GetRefs()[key]
}

// refKeyOf is the key of the kind's refs, which must be there.
func refKeyOf(o Object, key string) refKey {
	k, ok := refKeys[o.GetKind()][key]
	if !ok {
		panic(fmt.Sprintf("v1alpha2: %s has no refs.%s", o.GetKind(), key))
	}
	return k
}
