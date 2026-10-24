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

// Parse the whole document so references resolve across section boundaries.
func resolutionDocumentText(content string) string {
	source := []byte(content)
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
		case *ast.Image, *ast.RawHTML, *ast.HTMLBlock, *ast.CodeBlock, *ast.FencedCodeBlock:
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
