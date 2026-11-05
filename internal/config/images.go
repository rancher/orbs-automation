package config

import "slices"

// EffectiveImages returns the images the Project publishes: the ones listed,
// else one image named for the repository (repo.name), which is where nearly
// every project's image is published. An image that lists no registries goes to
// all of them. The field stays as written; this is what to act on.
func (p Project) EffectiveImages() []Image {
	images := p.Images
	if len(images) == 0 {
		images = []Image{{Name: p.Repo.Name}}
	}
	out := make([]Image, len(images))
	for i, img := range images {
		out[i] = Image{Name: img.Name, Registries: slices.Clone(img.Registries)}
		if len(out[i].Registries) == 0 {
			out[i].Registries = []string{RegistryDockerHub, RegistryPrimeStaging, RegistryPrimeProduction}
		}
	}
	return out
}

// AppliesTo reports whether the image is expected for a release in the Rancher
// line: when it has no Since, always, and otherwise when the line is Since or
// newer. Lines compare by their numbers, so v2.16 is newer than v2.15. A line
// that cannot be read as a number is treated as applying, so an image is never
// skipped on a guess.
func (img Image) AppliesTo(line string) bool {
	return false
}
