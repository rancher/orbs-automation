package v1alpha2

import "github.com/rancher/orbs-automation/internal/apis/meta"

// APIVersion is the apiVersion of every document in this package.
const APIVersion = "gero/v1alpha2"

// Document kinds.
const (
	KindProject  = "Project"
	KindStream   = "Stream"
	KindArtifact = "Artifact"
	KindHolding  = "Holding"
	KindEdit     = "Edit"
)

// The envelope is shared by every version (internal/apis/meta); these are its
// names here, so the kinds of this version read the same as the rest.
type (
	TypeMeta   = meta.TypeMeta
	ObjectMeta = meta.ObjectMeta
	Refs       = meta.Refs
	Object     = meta.Object

	Acknowledgement = meta.Acknowledgement
)
