package v1alpha2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestARangeContainsALine(t *testing.T) {
	tests := []struct {
		name  string
		rng   Range
		line  string
		count bool
	}{
		{"the zero range holds every line", Range{}, "v2.16", true},
		{"since holds its own line", Range{Since: "v2.15"}, "v2.15", true},
		{"since holds a later line", Range{Since: "v2.15"}, "v2.16", true},
		{"since does not hold an earlier line", Range{Since: "v2.15"}, "v2.14", false},
		{"until holds its own line", Range{Until: "v2.15"}, "v2.15", true},
		{"until holds an earlier line", Range{Until: "v2.15"}, "v2.9", true},
		{"until does not hold a later line", Range{Until: "v2.15"}, "v2.16", false},
		{"both ends", Range{Since: "v2.14", Until: "v2.16"}, "v2.15", true},
		{"the minor is a number, not text", Range{Since: "v2.9"}, "v2.10", true},
		{"the major comes first", Range{Since: "v2.9"}, "v3.0", true},
		{"a line without its v", Range{Since: "v2.15"}, "2.16", true},
		{"a line in capitals", Range{Since: "v2.15"}, "V2.14", false},
		{"a line that is not a line", Range{Since: "v2.15"}, "latest", true},
		{"an end that is not a line is open", Range{Since: "soon"}, "v2.1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.count, tt.rng.Contains(tt.line))
		})
	}
}
