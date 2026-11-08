package chain

// The names the graph presents to the domain, which the domain uses without
// importing the API version that spells them in config. names_test.go checks
// each is the one the config accepts.
const (
	LayoutSingle  = "single"
	LayoutPerLine = "perLine"

	HoldsRelease = "release"
	AcceptAll    = "all"

	RegistryDockerHub       = "dockerhub"
	RegistryPrimeStaging    = "prime-staging"
	RegistryPrimeProduction = "prime-production"
)
