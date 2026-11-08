// Package meta holds what every version of the config API shares: the envelope
// of a document (its apiVersion, kind, name and refs), and a Scheme that decodes
// a YAML stream into the typed objects the versions register. A version package
// such as v1alpha2 defines its kinds and registers them with AddToScheme; nothing
// here knows what a Project or a Stream is.
package meta
