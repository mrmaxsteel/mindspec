package panel

import (
	"regexp"
	"strings"
)

// MarkdownLine holds visible text and a section boundary, if present.
// Heading is empty for a conservative boundary used by gate checks.
// ATX headings have a title and level only at the document top level.
// Offset and End refer to the original input. End includes the newline.
type MarkdownLine struct {
	Offset, End int
	Text        string
	Level       int
	Heading     string
}

type container struct {
	quote  bool
	indent int
	marker byte
}

// MarkdownLines returns visible lines. Tabs advance to the next four-column stop.
// A container fence ends when its explicit container prefix ends.
func MarkdownLines(content string) []MarkdownLine {
	var result []MarkdownLine
	var containers []container
	var fenceByte byte
	fenceLength := 0
	inComment, inHTML := false, false
	paragraph := false
	uncertainContainer := false
	paragraphStart := 0
	htmlEnd := ""
	frontmatter := strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r")
	for offset := 0; offset < len(content); {
		end := strings.IndexAny(content[offset:], "\r\n")
		if end < 0 {
			end = len(content)
		} else {
			end += offset
		}
		next := end
		if next < len(content) {
			next++
			if content[end] == '\r' && next < len(content) && content[next] == '\n' {
				next++
			}
		}
		entry := MarkdownLine{Offset: offset, End: next}
		line := markdownExpandTabs(content[offset:end])
		offset = next
		if frontmatter {
			if entry.Offset > 0 && (line == "---" || line == "...") {
				frontmatter = false
			}
			continue
		}
		text, matched := markdownContinueContainers(line, containers)
		if fenceLength > 0 {
			// New list or quote markers inside code cannot close its fence.
			if matched == len(containers) {
				char, length, rest := markdownFence(text)
				if char == fenceByte && length >= fenceLength && strings.Trim(rest, " \t\r") == "" {
					fenceLength = 0
				}
				continue
			}
			fenceLength = 0
			paragraph = false
			containers = containers[:matched]
		}
		if inHTML {
			if (htmlEnd != "" && strings.Contains(strings.ToLower(line), htmlEnd)) || (htmlEnd == "" && strings.Trim(line, " ") == "") {
				inHTML = false
			}
			continue
		}
		if !inComment && strings.Trim(line, " ") == "" {
			paragraph = false
			continue
		}
		if !inComment && (markdownIndent(text) >= 4 || (uncertainContainer && matched == len(containers) && len(containers) > 0)) {
			continue
		}
		hadComment := inComment || markdownCommentStart(line) >= 0
		if !inComment {
			if matched < len(containers) {
				level, _ := markdownHeading(text)
				char, length, rest := markdownFence(text)
				opensFence := length >= 3 && (char != '`' || !strings.ContainsRune(rest, '`'))
				sibling := !containers[matched].quote && markdownListIndent(text, false) > 0 && containers[matched].marker == markdownListMarker(text)
				if paragraph && !sibling && level == 0 && !opensFence && !markdownThematicBreak(text) && markdownListIndent(text, true) == 0 {
					// Hide this uncertain container. Keep later sections and
					// paragraphs when the container ends.
					uncertainContainer = true
					continue
				}
				paragraph = false
				uncertainContainer = false
			}
			containers = containers[:matched]
			for markdownIndent(text) <= 3 {
				if markdownThematicBreak(text) {
					break
				}
				trimmed := strings.TrimLeft(text, " ")
				if strings.HasPrefix(trimmed, ">") {
					text = strings.TrimPrefix(trimmed, ">")
					text = strings.TrimPrefix(text, " ")
					containers = append(containers, container{quote: true})
					paragraph = false
					continue
				}
				width := markdownListIndent(text, paragraph)
				if width < 0 {
					// An empty item has one space of content indent.
					containers = append(containers, container{indent: len(strings.TrimRight(text, " ")) + 1, marker: markdownListMarker(text)})
					text = ""
					paragraph = false
					break
				}
				if width == 0 {
					break
				}
				containers = append(containers, container{indent: width, marker: markdownListMarker(text)})
				text = text[width:]
				paragraph = false
			}
		}
		if markdownIndent(text) >= 4 {
			continue
		}
		if hadComment {
			// Parse only container markers that precede the comment.
			text = markdownCommentText(text, &inComment)
		}
		char, length, rest := markdownFence(text)
		if length >= 3 && (char != '`' || !strings.ContainsRune(rest, '`')) {
			fenceByte, fenceLength = char, length
			paragraph = false
			continue
		}
		// Raw HTML can hide text or headings. Keep the whole block out.
		if end, starts := markdownHTMLBlockStart(text, paragraph); starts {
			htmlEnd = end
			inHTML = htmlEnd == "" || !strings.Contains(strings.ToLower(text), htmlEnd)
			paragraph = false
			continue
		}
		entry.Text = text
		wasParagraph := paragraph
		level, _ := markdownHeading(text)
		paragraph = level == 0 && strings.Trim(text, " ") != "" && !markdownThematicBreak(text)
		if paragraph && !wasParagraph {
			paragraphStart = len(result)
		}
		if len(containers) == 0 && !hadComment {
			entry.Level, entry.Heading = markdownHeading(text)
			// A setext underline can end a section but cannot open one here.
			trimmed := strings.Trim(text, " ")
			if wasParagraph && trimmed != "" && (strings.Trim(trimmed, "=") == "" || strings.Trim(trimmed, "-") == "") {
				entry.Level = 2
				result = result[:paragraphStart]
				paragraph = false
			}
		}
		result = append(result, entry)
	}
	return result
}

func markdownContinueContainers(line string, containers []container) (string, int) {
	for i, c := range containers {
		if c.quote {
			n := markdownIndent(line)
			if n > 3 || n == len(line) || line[n] != '>' {
				return line, i
			}
			line = strings.TrimPrefix(line[n+1:], " ")
		} else {
			if strings.Trim(line, " ") == "" {
				line = ""
				continue
			}
			if markdownIndent(line) < c.indent {
				return line, i
			}
			line = line[c.indent:]
		}
	}
	return line, len(containers)
}

func markdownIndent(line string) int { return len(line) - len(strings.TrimLeft(line, " ")) }

// These patterns match block openers after the container prefix.
var markdownRawHTML = regexp.MustCompile(`(?i)^<(script|pre|style|textarea)([ >]|$)`)
var markdownBlockTag = regexp.MustCompile(`(?i)^</?(address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h[1-6]|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|source|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)( |/?>|$)`)
var markdownCompleteTag = regexp.MustCompile(`(?i)^(?:</[a-z][a-z0-9-]* *> *|<[a-z][a-z0-9-]*(?: +[a-z_:][a-z0-9_.:-]*(?: *= *(?:[^ "'=<>` + "`" + `]+|'[^']*'|"[^"]*"))?)* */?> *)$`)
var markdownDeclaration = regexp.MustCompile(`^<![A-Z]`)

func markdownHTMLBlockStart(line string, paragraph bool) (string, bool) {
	if markdownIndent(line) > 3 {
		return "", false
	}
	text := strings.TrimLeft(line, " ")
	if match := markdownRawHTML.FindStringSubmatch(text); match != nil {
		return "</" + strings.ToLower(match[1]) + ">", true
	}
	for _, pair := range [][2]string{{"<!--", "-->"}, {"<?", "?>"}, {"<![CDATA[", "]]>"}} {
		if strings.HasPrefix(text, pair[0]) {
			return pair[1], true
		}
	}
	if markdownDeclaration.MatchString(text) {
		return ">", true
	}
	return "", markdownBlockTag.MatchString(text) || (!paragraph && markdownCompleteTag.MatchString(text))
}

func markdownThematicBreak(line string) bool {
	text := strings.ReplaceAll(strings.Trim(line, " "), " ", "")
	return len(text) >= 3 && strings.ContainsRune("-*_", rune(text[0])) && strings.Trim(text, text[:1]) == ""
}

func markdownListMarker(line string) byte {
	text := strings.TrimLeft(line, " ")
	i := 0
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	return text[i]
}

func markdownListIndent(line string, paragraph bool) int {
	n := markdownIndent(line)
	if n > 3 || n == len(line) {
		return 0
	}
	i := n
	if strings.ContainsRune("-+*", rune(line[i])) {
		i++
	} else {
		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			i++
		}
		if i == n || i-n > 9 || i == len(line) || (line[i] != '.' && line[i] != ')') {
			return 0
		}
		// Only an ordered list that starts at one can interrupt a paragraph.
		if paragraph && strings.TrimLeft(line[n:i], "0") != "1" {
			return 0
		}
		i++
	}
	if i == len(line) {
		if paragraph {
			return 0
		}
		return -1
	}
	if line[i] != ' ' {
		return 0
	}
	if strings.Trim(line[i:], " ") == "" {
		if paragraph {
			return 0
		}
		return -1
	}
	spaces := markdownIndent(line[i:])
	if spaces > 4 {
		spaces = 1
	}
	return i + spaces
}

func markdownFence(line string) (char byte, length int, rest string) {
	n := markdownIndent(line)
	if n > 3 || n == len(line) || (line[n] != '`' && line[n] != '~') {
		return 0, 0, ""
	}
	line = line[n:]
	char = line[0]
	for length < len(line) && line[length] == char {
		length++
	}
	return char, length, line[length:]
}

func markdownHeading(line string) (int, string) {
	n := markdownIndent(line)
	if n > 3 {
		return 0, ""
	}
	line = line[n:]
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || (level < len(line) && line[level] != ' ') {
		return 0, ""
	}
	title := strings.Trim(line[level:], " ")
	withoutClose := strings.TrimRight(title, "#")
	if withoutClose == "" || strings.HasSuffix(withoutClose, " ") {
		title = strings.Trim(withoutClose, " ")
	}
	return level, title
}

func markdownExpandTabs(line string) string {
	var out strings.Builder
	column := 0
	for _, r := range line {
		if r == '\t' {
			width := 4 - column%4
			out.WriteString(strings.Repeat(" ", width))
			column += width
		} else {
			out.WriteRune(r)
			column++
		}
	}
	return out.String()
}

func markdownCommentText(line string, inComment *bool) string {
	var visible strings.Builder
	for line != "" {
		if *inComment {
			end := strings.Index(line, "-->")
			if end < 0 {
				break
			}
			line = line[end+3:]
			*inComment = false
			visible.WriteByte(' ')
			continue
		}
		start := markdownCommentStart(line)
		if start < 0 {
			visible.WriteString(line)
			break
		}
		visible.WriteString(line[:start])
		visible.WriteByte(' ')
		line = line[start+4:]
		*inComment = true
	}
	return visible.String()
}

// markdownCommentStart skips code spans and escaped openers.
func markdownCommentStart(line string) int {
	escaped := func(pos int) bool {
		count := 0
		for pos > 0 && line[pos-1] == '\\' {
			count++
			pos--
		}
		return count%2 == 1
	}
	for i := 0; i < len(line); {
		if line[i] == '`' && !escaped(i) {
			run := 1
			for i+run < len(line) && line[i+run] == '`' {
				run++
			}
			next := i + run
			for next < len(line) {
				if line[next] != '`' {
					next++
					continue
				}
				end := next + 1
				for end < len(line) && line[end] == '`' {
					end++
				}
				if end-next == run {
					i = end
					break
				}
				next = end
			}
			if next < len(line) {
				continue
			}
			i += run
			continue
		}
		if strings.HasPrefix(line[i:], "<!--") && !escaped(i) {
			return i
		}
		i++
	}
	return -1
}
