package semver

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		input string
		pre   bool
		build bool
	}{
		{"1.2.3", false, false},
		{"v1.2.3", false, false},
		{"1.2.3-rc.1", true, false},
		{"v1.2.3-beta", true, false},
		{"1.2.3+build.1", false, true},
		{"1.2.3-rc.1+build.1", true, true},
		{"1.2.3+build-5", false, true},
		{"1.2.3-hotfix-1", true, false},
		{"1.2.3-", true, false},
		{"01.2.3-rc", true, false},
		{"1.2.3-rc..1", true, false},
		{"hotfix+urgent", false, true},
		{"1.2-rc.1", false, false},
		{"1.2.3.4-rc", false, false},
		{"V1.2.3-rc", false, false},
		{"vv1.2.3-rc", false, false},
		{"qa-build-42", false, false},
		{"", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Classify(tt.input)
			if got.Input != tt.input {
				t.Errorf("Input = %q, want %q", got.Input, tt.input)
			}
			if got.HasPrerelease != tt.pre {
				t.Errorf("HasPrerelease = %v, want %v", got.HasPrerelease, tt.pre)
			}
			if got.HasBuildMeta != tt.build {
				t.Errorf("HasBuildMeta = %v, want %v", got.HasBuildMeta, tt.build)
			}
		})
	}
}

func TestClassifyStripsOneLeadingV(t *testing.T) {
	for input, want := range map[string]string{
		"v1.2.3":  "1.2.3",
		"vv1.2.3": "v1.2.3",
		"V1.2.3":  "V1.2.3",
		"1.2.3":   "1.2.3",
		"":        "",
	} {
		if got := Classify(input).Version; got != want {
			t.Errorf("Classify(%q).Version = %q, want %q", input, got, want)
		}
	}
}
