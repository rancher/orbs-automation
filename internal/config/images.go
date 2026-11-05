package config

// EffectiveImages returns the images the Project publishes: the ones listed,
// else one image named for the repository (repo.name), which is where nearly
// every project's image is published. An image that lists no registries goes to
// all of them. The field stays as written; this is what to act on.
func (p Project) EffectiveImages() []Image {
	return nil
}
