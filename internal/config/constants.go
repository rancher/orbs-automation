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

	// RegistryDockerHub and the others name where an image is published. What
	// each means (host, and that production is for stable tags only) is built
	// into gero (docs/specs/IMAGES_EXIST.md).
	RegistryDockerHub       = "dockerhub"
	RegistryPrimeStaging    = "prime-staging"
	RegistryPrimeProduction = "prime-production"
)
