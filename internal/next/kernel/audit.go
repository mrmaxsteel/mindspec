package kernel

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/journal"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// AuditRow is a count with optional durable event details.
type AuditRow struct {
	Event   string
	Count   int
	Details []string
}

// Audit reads typed records and the supplied local journal without writing.
// Journal records have no project identity, so their counts are machine-local.
func Audit(s State, events []journal.Record) ([]AuditRow, error) {
	names := []string{"--allow-doc-skew", "--override-adr", "--allow-not-ready", "--allow-net-deletion", "deferrals", "takeovers", "decision-answers", "raw-claims", "unreceipted-closes", "carry-overs", "re-panels"}
	rows := make([]AuditRow, len(names))
	byName := map[string]*AuditRow{}
	for i, name := range names {
		rows[i].Event = name
		byName[name] = &rows[i]
	}
	for _, event := range events {
		if row := byName["--"+event.EscapeHatch]; row != nil {
			row.Count++
		}
	}
	ids := make([]string, 0, len(s.Records))
	for id := range s.Records {
		if owned(s, id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := s.Records[id]
		claims, _, err := Decode[[]RawClaim](r.Metadata, KeyRawClaims)
		if err != nil {
			return nil, err
		}
		byName["raw-claims"].Count += len(claims)
		deferrals, has, err := Decode[[]DeferRecord](r.Metadata, KeyDeferrals)
		if err != nil {
			return nil, err
		}
		if !has {
			old, ok, err := Decode[DeferRecord](r.Metadata, KeyDefer)
			if err != nil {
				return nil, err
			}
			if ok {
				deferrals = append(deferrals, old)
			}
		}
		for _, event := range deferrals {
			if event.Finding != "" {
				byName["deferrals"].Count++
			}
		}
		takes, has, err := Decode[[]TakeReason](r.Metadata, KeyTakes)
		if err != nil {
			return nil, err
		}
		if !has {
			old, ok, err := Decode[TakeReason](r.Metadata, KeyTake)
			if err != nil {
				return nil, err
			}
			if ok {
				takes = append(takes, old)
			}
		}
		for _, event := range takes {
			if event.Generation > 0 {
				byName["takeovers"].Count++
			}
		}
		receipt, has, err := Decode[Receipt](r.Metadata, KeyClose)
		if err != nil {
			return nil, err
		}
		if has && ValidReceipt(s.Root, receipt, r.Metadata) == nil {
			if receipt.Answer != nil {
				row := byName["decision-answers"]
				row.Count++
				row.Details = append(row.Details, fmt.Sprintf("%s %s %s", id, receipt.Answer.Option, receipt.Answer.At.UTC().Format(time.RFC3339)))
			}
			byName["carry-overs"].Count += len(receipt.CarryOvers)
			if receipt.Repaired && r.Status == "closed" {
				byName["unreceipted-closes"].Count++
			}
		}
		if r.Status == "closed" && !validClose(s, r) && RepairedClose(s, id) {
			byName["unreceipted-closes"].Count++
		}
		repanel, has, err := Decode[RePanel](r.Metadata, KeyRePanel)
		if err != nil {
			return nil, err
		}
		if has {
			row := byName["re-panels"]
			row.Count++
			row.Details = append(row.Details, fmt.Sprintf("%s: %d slots", id, repanel.SlotCount))
		}
	}
	claims, err := ancestorClaimFlags(s)
	if err != nil {
		return nil, err
	}
	byName["raw-claims"].Count += len(claims.Flags)
	return rows, nil
}

// PrintAudit prints stable counts; event details never control approval.
func PrintAudit(out io.Writer, s State, rows []AuditRow) {
	fmt.Fprintf(out, "Kernel audit: round limit %d; lease TTL %s\n", s.Policy.RoundLimit, termsafe.Escape(s.Policy.LeaseTTL))
	fmt.Fprintln(out, "Event | Count | Details")
	for _, row := range rows {
		fmt.Fprintf(out, "%s | %d", row.Event, row.Count)
		for _, detail := range row.Details {
			fmt.Fprintf(out, " | %s", termsafe.Escape(detail))
		}
		fmt.Fprintln(out)
	}
}

// ApprovalBlockers reads the current snapshot. It does not run a reducer pass.
func ApprovalBlockers(s State) ([]Flag, error) {
	var flags []Flag
	for _, id := range append(append([]string{}, s.Gates...), s.Steps...) {
		r := s.Records[id]
		if r.Status == "closed" {
			continue
		}
		t, _, _ := r.Tuple()
		code := "open-step"
		if t.Kind == "decision" {
			code = "open-decision"
		}
		flags = append(flags, Flag{Code: code, Step: id})
	}
	for _, id := range s.Foreign {
		if s.Records[id].Status != "closed" {
			flags = append(flags, Flag{Code: "foreign-child-open", Step: id})
		}
	}
	for _, item := range s.Items {
		if err := CheckWorkItem(s, item); err != nil {
			message := strings.TrimSuffix(err.Error(), "\nrecovery: mindspec next-action")
			flags = append(flags, Flag{Code: "work-item-blocked", Step: item, Message: message, Repair: []string{"mindspec", "next-action"}})
		}
	}
	registry := Rules()
	for _, name := range []string{"unreceipted-close", "raw-close", "forced-close", "id-collision"} {
		rule, ok := registry.entries[name]
		if !ok {
			return nil, fmt.Errorf("approval rule %s is missing", name)
		}
		c, err := rule.fn(s)
		if err != nil {
			return nil, fmt.Errorf("approval rule %s: %w", name, err)
		}
		flags = append(flags, c.Flags...)
	}
	sort.Slice(flags, func(i, j int) bool {
		if flags[i].Code != flags[j].Code {
			return flags[i].Code < flags[j].Code
		}
		return flags[i].Step < flags[j].Step
	})
	return flags, nil
}
