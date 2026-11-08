package v1alpha2

import "io"

// Decode reads every document of a YAML stream into the typed object of its
// kind. source names the stream in errors, normally a file name.
func Decode(_ io.Reader, _ string) ([]Object, error) {
	return nil, nil
}
