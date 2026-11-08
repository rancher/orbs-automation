package meta

// TypeMeta says which shape a document is.
type TypeMeta struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

// GetAPIVersion returns the document's apiVersion.
func (t TypeMeta) GetAPIVersion() string { return t.APIVersion }

// GetKind returns the document's kind.
func (t TypeMeta) GetKind() string { return t.Kind }

// ObjectMeta is what identifies a document and connects it to others.
type ObjectMeta struct {
	// Name is unique within the kind, and is how refs and logs name the document.
	Name string `yaml:"name"`
	// Refs holds every reference to another document: names only, and the key
	// says which kind the name is of.
	Refs Refs `yaml:"refs"`
}

// GetName returns the document's name.
func (o ObjectMeta) GetName() string { return o.Name }

// GetRefs returns the document's references.
func (o ObjectMeta) GetRefs() Refs { return o.Refs }

// Refs maps a reference key to the names it points at. It is written as one name
// or a list of names.
type Refs map[string][]string

// Object is a decoded document of any kind.
type Object interface {
	GetAPIVersion() string
	GetKind() string
	GetName() string
	GetRefs() Refs
}
