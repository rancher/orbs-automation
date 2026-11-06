package ship

// mergeBody makes the body a PR should have from the one it has and the one that
// would be made for it now: the visible text and the blocks gero ship writes come
// from the new one, and any other hidden gero block already on the PR (an
// HTML comment that starts <!-- gero:<kind>, such as the one the CI tracker will
// keep there) is kept exactly as it is, after them. Nothing that gero does not own
// is ever dropped for being unknown, so a block added by another part of gero
// survives a refresh from this one.
//
// STUB: the new body as it is, dropping what is there, until a later commit.
func mergeBody(existing, generated string) string { return generated }

// sameBody reports whether two bodies are the same text, leaving out the line
// endings GitHub may change and the whitespace at the ends.
//
// STUB: never the same until a later commit.
func sameBody(a, b string) bool { return false }
