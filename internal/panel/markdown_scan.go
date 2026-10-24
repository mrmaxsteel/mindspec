package panel

import "strings"

// MarkdownLine holds visible text and a section boundary, if present.
// Heading is empty for a conservative boundary. ATX headings have a title
// and level only at the document top level.
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
		if !inComment && markdownIndent(text) >= 4 {
			continue
		}
		hadComment := inComment || strings.Contains(line, "<!--")
		if !inComment {
			if matched < len(containers) {
				level, _ := markdownHeading(text)
				char, length, rest := markdownFence(text)
				opensFence := length >= 3 && (char != '`' || !strings.ContainsRune(rest, '`'))
				sibling := !containers[matched].quote && markdownListIndent(text, false) > 0 && containers[matched].marker == markdownListMarker(text)
				if paragraph && !sibling && level == 0 && !opensFence && !markdownThematicBreak(text) && markdownListIndent(text, true) == 0 {
					// A lazy continuation has no explicit container prefix.
					// End the accepted section instead of guessing its scope.
					entry.Level = 2
					result = append(result, entry)
					paragraph = false
					continue
				}
				paragraph = false
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
					// An empty item can also be a setext underline. Refuse
					// this document instead of guessing its content indent.
					return nil
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
		if strings.Contains(text, "<") {
			htmlEnd = markdownHtmlBlockEnd(text)
			inHTML = htmlEnd == "" || !strings.Contains(strings.ToLower(text), htmlEnd)
			paragraph = false
			continue
		}
		entry.Text = text
		level, _ := markdownHeading(text)
		paragraph = level == 0 && strings.Trim(text, " ") != "" && !markdownThematicBreak(text)
		if len(containers) == 0 && !hadComment {
			entry.Level, entry.Heading = markdownHeading(text)
			// A setext underline can end a section but cannot open one here.
			trimmed := strings.Trim(text, " ")
			if trimmed != "" && (strings.Trim(trimmed, "=") == "" || strings.Trim(trimmed, "-") == "") {
				entry.Level = 2
				if len(result) > 0 {
					result = result[:len(result)-1]
				}
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
			if markdownIndent(line) < c.indent {
				return line, i
			}
			line = line[c.indent:]
		}
	}
	return line, len(containers)
}

func markdownIndent(line string) int { return len(line) - len(strings.TrimLeft(line, " ")) }

func markdownHtmlBlockEnd(line string) string {
	lower := strings.ToLower(line)
	for _, tag := range []string{"script", "pre", "style", "textarea"} {
		if strings.Contains(lower, "<"+tag) {
			return "</" + tag + ">"
		}
	}
	for _, delimiters := range [][2]string{{"<?", "?>"}, {"<![cdata[", "]]>"}, {"<!", ">"}} {
		if strings.Contains(lower, delimiters[0]) {
			return delimiters[1]
		}
	}
	return ""
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
		return -1
	}
	if line[i] != ' ' {
		return 0
	}
	if strings.Trim(line[i:], " ") == "" {
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
		start := strings.Index(line, "<!--")
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
