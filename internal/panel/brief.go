package panel

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// BriefRegions contains values resolved by the command layer. SpecACs is the
// reviewed subject; it is not binary-authored guidance.
type BriefRegions struct {
	Question    string
	NotBlocking []string
	SpecACs     string
	Conduct     []BriefRule
}

type BriefRule struct {
	ID          string
	Prohibition string
	Replacement string
}

// AcceptanceCriteriaSection returns the complete level-two section, including
// its heading. Headings inside code fences do not delimit the section.
func AcceptanceCriteriaSection(spec string) string {
	start, end := -1, len(spec)
	scanMarkdownLines(spec, func(offset int, line string) {
		if start < 0 {
			if strings.TrimSpace(line) == "## Acceptance Criteria" {
				start = offset
			}
			return
		}
		if end == len(spec) && (strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ")) {
			end = offset
		}
	})
	if start < 0 {
		return "no acceptance criteria section found"
	}
	return spec[start:end]
}

func (r BriefRegions) authored() string {
	var b strings.Builder
	b.WriteString("<!-- mindspec:acceptance -->\n## Acceptance\n\n")
	b.WriteString(termsafe.Escape(r.Question))
	b.WriteString("\n\nNot blocking:\n")
	for _, item := range r.NotBlocking {
		fmt.Fprintf(&b, "- %s\n", termsafe.Escape(item))
	}
	b.WriteString("<!-- /mindspec:acceptance -->\n\n")
	b.WriteString("<!-- mindspec:conduct -->\n## Conduct\n\n")
	for _, rule := range r.Conduct {
		fmt.Fprintf(&b, "%s: %s — %s\n", termsafe.Escape(rule.ID), termsafe.Escape(rule.Prohibition), termsafe.Escape(rule.Replacement))
	}
	b.WriteString("<!-- /mindspec:conduct -->\n\n")
	b.WriteString(verdictContract)
	return b.String()
}

const verdictContract = `<!-- mindspec:verdict-contract -->
## Your job

Review the change at the reviewed commit. Return one JSON object.
Use mindspec panel schema for the JSON Schema.
Use mindspec panel submit or the output channel in the slot prompt.

- Required: schema (2), slot, round, reviewed_sha (40 lowercase hex digits), status (draft or final), verdict, model, rationale, concrete_changes_required, commands_run.
- verdict: APPROVE, REQUEST_CHANGES, REJECT or VOID.
- rationale: non-empty unless verdict is VOID.
- concrete_changes_required: an array of at most 100 items with non-empty text and basis (executed, traced or hypothesis).
- An executed item has command: the zero-based index of its commands_run entry. Other items have no command.
- REQUEST_CHANGES needs at least one item. Other verdicts have no items.
- commands_run: an array of at most 200 entries with cmd, exit (an integer) and tail (at most 4096 UTF-8 bytes).
- void_reason: required for VOID and refused for other verdicts.
- Optional: hard_block (boolean), findings and inputs_read (string arrays).
- Unknown fields are refused at every level. The verdict is at most 1 MiB.
<!-- /mindspec:verdict-contract -->
`

// renderBriefRegions replaces the former output contract within the header.
// It leaves registration and the authored body outside the header intact.
func renderBriefRegions(header string, r BriefRegions) string {
	registration, _, _ := strings.Cut(header, "\n## Your job")
	var b strings.Builder
	b.WriteString(registration)
	b.WriteString("\n")
	b.WriteString(r.authored())
	b.WriteString("\n<!-- mindspec:spec-acs -->\n## Quoted acceptance criteria\n\n")
	quote := escapeBriefText(r.SpecACs)
	if quote == "" {
		quote = "no acceptance criteria section found"
	}
	// A longer fence keeps code and marker examples inside the quotation.
	fence := "```"
	for strings.Contains(quote, fence) {
		fence += "`"
	}
	b.WriteString(fence + "text\n" + quote)
	if !strings.HasSuffix(quote, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString(fence + "\n<!-- /mindspec:spec-acs -->\n")
	b.WriteString(briefHeaderClose)
	return b.String()
}

// escapeBriefText keeps printable text and line breaks unchanged.
// It escapes control runes and invalid UTF-8 bytes without quoting the line.
func escapeBriefText(text string) string {
	var b strings.Builder
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, "\\x%02x", text[0])
		case r == '\n' || strconv.IsPrint(r):
			b.WriteString(text[:size])
		case r <= 0xff:
			fmt.Fprintf(&b, "\\x%02x", r)
		case r <= 0xffff:
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			fmt.Fprintf(&b, "\\U%08x", r)
		}
		text = text[size:]
	}
	return b.String()
}

var briefInternalReference = regexp.MustCompile(`ADR-[0-9]+|[Ss]pec [0-9]{3}|\bR[0-9]+\.[0-9]+|#[0-9]{3}|mindspec-[a-z0-9]+(?:\.[0-9]+)?|§`)
var briefPath = regexp.MustCompile(`\.[a-z][a-z0-9]*$`)

// briefRepositoryPaths selects tokens inside backtick code spans. A repository
// path has no space and contains a slash. Without a slash, its last dot part
// must start with a lowercase letter and contain only lowercase letters and
// digits. A command that starts with "mindspec " is not a path.
// Bare words outside code spans are not paths.
func briefRepositoryPaths(text string) []string {
	var paths []string
	for len(text) > 0 {
		start := strings.IndexByte(text, '`')
		if start < 0 {
			break
		}
		text = text[start:]
		n := len(text) - len(strings.TrimLeft(text, "`"))
		text = text[n:]
		for cursor := 0; ; {
			relative := strings.IndexByte(text[cursor:], '`')
			if relative < 0 {
				// An unmatched opening run is literal text. Scan later runs.
				break
			}
			end := cursor + relative
			length := len(text[end:]) - len(strings.TrimLeft(text[end:], "`"))
			if length == n {
				token := text[:end]
				if !strings.HasPrefix(token, "mindspec ") && strings.IndexFunc(token, unicode.IsSpace) < 0 &&
					(strings.Contains(token, "/") || briefPath.MatchString(token)) {
					paths = append(paths, token)
				}
				text = text[end+length:]
				break
			}
			cursor = end + length
		}
	}
	return paths
}

// ValidateBriefRegions checks only binary-authored guidance. The callback reads
// paths at the reviewed commit through the executor. Generated panel artifacts
// are output names, not repository input paths.
func ValidateBriefRegions(r BriefRegions, fileAtRef func(string) ([]byte, bool, error)) error {
	authored := r.authored()
	if briefInternalReference.MatchString(authored) {
		return submitRefusal("review guidance contains an internal reference", "remove the internal reference from the gate acceptance or conduct text")
	}
	seen := map[string]bool{}
	for _, path := range briefRepositoryPaths(authored) {
		if path == "panel.json" || seen[path] {
			continue
		}
		seen[path] = true
		if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".rst") || strings.HasSuffix(path, ".adoc") || strings.Contains(path, "/domains/") {
			return submitRefusal("review guidance names a documentation path: "+path, "state the review rule without a documentation path")
		}
		_, present, err := fileAtRef(path)
		if err != nil {
			return submitRefusal("cannot check review guidance path "+path+": "+err.Error(), "check the reviewed commit and run panel create again")
		}
		if !present {
			return submitRefusal("review guidance path is absent at the reviewed commit: "+path, "remove the path from the review guidance or select a commit that contains it")
		}
	}
	return nil
}

// scanMarkdownLines visits lines outside backtick and tilde fences. A nested
// shorter fence cannot close the fence used to quote an acceptance section.
func scanMarkdownLines(s string, fn func(int, string)) {
	var fenceByte byte
	fenceLength := 0
	for offset := 0; offset < len(s); {
		end := strings.IndexByte(s[offset:], '\n')
		if end < 0 {
			end = len(s)
		} else {
			end += offset
		}
		line := strings.TrimSuffix(s[offset:end], "\r")
		trimmed := strings.TrimSpace(line)
		run := 0
		if len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
			for run < len(trimmed) && trimmed[run] == trimmed[0] {
				run++
			}
		}
		if fenceLength == 0 {
			if run >= 3 {
				fenceByte, fenceLength = trimmed[0], run
			} else {
				fn(offset, line)
			}
		} else if run >= fenceLength && trimmed[0] == fenceByte && strings.TrimSpace(trimmed[run:]) == "" {
			fenceLength = 0
		}
		offset = end + 1
	}
}

// ReadFilledBrief refuses unfinished review instructions before dispatch.
func ReadFilledBrief(file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read panel brief %s: %s", termsafe.Escape(file), termsafe.Escape(err.Error()))
	}
	if strings.Contains(string(data), "TODO(skill)") {
		return nil, fmt.Errorf("panel brief %s contains TODO(skill) stubs; fill it or pass --brief-file to mindspec panel create", termsafe.Escape(file))
	}
	return data, nil
}

func readBriefFile(file string) ([]byte, error) {
	if !filepath.IsAbs(file) {
		return nil, fmt.Errorf("brief file must be an absolute path: %s", termsafe.Escape(file))
	}
	info, err := os.Lstat(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read brief file %s: %s", termsafe.Escape(file), termsafe.Escape(err.Error()))
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("brief file must be a regular file: %s", termsafe.Escape(file))
	}
	return ReadFilledBrief(file)
}
