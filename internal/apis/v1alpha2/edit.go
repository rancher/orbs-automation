package v1alpha2

// Edit is the extra steps when shipping, beyond writing a holding's form.
type Edit struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       EditSpec `yaml:"spec"`
}

// EditSpec is the fields of an Edit.
type EditSpec struct {
	PreEdit  []Step            `yaml:"preEdit"`
	PostEdit []Step            `yaml:"postEdit"`
	Strategy string            `yaml:"strategy"`
	With     map[string]string `yaml:"with"`
	// Env is the environment every step and strategy of the Edit runs in for one
	// target: names to templates (templates.EnvVars).
	Env map[string]string `yaml:"env"`
}
