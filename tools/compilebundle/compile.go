package main

import (
	"io/fs"
	"time"
)

// Header is what the first document of a bundle says about it.
type Header struct {
	GeroVersion  string    // the gero release the bundle is made for, such as v0.5.0
	SourceCommit string    // the commit of the central repo that config was read from
	CompiledAt   time.Time // when, as the release job says: never the clock, so a build repeats
}

// bundleName is the file name of the bundle, in the output directory.
const bundleName = "gero-config-bundle.yaml"

// Compile validates the config under fsys as every command does and returns the
// bundle: the header document, then each YAML file's documents as written, in the
// order of the file names.
func Compile(fsys fs.FS, h Header) ([]byte, error) {
	return nil, nil
}
