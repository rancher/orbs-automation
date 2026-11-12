// Package config carries the central config directory into the binary: the YAML
// files beside this one, as the files they are. It is the source the embedded
// bundle is compiled from (internal/bundle/embedded), so there is no second,
// generated copy of them to keep in step. Everything else in this directory is data.
package config

import "embed"

// Files is the YAML files of this directory.
//
//go:embed *.yaml
var Files embed.FS
