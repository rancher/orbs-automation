package v1alpha2

import "github.com/rancher/orbs-automation/internal/apis/lines"

// KindRancherLines is the kind of the one document that lists the Rancher lines.
const KindRancherLines = "RancherLines"

// RancherLines is the set of Rancher lines and facts about each. It is public
// product data, with no branches: each project maps its own.
type RancherLines struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       RancherLinesSpec `yaml:"spec"`
}

// RancherLinesSpec is the fields of RancherLines.
type RancherLinesSpec struct {
	Lines []lines.Line `yaml:"lines"`
}
