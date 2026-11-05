package config

// The values an enumerated field may take (config/README.md).
const (
	BranchesSingle  = "single"
	BranchesPerLine = "perLine"

	AcceptStable = "stable"
	AcceptAll    = "all"

	AcceptLinesAll       = "all"
	AcceptLinesUniversal = "universal"

	ActionTrackOnly = "trackOnly"
	ActionReact     = "react"
	ActionNotify    = "notify"

	PhaseDevelopment = "development"
	PhaseGA          = "ga"
	PhaseEOM         = "eom"
	PhaseEOL         = "eol"
)
