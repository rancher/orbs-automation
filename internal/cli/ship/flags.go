package ship

import "github.com/spf13/pflag"

// BindFlags declares on flags the flags that gero ship and gero ci:ship share,
// and binds them to in. What is only the workstation's to say (asking, pushing
// or not, where to push) is declared by gero ship alone.
func BindFlags(flags *pflag.FlagSet, in *Options) {
	flags.StringVar(&in.To, "to", "", "the consumer to ship to, when the project ships to several")
	flags.StringSliceVar(&in.Lines, "line", nil, "ship only to this Rancher line of the consumer (v2.15), on whichever branch it keeps that line, instead of every line the release goes to; repeat it or separate lines with commas for several")
	flags.BoolVar(&in.Prerelease, "prerelease", false, "let prereleases count")
	flags.BoolVar(&in.AllowDowngrade, "allow-downgrade", false, "ship a release older than what the consumer holds")
	flags.BoolVar(&in.Preview, "preview", false, "print the commit message, the PR body and the commit each step would make too")
	flags.BoolVar(&in.Refresh, "refresh", false, "rebuild your open PR for this release and replace its branch if it differs")
}
