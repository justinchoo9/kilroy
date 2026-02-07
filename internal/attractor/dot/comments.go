package dot

import "fmt"

// stripComments removes // and /* */ comments from DOT source, while preserving comment-like
// sequences inside double-quoted strings.
func stripComments(src []byte) ([]byte, error) {
	out := make([]byte, 0, len(src))
	inString := false
	escaped := false

	for i := 0; i < len(src); {
		ch := src[i]
		if inString {
			out = append(out, ch)
			i++
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		// Not in string: detect comment starts.
		if ch == '"' {
			inString = true
			out = append(out, ch)
			i++
			continue
		}

		if ch == '/' && i+1 < len(src) {
			next := src[i+1]
			if next == '/' {
				// Line comment: skip until newline (but keep the newline).
				i += 2
				for i < len(src) && src[i] != '\n' {
					i++
				}
				continue
			}
			if next == '*' {
				// Block comment: skip until closing */.
				i += 2
				for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
					i++
				}
				if i+1 >= len(src) {
					return nil, fmt.Errorf("dot: unterminated block comment")
				}
				i += 2
				continue
			}
		}

		out = append(out, ch)
		i++
	}
	if inString {
		return nil, fmt.Errorf("dot: unterminated string (while stripping comments)")
	}
	return out, nil
}
