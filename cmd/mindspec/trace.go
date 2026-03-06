package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/trace"
	"github.com/spf13/cobra"
)

var traceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Inspect and analyze trace files",
}

var traceSummaryCmd = &cobra.Command{
	Use:   "summary <trace-file>",
	Short: "Print aggregate stats from a trace file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("opening trace file: %w", err)
		}
		defer f.Close()

		type eventStats struct {
			Count    int
			TotalMs  float64
			TotalTok int
		}

		stats := make(map[string]*eventStats)
		var totalMs float64
		var totalTokens int
		var eventCount int

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var e trace.Event
			if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
				continue
			}
			eventCount++

			s, ok := stats[e.Event]
			if !ok {
				s = &eventStats{}
				stats[e.Event] = s
			}
			s.Count++
			s.TotalMs += e.DurMs
			s.TotalTok += e.Tokens
			totalMs += e.DurMs
			totalTokens += e.Tokens
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading trace file: %w", err)
		}

		if eventCount == 0 {
			fmt.Println("No trace events found.")
			return nil
		}

		fmt.Printf("Trace Summary: %s\n", args[0])
		fmt.Printf("  Events:     %d\n", eventCount)
		fmt.Printf("  Duration:   %.1f ms\n", totalMs)
		fmt.Printf("  Tokens:     %d\n", totalTokens)
		fmt.Println()

		// Sort event types by name
		names := make([]string, 0, len(stats))
		for name := range stats {
			names = append(names, name)
		}
		sort.Strings(names)

		// Header
		fmt.Printf("  %-25s %6s %10s %8s\n", "Event", "Count", "Duration", "Tokens")
		fmt.Printf("  %s\n", strings.Repeat("-", 53))

		for _, name := range names {
			s := stats[name]
			durStr := "-"
			if s.TotalMs > 0 {
				durStr = fmt.Sprintf("%.1f ms", s.TotalMs)
			}
			tokStr := "-"
			if s.TotalTok > 0 {
				tokStr = fmt.Sprintf("%d", s.TotalTok)
			}
			fmt.Printf("  %-25s %6d %10s %8s\n", name, s.Count, durStr, tokStr)
		}

		return nil
	},
}

func init() {
	traceCmd.AddCommand(traceSummaryCmd)
}
