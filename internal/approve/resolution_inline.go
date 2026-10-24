package approve

import (
	"regexp"
	"strings"
)

var resolutionListItem = regexp.MustCompile(`^[ \t]*(?:[-+*]|[0-9]+[.)])(?:[ \t\r\n]|$)`)

// resolutionVisibleText keeps link labels and code spans.
// It hides complete link targets and raw HTML tags.
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
		if end := resolutionHTMLEnd(text, i); end > i {
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
			visible.WriteByte('[')
			i++
			continue
		}
		next := close + 1
		if references && next < len(text) && text[next] == ':' && strings.TrimSpace(visible.String()) == "" {
			if end := resolutionReferenceEnd(text, next+1); end >= 0 {
				i = end
				continue
			}
		}
		label := resolutionInlineText(text[i+1:close], false)
		if next < len(text) && text[next] == '(' {
			if end := resolutionTargetEnd(text, next); end >= 0 {
				visible.WriteString(label)
				i = end
			} else {
				visible.WriteByte('[')
				i++
			}
		} else if next < len(text) && text[next] == '[' {
			end := resolutionLabelEnd(text, next)
			if end < 0 {
				visible.WriteByte('[')
				i++
			} else {
				visible.WriteString(label)
				i = end + 1
			}
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
		if end := resolutionHTMLEnd(text, i); end > i {
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
	return -1
}

// resolutionReferenceEnd hides a complete definition and its optional title.
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
		return -1
	}
	if text[i] == '<' {
		i++
		for i < len(text) && text[i] != '>' {
			if text[i] == '\n' || text[i] == '<' {
				return -1
			}
			if text[i] == '\\' && i+1 < len(text) {
				i++
			}
			i++
		}
		if i == len(text) {
			return -1
		}
		i++
	} else {
		depth := 0
		for i < len(text) && !strings.ContainsRune(" \t\n", rune(text[i])) {
			switch text[i] {
			case '\\':
				if i+1 < len(text) {
					i++
				}
			case '(':
				depth++
			case ')':
				if depth == 0 {
					return -1
				}
				depth--
			case '<':
				return -1
			}
			i++
		}
		if depth != 0 {
			return -1
		}
	}
	end := lineEnd(i)
	for i < len(text) && strings.ContainsRune(" \t\n", rune(text[i])) {
		i++
	}
	// A title on a later line cannot invalidate the destination line.
	fallback := -1
	if i >= end || i == len(text) {
		fallback = end
	}
	if i >= len(text) || (text[i] != '\'' && text[i] != '"' && text[i] != '(') {
		return fallback
	}
	closer := text[i]
	if closer == '(' {
		closer = ')'
	}
	for j := i + 1; j < len(text); j++ {
		if text[j] == '\\' {
			j++
		} else if text[j] == closer {
			end := lineEnd(j)
			if strings.TrimSpace(text[j+1:end]) == "" {
				return end
			}
			return fallback
		}
	}
	return fallback
}

// Match complete raw inline HTML. Autolinks do not match these forms.
var resolutionHTML = regexp.MustCompile(`(?s)^(?:<[A-Za-z][A-Za-z0-9-]*(?:[ \t\r\n]+[A-Za-z_:][A-Za-z0-9_.:-]*(?:[ \t\r\n]*=[ \t\r\n]*(?:[^ \t\r\n"'=<>` + "`" + `]+|'[^']*'|"[^"]*"))?)*[ \t\r\n]*/?>|</[A-Za-z][A-Za-z0-9-]*[ \t\r\n]*>|<!--(?:[^-]|-[^-])*-->|<\?.*?\?>|<![A-Z]+[ \t\r\n]+[^>]*>|<!\[CDATA\[.*?\]\]>)`)

func resolutionHTMLEnd(text string, start int) int {
	if text[start] != '<' {
		return start
	}
	match := resolutionHTML.FindString(text[start:])
	if strings.HasPrefix(match, "<!--") {
		body := match[4 : len(match)-3]
		if strings.HasPrefix(body, ">") || strings.HasPrefix(body, "->") {
			return start
		}
	}
	return start + len(match)
}
