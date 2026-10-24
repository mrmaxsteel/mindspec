package approve

import (
	"regexp"
	"strings"
)

var resolutionListItem = regexp.MustCompile(`^[ \t]*(?:[-+*]|[0-9]+[.)])(?:[ \t\r\n]|$)`)

// resolutionVisibleText keeps link labels and code spans. It hides link targets.
// The caller limits each input to one paragraph or list item.
func resolutionVisibleText(text string) string {
	return resolutionInlineText(text, true)
}

func resolutionInlineText(text string, references bool) string {
	var visible strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '\\' && i+1 < len(text) {
			visible.WriteByte(text[i+1])
			i += 2
			continue
		}
		if end, body := resolutionCodeSpan(text, i); end > i {
			visible.WriteString(body)
			i = end
			continue
		}
		if text[i] != '[' {
			visible.WriteByte(text[i])
			i++
			continue
		}
		close := resolutionLabelEnd(text, i)
		if close < 0 {
			// Keep uncertainty on this line.
			next := strings.IndexByte(text[i:], '\n')
			if next < 0 {
				break
			}
			i += next + 1
			visible.WriteByte('\n')
			continue
		}
		next := close + 1
		if references && next < len(text) && text[next] == ':' && strings.TrimSpace(visible.String()) == "" {
			i = resolutionReferenceEnd(text, next+1)
			continue
		}
		label := resolutionInlineText(text[i+1:close], false)
		if next < len(text) && text[next] == '(' {
			visible.WriteString(label)
			i = resolutionTargetEnd(text, next)
		} else if next < len(text) && text[next] == '[' {
			visible.WriteString(label)
			end := resolutionLabelEnd(text, next)
			if end < 0 {
				return visible.String()
			}
			i = end + 1
		} else {
			visible.WriteByte('[')
			visible.WriteString(label)
			visible.WriteByte(']')
			i = next
		}
	}
	return visible.String()
}

// resolutionCodeSpan reads a span before brackets are treated as markup.
func resolutionCodeSpan(text string, start int) (int, string) {
	if text[start] != '`' {
		return start, ""
	}
	width := 1
	for start+width < len(text) && text[start+width] == '`' {
		width++
	}
	for i := start + width; i < len(text); {
		if text[i] != '`' {
			i++
			continue
		}
		end := i + 1
		for end < len(text) && text[end] == '`' {
			end++
		}
		if end-i == width {
			return end, strings.ReplaceAll(text[start+width:i], "\n", " ")
		}
		i = end
	}
	return start, ""
}

func resolutionLabelEnd(text string, start int) int {
	depth := 1
	for i := start + 1; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		if end, _ := resolutionCodeSpan(text, i); end > i {
			i = end - 1
			continue
		}
		if text[i] == '[' {
			depth++
		} else if text[i] == ']' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func resolutionTargetEnd(text string, start int) int {
	depth := 1
	var quote byte
	for i := start + 1; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		if quote != 0 {
			if text[i] == quote {
				quote = 0
			}
			continue
		}
		switch text[i] {
		case '<':
			for i++; i < len(text) && text[i] != '>'; i++ {
				if text[i] == '\\' && i+1 < len(text) {
					i++
				}
			}
		case '\'', '"':
			if i > start+1 && strings.ContainsRune(" \t\n", rune(text[i-1])) {
				quote = text[i]
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	// An incomplete target can hide only the current paragraph or item.
	return len(text)
}

// resolutionReferenceEnd hides a definition and its optional title.
func resolutionReferenceEnd(text string, start int) int {
	lineEnd := func(i int) int {
		if n := strings.IndexByte(text[i:], '\n'); n >= 0 {
			return i + n + 1
		}
		return len(text)
	}
	i := start
	for i < len(text) && strings.ContainsRune(" \t\n", rune(text[i])) {
		i++
	}
	if i == len(text) {
		return i
	}
	if text[i] == '<' {
		i++
		for i < len(text) && text[i] != '>' {
			if text[i] == '\\' && i+1 < len(text) {
				i++
			}
			i++
		}
		if i == len(text) {
			return i
		}
		i++
	} else {
		for i < len(text) && !strings.ContainsRune(" \t\n", rune(text[i])) {
			if text[i] == '\\' && i+1 < len(text) {
				i++
			}
			i++
		}
	}
	end := lineEnd(i)
	for i < len(text) && strings.ContainsRune(" \t\n", rune(text[i])) {
		i++
	}
	if i >= len(text) || (text[i] != '\'' && text[i] != '"' && text[i] != '(') {
		return end
	}
	closer := text[i]
	if closer == '(' {
		closer = ')'
	}
	for j := i + 1; j < len(text); j++ {
		if text[j] == '\\' {
			j++
		} else if text[j] == closer {
			return lineEnd(j)
		}
	}
	// Keep an incomplete title within this paragraph or item.
	return len(text)
}
