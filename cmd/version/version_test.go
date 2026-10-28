package version

import "testing"

func TestStringDefaults(t *testing.T) {
	if got, want := String(), "dev (commit none, built unknown)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestStringUsesInjectedValues(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = oldV, oldC, oldD })
	Version, Commit, Date = "1.2.3", "abc1234", "2025-01-02T03:04:05Z"

	want := "1.2.3 (commit abc1234, built 2025-01-02T03:04:05Z)"
	if got := String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
