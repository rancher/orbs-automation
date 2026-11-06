package shellenv

import (
	"errors"
	"strings"
)

// ErrBrokenBlock is returned when a profile holds one of the block's markers
// without the other, or more than one block, so there is no safe region to edit.
var ErrBrokenBlock = errors.New("the gero block in the profile is damaged")

// Insert returns content with block in it: replacing the block that is there,
// else added at the end after a blank line. changed is false when the content
// already holds exactly that block.
func Insert(content, block string) (out string, changed bool, err error) {
	start, end, found, err := locate(content)
	if err != nil {
		return "", false, err
	}
	if !found {
		sep := ""
		switch {
		case content == "":
		case strings.HasSuffix(content, "\n\n"):
		case strings.HasSuffix(content, "\n"):
			sep = "\n"
		default:
			sep = "\n\n"
		}
		return content + sep + block, true, nil
	}
	out = content[:start] + block + content[end:]
	return out, out != content, nil
}

// Remove returns content without the block, and the blank line that Insert put
// before it. changed is false when there was no block.
func Remove(content string) (out string, changed bool, err error) {
	start, end, found, err := locate(content)
	if err != nil || !found {
		return content, false, err
	}
	before := content[:start]
	if strings.HasSuffix(before, "\n\n") {
		before = before[:len(before)-1]
	}
	return before + content[end:], true, nil
}

// locate finds the block: from the start of its first marker line to just after
// the newline of its last. found is false when neither marker is present.
func locate(content string) (start, end int, found bool, err error) {
	starts := strings.Count(content, BlockStart)
	ends := strings.Count(content, BlockEnd)
	if starts == 0 && ends == 0 {
		return 0, 0, false, nil
	}
	if starts != 1 || ends != 1 {
		return 0, 0, false, ErrBrokenBlock
	}
	start = strings.Index(content, BlockStart)
	if start > 0 && content[start-1] != '\n' {
		return 0, 0, false, ErrBrokenBlock
	}
	endMarker := strings.Index(content, BlockEnd)
	if endMarker < start {
		return 0, 0, false, ErrBrokenBlock
	}
	end = endMarker + len(BlockEnd)
	if i := strings.IndexByte(content[end:], '\n'); i >= 0 {
		end += i + 1
	} else {
		end = len(content)
	}
	return start, end, true, nil
}
