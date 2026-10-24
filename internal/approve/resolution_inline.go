package approve

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

var resolutionContainerMarker = regexp.MustCompile(`^(?:>[ \t]?|(?:[-+*]|[0-9]{1,9}[.)])(?:[ \t]+|$))`)
var resolutionHeading = regexp.MustCompile(`^ {0,3}#{1,6}(?:[ \t]|$)`)

// Keep block boundaries before parsing inline text. Read definitions from all sections.
func resolutionDocumentText(content string) string {
	inSection := false
	var all, section, block []string
	end, quoteDepth, listIndent := 0, 0, 0
	uncertain := false
	titleAt := -1
	flush := func() {
		if len(block) == 0 {
			return
		}
		text := strings.Join(block, "\n")
		if titleAt >= 0 {
			uncertain = uncertain || !resolutionDefinitionCovers(text, titleAt)
			titleAt = -1
		}
		if uncertain {
			block = nil
			uncertain = false
			return
		}
		all = append(all, text)
		if inSection {
			section = append(section, text)
		}
		block = nil
	}
	// A skipped source line can still continue the current paragraph.
	readGap := func(gap string) {
		for gap != "" {
			end := strings.IndexAny(gap, "\r\n")
			if end < 0 {
				end = len(gap)
			}
			raw := gap[:end]
			next := end
			if next < len(gap) {
				next++
				if gap[end] == '\r' && next < len(gap) && gap[next] == '\n' {
					next++
				}
			}
			gap = gap[next:]
			text, interrupts, unsure := resolutionContinuationLine(raw, quoteDepth, listIndent)
			paragraph := resolutionParagraphOpen(block)
			if !paragraph && len(block) > 0 && text != "" && strings.ContainsRune("\"'(", rune(text[0])) {
				// A definition can have an indented title on the next line.
				titleAt = len(strings.Join(block, "\n")) + 1
				paragraph = true
			}
			if strings.Trim(text, " \t") == "" || interrupts || !paragraph {
				flush()
				continue
			}
			uncertain = uncertain || unsure
			block = append(block, text)
		}
	}
	for _, line := range panel.MarkdownLines(content) {
		raw := content[line.Offset:line.End]
		depth, item, indent := resolutionContainers(raw, line.Text)
		readGap(content[end:line.Offset])
		if item {
			listIndent = indent
		} else if depth != quoteDepth || len(block) == 0 {
			listIndent = min(listIndent, indent)
		}
		separate := resolutionHeading.MatchString(line.Text) || resolutionThematicBreak(line.Text)
		if depth != quoteDepth || item || separate || line.Level > 0 {
			flush()
		}
		if line.Level > 0 && line.Level <= 2 {
			inSection = line.Level == 2 && line.Heading == "Resolutions"
			all = append(all, line.Text)
		} else if strings.Trim(line.Text, " \t\r") == "" {
			flush()
		} else {
			block = append(block, line.Text)
		}
		if separate || line.Level > 0 {
			flush()
		}
		end, quoteDepth = line.End, depth
	}
	readGap(content[end:])
	flush()
	references := make(map[string]bool)
	resolutionBlockBodies(all, references)
	return resolutionRenderBlocks(resolutionBlockBodies(section, references), references)
}

// Definitions do not open a paragraph. They can precede one in the same block.
func resolutionParagraphOpen(block []string) bool {
	text := strings.Join(block, "\n")
	for text != "" {
		end, _ := resolutionDefinition(text, 0)
		if end < 0 {
			return true
		}
		text = text[end:]
	}
	return false
}

// A restored title must be part of a complete definition.
func resolutionDefinitionCovers(text string, position int) bool {
	for i := 0; i < len(text); {
		end, _ := resolutionDefinition(text, i)
		if end < 0 {
			return false
		}
		if end > position {
			return true
		}
		i = end
	}
	return false
}

// Read omitted paragraph text without changing the shared block scanner.
func resolutionContinuationLine(raw string, quotes, listIndent int) (text string, interrupts, uncertain bool) {
	var expanded strings.Builder
	column := 0
	for _, r := range raw {
		if r == '\t' {
			width := 4 - column%4
			expanded.WriteString(strings.Repeat(" ", width))
			column += width
		} else {
			expanded.WriteRune(r)
			column++
		}
	}
	text = expanded.String()
	for n := 0; ; n++ {
		spaces := len(text) - len(strings.TrimLeft(text, " "))
		remove := min(spaces, listIndent)
		text = text[remove:]
		listIndent -= remove
		if n == quotes {
			break
		}
		trimmed := strings.TrimLeft(text, " ")
		if len(text)-len(trimmed) > 3 || !strings.HasPrefix(trimmed, ">") {
			break
		}
		text = strings.TrimPrefix(trimmed[1:], " ")
	}
	trimmed := strings.TrimLeft(text, " ")
	indent := len(text) - len(trimmed)
	if indent <= 3 {
		// Fences and HTML block types one through six interrupt paragraphs.
		if resolutionInterruptingHTML.MatchString(trimmed) {
			return "", true, false
		}
		if len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			width := 1
			for width < len(trimmed) && trimmed[width] == trimmed[0] {
				width++
			}
			if width >= 3 && (trimmed[0] != '`' || !strings.ContainsRune(trimmed[width:], '`')) {
				return "", true, false
			}
		}
		// The scanner can hide later lines of an uncertain container.
		// Discard that whole block if a new container or block cannot be placed.
		uncertain = resolutionContainerMarker.MatchString(trimmed) || resolutionHeading.MatchString(trimmed) || resolutionThematicBreak(trimmed)
	}
	return trimmed, false, uncertain
}

var resolutionInterruptingHTML = regexp.MustCompile(`(?i)^(?:<(?:script|pre|style|textarea)(?:[ \t>]|$)|<!--|<\?|<![A-Za-z]|<!\[CDATA\[|</?(?:address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h[1-6]|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|source|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)(?:[ \t]|/?>|$))`)

// The block scanner removes container markers from Text. Count only that prefix.
func resolutionContainers(raw, text string) (quotes int, item bool, indent int) {
	// Keep markers literal while the block scanner expands tabs and removes comments.
	// The four-column prefix preserves the source tab stops.
	raw = panel.MarkdownLines("text" + raw)[0].Text[4:]
	raw = strings.TrimRight(raw, " \t\r\n")
	text = strings.Trim(text, " \t\r\n")
	for strings.TrimLeft(raw, " \t") != text {
		trimmed := strings.TrimLeft(raw, " \t")
		indent += len(raw) - len(trimmed)
		raw = trimmed
		marker := resolutionContainerMarker.FindString(raw)
		if marker == "" {
			break
		}
		if marker[0] == '>' {
			quotes++
		} else {
			item = true
			indent += len(marker)
		}
		raw = raw[len(marker):]
	}
	indent += len(raw) - len(strings.TrimLeft(raw, " \t"))
	return quotes, item, indent
}

func resolutionThematicBreak(text string) bool {
	text = strings.ReplaceAll(strings.ReplaceAll(strings.Trim(text, " \t\r\n"), " ", ""), "\t", "")
	return len(text) >= 3 && strings.ContainsRune("-*_", rune(text[0])) && strings.Trim(text, text[:1]) == ""
}

// resolutionVisibleText keeps link labels and code spans.
// It hides complete link targets, images, definitions and raw HTML tags.
func resolutionVisibleText(text string) string {
	return resolutionBlocksText([]string{text})
}

// Collect definitions before scanning links. Definitions can follow their uses.
func resolutionBlocksText(blocks []string) string {
	references := make(map[string]bool)
	bodies := resolutionBlockBodies(blocks, references)
	return resolutionRenderBlocks(bodies, references)
}

func resolutionBlockBodies(blocks []string, references map[string]bool) []string {
	bodies := make([]string, 0, len(blocks))
	for _, text := range blocks {
		i := 0
		for i < len(text) {
			end, label := resolutionDefinition(text, i)
			if end < 0 {
				break
			}
			references[label] = true
			i = end
		}
		bodies = append(bodies, text[i:])
	}
	return bodies
}

func resolutionRenderBlocks(bodies []string, references map[string]bool) string {
	for i, text := range bodies {
		bodies[i] = resolutionInlineText(text, references)
	}
	return strings.Join(bodies, "\n")
}

func resolutionInlineText(text string, references map[string]bool) string {
	type opener struct {
		source, output int
		image, active  bool
		bottom         *resolutionDelimiter
	}
	var brackets []opener
	var visible []byte
	omit := make(map[int]bool)
	// The sentinel precedes every real output position.
	root := &resolutionDelimiter{output: -1}
	top := root
	processEmphasis := func(bottom *resolutionDelimiter) {
		resolutionProcessEmphasis(bottom, omit)
		top = bottom
	}
	deactivateLinks := func() {
		for n := range brackets {
			if !brackets[n].image {
				brackets[n].active = false
			}
		}
	}
	for i := 0; i < len(text); {
		if resolutionEscape(text, i) {
			visible = append(visible, text[i+1])
			i += 2
			continue
		}
		if end, body := resolutionCodeSpan(text, i); end > i {
			visible = append(visible, body...)
			i = end
			continue
		}
		if text[i] == '`' {
			// An unmatched run is literal in full. Do not start a span inside it.
			end := i + 1
			for end < len(text) && text[end] == '`' {
				end++
			}
			visible = append(visible, text[i:end]...)
			i = end
			continue
		}
		if end := resolutionHTMLEnd(text, i); end > i {
			i = end
			continue
		}
		if end := resolutionAutolinkEnd(text, i); end > i {
			visible = append(visible, text[i+1:end-1]...)
			i = end
			continue
		}
		if text[i] == '*' || text[i] == '_' {
			end := i + 1
			for end < len(text) && text[end] == text[i] {
				end++
			}
			open, close := resolutionDelimiterFlanks(text, i, end)
			delim := &resolutionDelimiter{output: len(visible), length: end - i, original: end - i, char: text[i], open: open, close: close, previous: top}
			top.next = delim
			top = delim
			visible = append(visible, text[i:end]...)
			i = end
			continue
		}
		image := text[i] == '!' && i+1 < len(text) && text[i+1] == '['
		if text[i] == '[' || image {
			brackets = append(brackets, opener{i, len(visible), image, true, top})
			if image {
				visible = append(visible, '!')
				i++
			}
			visible = append(visible, '[')
			i++
			continue
		}
		if text[i] != ']' || len(brackets) == 0 {
			visible = append(visible, text[i])
			i++
			continue
		}
		open := brackets[len(brackets)-1]
		brackets = brackets[:len(brackets)-1]
		next, end := i+1, -1
		labelStart := open.source + 1
		if open.image {
			labelStart++
		}
		if open.active {
			if next < len(text) && text[next] == '(' {
				end = resolutionLinkTarget(text, next+1, true)
			}
			if end < 0 {
				label := text[labelStart:i]
				labelEnd := resolutionReferenceLabelEnd(text, labelStart-1)
				validLabel := labelEnd == i
				if next < len(text) && text[next] == '[' {
					if refEnd := resolutionReferenceLabelEnd(text, next); refEnd >= 0 {
						if refEnd > next+1 {
							label = text[next+1 : refEnd]
							validLabel = true
						}
						if validLabel && references[resolutionNormalizeLabel(label)] {
							end = refEnd + 1
						}
					}
				} else if validLabel && references[resolutionNormalizeLabel(label)] {
					end = next
				}
			}
		}
		if end >= 0 {
			// A formed link bounds emphasis within its label.
			processEmphasis(open.bottom)
			if open.image {
				visible = visible[:open.output]
				for position := range omit {
					if position >= open.output {
						delete(omit, position)
					}
				}
			} else {
				// Keep output positions stable until all delimiters are processed.
				omit[open.output] = true
				deactivateLinks()
			}
			i = end
			continue
		}
		// Failed links keep their brackets. Scan the next byte as normal text.
		visible = append(visible, ']')
		i++
	}
	processEmphasis(root)
	var result strings.Builder
	for i, b := range visible {
		if !omit[i] {
			result.WriteByte(b)
		}
	}
	return result.String()
}

// Delimiters keep stable output positions. Removed markup leaves the text intact.
type resolutionDelimiter struct {
	previous, next           *resolutionDelimiter
	output, length, original int
	char                     byte
	open, close              bool
}

func resolutionDelimiterFlanks(text string, start, end int) (open, close bool) {
	before, after := rune(' '), rune(' ')
	if start > 0 {
		before, _ = utf8.DecodeLastRuneInString(text[:start])
	}
	if end < len(text) {
		after, _ = utf8.DecodeRuneInString(text[end:])
	}
	space := func(r rune) bool {
		return unicode.Is(unicode.Zs, r) || r == '\t' || r == '\n' || r == '\f' || r == '\r'
	}
	punct := func(r rune) bool {
		return (r >= '!' && r <= '/') || (r >= ':' && r <= '@') || (r >= '[' && r <= '`') || (r >= '{' && r <= '~') || unicode.IsPunct(r) || unicode.IsSymbol(r)
	}
	left := !space(after) && (!punct(after) || space(before) || punct(before))
	right := !space(before) && (!punct(before) || space(after) || punct(after))
	if text[start] == '_' {
		return left && (!right || punct(before)), right && (!left || punct(after))
	}
	return left, right
}

// Process closers in source order. A formed pair removes intervening delimiters.
func resolutionProcessEmphasis(bottom *resolutionDelimiter, omit map[int]bool) {
	var lower [2][2][3]int
	for typ := range lower {
		for open := range lower[typ] {
			for mod := range lower[typ][open] {
				lower[typ][open][mod] = -1
			}
		}
	}
	unlink := func(d *resolutionDelimiter) {
		d.previous.next = d.next
		if d.next != nil {
			d.next.previous = d.previous
		}
	}
	for closer := bottom.next; closer != nil; {
		if !closer.close {
			closer = closer.next
			continue
		}
		typ, canOpen := 0, 0
		if closer.char == '_' {
			typ = 1
		}
		if closer.open {
			canOpen = 1
		}
		bound := &lower[typ][canOpen][closer.original%3]
		var opener *resolutionDelimiter
		for candidate := closer.previous; candidate != bottom && candidate.output > *bound; candidate = candidate.previous {
			if candidate.char != closer.char || !candidate.open {
				continue
			}
			// A dual-purpose run obeys the rule of three.
			if (candidate.close || closer.open) && (candidate.original+closer.original)%3 == 0 && (candidate.original%3 != 0 || closer.original%3 != 0) {
				continue
			}
			opener = candidate
			break
		}
		if opener == nil {
			*bound = closer.previous.output
			next := closer.next
			if !closer.open {
				unlink(closer)
			}
			closer = next
			continue
		}
		use := 1
		if opener.length >= 2 && closer.length >= 2 {
			use = 2
		}
		for d := opener.next; d != closer; {
			next := d.next
			unlink(d)
			d = next
		}
		opener.length -= use
		for i := 0; i < use; i++ {
			omit[opener.output+opener.length+i] = true
			omit[closer.output+i] = true
		}
		closer.output += use
		closer.length -= use
		if opener.length == 0 {
			unlink(opener)
		}
		if closer.length == 0 {
			next := closer.next
			unlink(closer)
			closer = next
		}
	}
	bottom.next = nil
}

func resolutionEscape(text string, i int) bool {
	return text[i] == '\\' && i+1 < len(text) && strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", rune(text[i+1]))
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
			body := strings.ReplaceAll(text[start+width:i], "\n", " ")
			if strings.HasPrefix(body, " ") && strings.HasSuffix(body, " ") && strings.Trim(body, " ") != "" {
				body = body[1 : len(body)-1]
			}
			return end, body
		}
		i = end
	}
	return start, ""
}

// Parse one destination and optional title for both links and definitions.
// For inline links, start follows '('. For definitions, it follows ':'.
func resolutionLinkTarget(text string, start int, inline bool) int {
	i := resolutionSpace(text, start, true)
	if inline && i < len(text) && text[i] == ')' {
		return i + 1
	}
	if i == len(text) {
		return -1
	}
	if text[i] == '<' {
		i++
		for i < len(text) && text[i] != '>' {
			if text[i] == '\n' || text[i] == '\r' || text[i] == '<' {
				return -1
			}
			if resolutionEscape(text, i) {
				i++
			}
			i++
		}
		if i == len(text) {
			return -1
		}
		i++
	} else {
		dest, depth := i, 0
		for i < len(text) && text[i] > ' ' && text[i] != 0x7f {
			if resolutionEscape(text, i) {
				i += 2
				continue
			}
			if text[i] == '(' {
				depth++
			}
			if text[i] == ')' {
				if depth == 0 {
					break
				}
				depth--
			}
			i++
		}
		if i == dest || depth != 0 {
			return -1
		}
	}
	destEnd := i
	// A title on a later line cannot invalidate a valid definition line.
	fallback := -1
	if !inline {
		fallback = resolutionTargetFinish(text, destEnd, false)
	}
	if inline {
		if end := resolutionTargetFinish(text, i, true); end >= 0 {
			return end
		}
	}
	i = resolutionSpace(text, i, true)
	if i == destEnd || i == len(text) {
		return fallback
	}
	closer := text[i]
	if closer != '"' && closer != '\'' && closer != '(' {
		return fallback
	}
	if closer == '(' {
		closer = ')'
	}
	titleStart := i + 1
	for i = titleStart; i < len(text); i++ {
		if resolutionEscape(text, i) {
			i++
			continue
		}
		if closer == ')' && text[i] == '(' {
			return fallback
		}
		if text[i] == '\n' || text[i] == '\r' {
			next := resolutionSpace(text, i, true)
			if next < len(text) && (text[next] == '\n' || text[next] == '\r') {
				return fallback
			}
		}
		if text[i] == closer {
			if end := resolutionTargetFinish(text, i+1, inline); end >= 0 {
				return end
			}
			return fallback
		}
	}
	return fallback
}

// Whitespace around a destination or title permits at most one line ending.
func resolutionSpace(text string, i int, newline bool) int {
	for i < len(text) {
		if text[i] == ' ' || text[i] == '\t' {
			i++
			continue
		}
		if newline && (text[i] == '\n' || text[i] == '\r') {
			if text[i] == '\r' && i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
			i++
			newline = false
			continue
		}
		break
	}
	return i
}

func resolutionTargetFinish(text string, i int, inline bool) int {
	i = resolutionSpace(text, i, inline)
	if inline {
		if i < len(text) && text[i] == ')' {
			return i + 1
		}
		return -1
	}
	if i == len(text) {
		return i
	}
	if text[i] == '\n' || text[i] == '\r' {
		if text[i] == '\r' && i+1 < len(text) && text[i+1] == '\n' {
			i++
		}
		return i + 1
	}
	return -1
}

func resolutionDefinition(text string, start int) (int, string) {
	i := start
	for i < len(text) && text[i] == ' ' && i-start < 3 {
		i++
	}
	if i == len(text) || text[i] != '[' {
		return -1, ""
	}
	close := resolutionReferenceLabelEnd(text, i)
	if close < 0 || close+1 == len(text) || text[close+1] != ':' {
		return -1, ""
	}
	label := resolutionNormalizeLabel(text[i+1 : close])
	if label == "" {
		return -1, ""
	}
	return resolutionLinkTarget(text, close+2, false), label
}

// Reference labels contain at most 999 characters and no unescaped brackets.
func resolutionReferenceLabelEnd(text string, start int) int {
	for i, size := start+1, 0; i < len(text) && size <= 999; size++ {
		if text[i] == ']' {
			return i
		}
		if text[i] == '[' {
			return -1
		}
		if resolutionEscape(text, i) {
			i++
			size++
		}
		_, width := utf8.DecodeRuneInString(text[i:])
		i += width
	}
	return -1
}

func resolutionNormalizeLabel(label string) string {
	label = strings.Join(strings.FieldsFunc(label, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\r' || r == '\n'
	}), " ")
	var folded strings.Builder
	for _, r := range label {
		part := string(r)
		if expansion, ok := resolutionFoldExpansions[r]; ok {
			part = expansion
		}
		for _, c := range part {
			key := c
			for next := unicode.SimpleFold(c); next != c; next = unicode.SimpleFold(next) {
				if next < key {
					key = next
				}
			}
			folded.WriteRune(key)
		}
	}
	return folded.String()
}

// Full case folding also expands these Unicode characters.
var resolutionFoldExpansions = map[rune]string{
	'\U000000df': "\U00000073\U00000073",
	'\U00000130': "\U00000069\U00000307",
	'\U00000149': "\U000002bc\U0000006e",
	'\U000001f0': "\U0000006a\U0000030c",
	'\U00000390': "\U000003b9\U00000308\U00000301",
	'\U000003b0': "\U000003c5\U00000308\U00000301",
	'\U00000587': "\U00000565\U00000582",
	'\U00001e96': "\U00000068\U00000331",
	'\U00001e97': "\U00000074\U00000308",
	'\U00001e98': "\U00000077\U0000030a",
	'\U00001e99': "\U00000079\U0000030a",
	'\U00001e9a': "\U00000061\U000002be",
	'\U00001e9e': "\U00000073\U00000073",
	'\U00001f50': "\U000003c5\U00000313",
	'\U00001f52': "\U000003c5\U00000313\U00000300",
	'\U00001f54': "\U000003c5\U00000313\U00000301",
	'\U00001f56': "\U000003c5\U00000313\U00000342",
	'\U00001f80': "\U00001f00\U000003b9",
	'\U00001f81': "\U00001f01\U000003b9",
	'\U00001f82': "\U00001f02\U000003b9",
	'\U00001f83': "\U00001f03\U000003b9",
	'\U00001f84': "\U00001f04\U000003b9",
	'\U00001f85': "\U00001f05\U000003b9",
	'\U00001f86': "\U00001f06\U000003b9",
	'\U00001f87': "\U00001f07\U000003b9",
	'\U00001f88': "\U00001f00\U000003b9",
	'\U00001f89': "\U00001f01\U000003b9",
	'\U00001f8a': "\U00001f02\U000003b9",
	'\U00001f8b': "\U00001f03\U000003b9",
	'\U00001f8c': "\U00001f04\U000003b9",
	'\U00001f8d': "\U00001f05\U000003b9",
	'\U00001f8e': "\U00001f06\U000003b9",
	'\U00001f8f': "\U00001f07\U000003b9",
	'\U00001f90': "\U00001f20\U000003b9",
	'\U00001f91': "\U00001f21\U000003b9",
	'\U00001f92': "\U00001f22\U000003b9",
	'\U00001f93': "\U00001f23\U000003b9",
	'\U00001f94': "\U00001f24\U000003b9",
	'\U00001f95': "\U00001f25\U000003b9",
	'\U00001f96': "\U00001f26\U000003b9",
	'\U00001f97': "\U00001f27\U000003b9",
	'\U00001f98': "\U00001f20\U000003b9",
	'\U00001f99': "\U00001f21\U000003b9",
	'\U00001f9a': "\U00001f22\U000003b9",
	'\U00001f9b': "\U00001f23\U000003b9",
	'\U00001f9c': "\U00001f24\U000003b9",
	'\U00001f9d': "\U00001f25\U000003b9",
	'\U00001f9e': "\U00001f26\U000003b9",
	'\U00001f9f': "\U00001f27\U000003b9",
	'\U00001fa0': "\U00001f60\U000003b9",
	'\U00001fa1': "\U00001f61\U000003b9",
	'\U00001fa2': "\U00001f62\U000003b9",
	'\U00001fa3': "\U00001f63\U000003b9",
	'\U00001fa4': "\U00001f64\U000003b9",
	'\U00001fa5': "\U00001f65\U000003b9",
	'\U00001fa6': "\U00001f66\U000003b9",
	'\U00001fa7': "\U00001f67\U000003b9",
	'\U00001fa8': "\U00001f60\U000003b9",
	'\U00001fa9': "\U00001f61\U000003b9",
	'\U00001faa': "\U00001f62\U000003b9",
	'\U00001fab': "\U00001f63\U000003b9",
	'\U00001fac': "\U00001f64\U000003b9",
	'\U00001fad': "\U00001f65\U000003b9",
	'\U00001fae': "\U00001f66\U000003b9",
	'\U00001faf': "\U00001f67\U000003b9",
	'\U00001fb2': "\U00001f70\U000003b9",
	'\U00001fb3': "\U000003b1\U000003b9",
	'\U00001fb4': "\U000003ac\U000003b9",
	'\U00001fb6': "\U000003b1\U00000342",
	'\U00001fb7': "\U000003b1\U00000342\U000003b9",
	'\U00001fbc': "\U000003b1\U000003b9",
	'\U00001fc2': "\U00001f74\U000003b9",
	'\U00001fc3': "\U000003b7\U000003b9",
	'\U00001fc4': "\U000003ae\U000003b9",
	'\U00001fc6': "\U000003b7\U00000342",
	'\U00001fc7': "\U000003b7\U00000342\U000003b9",
	'\U00001fcc': "\U000003b7\U000003b9",
	'\U00001fd2': "\U000003b9\U00000308\U00000300",
	'\U00001fd3': "\U000003b9\U00000308\U00000301",
	'\U00001fd6': "\U000003b9\U00000342",
	'\U00001fd7': "\U000003b9\U00000308\U00000342",
	'\U00001fe2': "\U000003c5\U00000308\U00000300",
	'\U00001fe3': "\U000003c5\U00000308\U00000301",
	'\U00001fe4': "\U000003c1\U00000313",
	'\U00001fe6': "\U000003c5\U00000342",
	'\U00001fe7': "\U000003c5\U00000308\U00000342",
	'\U00001ff2': "\U00001f7c\U000003b9",
	'\U00001ff3': "\U000003c9\U000003b9",
	'\U00001ff4': "\U000003ce\U000003b9",
	'\U00001ff6': "\U000003c9\U00000342",
	'\U00001ff7': "\U000003c9\U00000342\U000003b9",
	'\U00001ffc': "\U000003c9\U000003b9",
	'\U0000fb00': "\U00000066\U00000066",
	'\U0000fb01': "\U00000066\U00000069",
	'\U0000fb02': "\U00000066\U0000006c",
	'\U0000fb03': "\U00000066\U00000066\U00000069",
	'\U0000fb04': "\U00000066\U00000066\U0000006c",
	'\U0000fb05': "\U00000073\U00000074",
	'\U0000fb06': "\U00000073\U00000074",
	'\U0000fb13': "\U00000574\U00000576",
	'\U0000fb14': "\U00000574\U00000565",
	'\U0000fb15': "\U00000574\U0000056b",
	'\U0000fb16': "\U0000057e\U00000576",
	'\U0000fb17': "\U00000574\U0000056d",
}

var resolutionAutolink = regexp.MustCompile(`^(?:<[A-Za-z][A-Za-z0-9+.-]{1,31}:[^<>\x00-\x20\x7f]*>|<[A-Za-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*>)`)

func resolutionAutolinkEnd(text string, start int) int {
	if text[start] != '<' {
		return start
	}
	return start + len(resolutionAutolink.FindString(text[start:]))
}

// Match complete raw inline HTML. Autolinks do not match these forms.
var resolutionHTML = regexp.MustCompile(`(?s)^(?:<[A-Za-z][A-Za-z0-9-]*(?:[ \t\r\n]+[A-Za-z_:][A-Za-z0-9_.:-]*(?:[ \t\r\n]*=[ \t\r\n]*(?:[^ \t\r\n"'=<>` + "`" + `]+|'[^']*'|"[^"]*"))?)*[ \t\r\n]*/?>|</[A-Za-z][A-Za-z0-9-]*[ \t\r\n]*>|<!--(?:[^-]|-[^-])*-->|<\?.*?\?>|<![A-Za-z][^>]*>|<!\[CDATA\[.*?\]\]>)`)

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
