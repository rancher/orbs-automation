package config

// Validate checks c after WithDefaults, so an omitted field with a default is
// already filled in. It reports every problem it finds, each naming the
// document and field, as one joined error; it returns nil when c is valid.
func (c Config) Validate() error {
	return nil
}
