package ship

import "strings"

const (
	blockOpen  = "<!-- gero:"
	blockClose = "-->"
)

// mergeBody makes the body a PR should have from the one it has and the one that
// would be made for it now: the visible text and the blocks gero ship writes come
// from the new one, and any other hidden gero block already on the PR (an
// HTML comment that starts <!-- gero:<kind>, such as the one the CI tracker will
// keep there) is kept exactly as it is, after them. Nothing that gero does not own
// is ever dropped for being unknown, so a block added by another part of gero
// survives a refresh from this one. A block of a kind the new body also has is
// the new one's. Other comments are not gero's and are not carried over.
func mergeBody(existing, generated string) string {
	own := map[string]bool{}
	for _, b := range geroBlocks(generated) {
		own[b.kind] = true
	}
	merged := generated
	for _, b := range geroBlocks(existing) {
		if own[b.kind] {
			continue
		}
		merged = strings.TrimRight(merged, "\n") + "\n\n" + b.text + "\n"
	}
	return merged
}

type geroBlock struct {
	kind string // the word after gero:, such as ship or tracker
	text string // the whole comment, as written
}

// geroBlocks finds every <!-- gero:<kind> ... --> comment in a body, in order.
func geroBlocks(body string) []geroBlock {
	var out []geroBlock
	for {
		start := strings.Index(body, blockOpen)
		if start < 0 {
			return out
		}
		rest := body[start:]
		end := strings.Index(rest, blockClose)
		if end < 0 {
			return out
		}
		text := rest[:end+len(blockClose)]
		kind, _, _ := strings.Cut(strings.TrimPrefix(text, blockOpen), " ")
		kind, _, _ = strings.Cut(kind, "\n")
		out = append(out, geroBlock{kind: kind, text: text})
		body = rest[end+len(blockClose):]
	}
}

// sameBody reports whether two bodies are the same text, leaving out the line
// endings GitHub may change and the whitespace at the ends.
func sameBody(a, b string) bool { return normalizeBody(a) == normalizeBody(b) }

func normalizeBody(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}
