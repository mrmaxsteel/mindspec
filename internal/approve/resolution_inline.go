package approve

import (
	"bufio"
	"bytes"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	goldhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// Use CommonMark 0.31.2 section 4.6 condition-6 and condition-1 names, plus replaced and form boxes.
var resolutionSeparatingHTMLTags = map[string]bool{
	"address": true, "article": true, "aside": true, "base": true, "basefont": true,
	"blockquote": true, "body": true, "caption": true, "center": true, "col": true,
	"colgroup": true, "dd": true, "details": true, "dialog": true, "dir": true,
	"div": true, "dl": true, "dt": true, "fieldset": true, "figcaption": true,
	"figure": true, "footer": true, "form": true, "frame": true, "frameset": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"head": true, "header": true, "hr": true, "html": true, "iframe": true,
	"legend": true, "li": true, "link": true, "main": true, "menu": true,
	"menuitem": true, "nav": true, "noframes": true, "ol": true, "optgroup": true,
	"option": true, "p": true, "param": true, "search": true, "section": true,
	"summary": true, "table": true, "tbody": true, "td": true, "tfoot": true,
	"th": true, "thead": true, "title": true, "tr": true, "track": true, "ul": true,
	"pre": true, "script": true, "style": true, "textarea": true,
	"br": true, "img": true, "input": true, "button": true, "select": true,
	"embed": true, "object": true, "video": true, "audio": true, "canvas": true,
	"svg": true, "math": true,
}

// Parse the whole body so references resolve across section boundaries.
func resolutionDocumentText(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	source := []byte(resolutionMarkdownBody(content))
	document := goldmark.New().Parser().Parse(text.NewReader(source))
	var section strings.Builder
	inSection := false
	for block := document.FirstChild(); block != nil; block = block.NextSibling() {
		if heading, ok := block.(*ast.Heading); ok && heading.Level <= 2 {
			inSection = heading.Level == 2 && strings.TrimSpace(resolutionNodeText(heading, source)) == "Resolutions"
			continue
		}
		if inSection {
			section.WriteString(resolutionNodeText(block, source))
		}
	}
	return section.String()
}

// Use the frontmatter delimiters from panel.MarkdownLines. An unclosed block
// has no visible body. Keep the body bytes for reference resolution.
func resolutionMarkdownBody(content string) string {
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r") {
		return content
	}
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
		if offset > 0 && (content[offset:end] == "---" || content[offset:end] == "...") {
			return content[next:]
		}
		offset = next
	}
	return ""
}

// Use the renderer's text rules. Omit hidden nodes before decoding HTML escapes.
func resolutionNodeText(root ast.Node, source []byte) string {
	var escaped bytes.Buffer
	writer := bufio.NewWriter(&escaped)
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			if node.Type() == ast.TypeBlock {
				_ = writer.WriteByte('\n')
			}
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.Image:
			_ = writer.WriteByte(' ')
			return ast.WalkSkipChildren, nil
		case *ast.RawHTML:
			raw := bytes.TrimPrefix(n.Segments.Value(source), []byte("<"))
			raw = bytes.TrimPrefix(raw, []byte("/"))
			end := bytes.IndexAny(raw, " \t\r\n\f/>")
			if end >= 0 && resolutionSeparatingHTMLTags[strings.ToLower(string(raw[:end]))] {
				_ = writer.WriteByte(' ')
			}
			return ast.WalkSkipChildren, nil
		case *ast.HTMLBlock, *ast.CodeBlock, *ast.FencedCodeBlock:
			return ast.WalkSkipChildren, nil
		case *ast.CodeSpan:
			for child := n.FirstChild(); child != nil; child = child.NextSibling() {
				value := child.(*ast.Text).Segment.Value(source)
				if bytes.HasSuffix(value, []byte("\n")) {
					goldhtml.DefaultWriter.RawWrite(writer, value[:len(value)-1])
					_ = writer.WriteByte(' ')
				} else {
					goldhtml.DefaultWriter.RawWrite(writer, value)
				}
			}
			return ast.WalkSkipChildren, nil
		case *ast.AutoLink:
			goldhtml.DefaultWriter.RawWrite(writer, n.Label(source))
		case *ast.Text:
			if n.IsRaw() {
				goldhtml.DefaultWriter.RawWrite(writer, n.Segment.Value(source))
			} else {
				goldhtml.DefaultWriter.Write(writer, n.Segment.Value(source))
			}
			if n.SoftLineBreak() || n.HardLineBreak() {
				_ = writer.WriteByte('\n')
			}
		case *ast.String:
			if n.IsCode() || n.IsRaw() {
				goldhtml.DefaultWriter.RawWrite(writer, n.Value)
			} else {
				goldhtml.DefaultWriter.Write(writer, n.Value)
			}
		}
		return ast.WalkContinue, nil
	})
	_ = writer.Flush()
	return html.UnescapeString(escaped.String())
}
