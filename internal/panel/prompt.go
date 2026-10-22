package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// PromptInput is resolved by the adapter. Launch supplies a fresh Nonce and its
// new attempt; the harness prompt uses the latest dispatched attempt.
type PromptInput struct {
	Brief      string
	Slot       string
	Family     string
	Model      string
	Lens       string
	Focus      string
	SlotDir    string
	Tree       string
	Kind       string
	Output     string
	Nonce      string
	Attempt    int
	PriorItems PriorItemsResult
}

// PriorItemsResult holds verified items or a warning about unavailable evidence.
type PriorItemsResult struct {
	Items   []string
	Warning string
}

func unavailablePriorItems(reason string) PriorItemsResult {
	return PriorItemsResult{Warning: "Prior-round items could not be read: " + reason + "."}
}

// RenderProcessOutput is shared by the read-only prompt and the launcher.
func RenderProcessOutput(output, nonce, verdictPath string) string {
	if output == "file" {
		return "Write the verdict JSON to " + termsafe.Escape(verdictPath) + ".\n"
	}
	return "Print exactly one verdict JSON object on stdout between these marker lines:\n" +
		"MINDSPEC-VERDICT-BEGIN " + termsafe.Escape(nonce) + "\n" +
		"MINDSPEC-VERDICT-END " + termsafe.Escape(nonce) + "\n"
}

func promptShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

// RenderSlotPrompt renders without writes or process calls.
func RenderSlotPrompt(in PromptInput) string {
	var b strings.Builder
	// Keep stored printable text unchanged. Escape any added control bytes.
	b.WriteString(escapeBriefText(in.Brief))
	b.WriteString("\n\n## Slot\n\n")
	fmt.Fprintf(&b, "Slot: %s\nFamily: %s\nModel (the verdict model value): %s\nLens: %s\nFocus: %s\n",
		termsafe.Escape(in.Slot), termsafe.Escape(in.Family), termsafe.Escape(in.Model), termsafe.Escape(in.Lens), termsafe.Escape(in.Focus))
	fmt.Fprintf(&b, "Slot directory: %s\nTree: %s\n", termsafe.Escape(in.SlotDir), termsafe.Escape(in.Tree))
	b.WriteString("Run checks in the slot tree at the reviewed commit.\n")
	b.WriteString("\n## Output\n\n")
	verdictPath := filepath.Join(in.SlotDir, fmt.Sprintf("attempt-%d", in.Attempt), "verdict.json")
	if in.Kind == "process" {
		b.WriteString(RenderProcessOutput(in.Output, in.Nonce, verdictPath))
	} else if in.Attempt > 0 {
		// Refuse non-printable or invalid path bytes. Display escapes change
		// the shell argument. Keep printable Unicode and quote characters.
		if !utf8.ValidString(verdictPath) || strings.IndexFunc(verdictPath, func(r rune) bool { return !strconv.IsPrint(r) }) >= 0 {
			b.WriteString("Cannot render the submission command: the verdict path contains a non-printable character or invalid UTF-8.\n")
		} else {
			b.WriteString("Submit the verdict JSON through this command:\n")
			b.WriteString("mindspec panel submit --file " + promptShellQuote(verdictPath) + "\n")
		}
	} else {
		b.WriteString("Use mindspec panel checkout with --slot and --dispatch. Then run mindspec panel prompt again for the submission command.\n")
	}
	b.WriteString("\n## Prior-round items\n\n")
	if in.PriorItems.Warning != "" {
		b.WriteString(termsafe.Escape(in.PriorItems.Warning) + "\n")
	} else if len(in.PriorItems.Items) == 0 {
		b.WriteString("no prior-round items\n")
	} else {
		b.WriteString("Grade each item ADDRESSED, PARTIAL, MISSED or NEW_ISSUE.\n")
		for i, item := range in.PriorItems.Items {
			fmt.Fprintf(&b, "%d. %s\n", i+1, termsafe.Escape(item))
		}
	}
	b.WriteString("\nWrite scratch only in the slot directory. Use absolute paths within it.\n")
	return b.String()
}

// PriorRoundItems selects N-1 before reading any submission or verdict. A file
// without the latest submission's digest cannot supply v2 reviewer evidence.
func PriorRoundItems(dir, slot string, round int) (PriorItemsResult, error) {
	if round < 2 {
		return PriorItemsResult{}, nil
	}
	lines, err := ReadLedger(dir)
	if err != nil {
		return PriorItemsResult{}, err
	}
	var prior RoundRecord
	for _, line := range lines {
		var r RoundRecord
		if err := json.Unmarshal(line, &r); err != nil {
			return PriorItemsResult{}, err
		}
		if r.Record == RecordRound && r.Round == round-1 {
			prior = r
		}
	}
	data, err := ReadVerdictFile(filepath.Join(dir, fmt.Sprintf("%s-round-%d.json", slot, round-1)))
	if os.IsNotExist(err) {
		if prior.VerdictSchema == 2 {
			return unavailablePriorItems("file is absent"), nil
		}
		return PriorItemsResult{}, nil
	}
	if err != nil {
		return PriorItemsResult{}, err
	}
	if prior.VerdictSchema == 2 {
		latest, err := LatestSubmissions(dir, round-1)
		if err != nil {
			return PriorItemsResult{}, err
		}
		submission, ok := latest[slot]
		if !ok {
			return unavailablePriorItems("no latest submission is recorded"), nil
		}
		if submission.FileSHA256 != findingHash(data) {
			return unavailablePriorItems("file digest differs from the latest submission"), nil
		}
		v, err := ParseVerdictV2(data)
		if err != nil {
			return unavailablePriorItems("file does not contain a valid verdict"), nil
		}
		items := make([]string, len(v.ConcreteChangesRequired))
		for i, item := range v.ConcreteChangesRequired {
			items[i] = item.Text
		}
		return PriorItemsResult{Items: items}, nil
	}
	var v FindingVerdict
	if json.Unmarshal(data, &v) != nil {
		return PriorItemsResult{}, nil
	}
	return PriorItemsResult{Items: v.ConcreteChangesRequired}, nil
}
