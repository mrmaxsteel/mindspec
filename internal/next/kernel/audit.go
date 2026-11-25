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
	names := []string{"--allow-doc-skew", "--override-adr", "--allow-not-ready", "--allow-net-deletion", "deferrals", "takeovers", "decision-answers", "raw-claims", "unreceipted-closes", "carry-overs", "re-panels", "verify-reproduced", "verify-refuted", "verify-deferred", "late-submissions", "repair-receipts", "lens-steps"}
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
	leaseTakes, repairs := 0, 0
	repanels := map[string]bool{}
	lensAttempts := map[lensRound]int{}
	var lensRounds []lensRound
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
		// Every take and repair event counts, whatever its generation.
		for _, event := range takes {
			byName["takeovers"].Count++
			if event.Via == "repair" {
				repairs++
			} else {
				leaseTakes++
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
			if receipt.Repaired {
				byName["unreceipted-closes"].Count++
			}
			if receipt.SupersededGeneration != 0 {
				byName["late-submissions"].Count++
			}
			if receipt.Via == "repair" {
				byName["repair-receipts"].Count++
			}
		}
		tuple, _, _ := r.Tuple()
		switch tuple.Kind {
		case "verify":
			returns, _, err := Decode[[]VerifyReturn](r.Metadata, KeyReturns)
			if err != nil {
				return nil, err
			}
			// Each return type has its own row: reproduced, refuted or deferred.
			for _, ret := range returns {
				if row := byName["verify-"+ret.Return]; row != nil {
					row.Count++
				}
			}
		case "lens":
			key := lensRound{tuple.Gate, tuple.Round}
			if lensAttempts[key] == 0 {
				lensRounds = append(lensRounds, key)
			}
			lensAttempts[key]++
		}
		repanel, has, err := Decode[RePanel](r.Metadata, KeyRePanel)
		if err != nil {
			return nil, err
		}
		// The gate attempt and its lens step carry the same judged scope; count the judgement once.
		if has && (repanel.Judge == "" || !repanels[repanel.Judge]) {
			if repanel.Judge != "" {
				repanels[repanel.Judge] = true
			}
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
	byName["takeovers"].Details = []string{fmt.Sprintf("lease %d, repair %d", leaseTakes, repairs)}
	sort.Slice(lensRounds, func(i, j int) bool {
		if lensRounds[i].gate != lensRounds[j].gate {
			return lensRounds[i].gate < lensRounds[j].gate
		}
		return lensRounds[i].round < lensRounds[j].round
	})
	lens := byName["lens-steps"]
	for _, key := range lensRounds {
		lens.Count += lensAttempts[key]
		lens.Details = append(lens.Details, fmt.Sprintf("%s r%d: %d", key.gate, key.round, lensAttempts[key]))
	}
	return rows, nil
}

// lensRound identifies one gate round for the lens-steps details.
type lensRound struct {
	gate  string
	round int
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
