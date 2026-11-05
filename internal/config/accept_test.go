package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSourceAcceptLines(t *testing.T) {
	lined := Project{Release: Release{Lines: []ProjectLine{{Line: "v2.16"}}}}
	universalProducer := Project{}

	tests := []struct {
		name     string
		accept   string
		producer Project
		want     string
	}{
		{"omitted for a universal producer is universal", "", universalProducer, AcceptLinesUniversal},
		{"omitted for a producer with lines is all", "", lined, AcceptLinesAll},
		{"an explicit all is kept for a universal producer", AcceptLinesAll, universalProducer, AcceptLinesAll},
		{"an explicit universal is kept for a producer with lines", AcceptLinesUniversal, lined, AcceptLinesUniversal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Source{Accept: Accept{Lines: tt.accept}}
			assert.Equal(t, tt.want, s.AcceptLines(tt.producer))
		})
	}
}
