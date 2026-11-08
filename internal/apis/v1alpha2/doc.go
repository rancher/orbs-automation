// Package v1alpha2 holds the types of the application config as apiVersion
// gero/v1alpha2: a set of small kinds (Project, Stream, Artifact, Holding and
// Edit) that share one envelope and point at each other by name under refs.
// The shapes are specified in docs/design/CONFIG_SCHEMA.md. The package decodes
// documents into those types and checks nothing else: defaults, rules inside one
// kind and rules across documents are added in their own steps.
//
// The envelope and the decoding are shared by every version, in
// internal/apis/meta; this package defines its kinds and registers them.
//
// The old shape, apiVersion gero/v1alpha1, is gone: a document of it is refused.
package v1alpha2
