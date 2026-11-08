package config

// The values an enumerated field may take (config/README.md).
const (
	BranchingSingle  = "single"
	BranchingPerLine = "perLine"
	// BranchingPerLineTag is one branch whose lines are found in its tags.
	BranchingPerLineTag = "perLineTag"

	// TagSchemeSemver is the default scheme. TagSchemeCalverLineStage is
	// v{minor}-{YYYYMMDD}T{HHMM}Z with -dev for the dev stage
	// (docs/specs/CHAIN_TRACING.md).
	TagSchemeSemver          = "semver"
	TagSchemeCalverLineStage = "calverLineStage"

	// TeamOrbs is who manages an owned project that names no other team.
	TeamOrbs = "orbs"

	// The stages a tag scheme can give.
	StageDev     = "dev"
	StageRelease = "release"
	// StageStable is the second stage; for now it has the value of StageRelease.
	StageStable = "release"

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
