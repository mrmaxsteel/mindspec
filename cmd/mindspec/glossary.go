package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mindspec/mindspec/internal/glossary"
	"github.com/mindspec/mindspec/internal/trace"
	"github.com/mindspec/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var glossaryCmd = &cobra.Command{
	Use:   "glossary",
	Short: "Glossary-based context injection commands",
	Long:  `Parse, match, and display glossary entries for deterministic context injection.`,
}

var glossaryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all glossary terms and targets",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		entries, err := glossary.Parse(root)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Println("No glossary entries found.")
			return nil
		}

		// Find max term width for alignment
		maxWidth := 0
		for _, e := range entries {
			if len(e.Term) > maxWidth {
				maxWidth = len(e.Term)
			}
		}

		for _, e := range entries {
			fmt.Printf("  %-*s  %s\n", maxWidth, e.Term, e.Target)
		}
		fmt.Printf("\n%d terms\n", len(entries))
		return nil
	},
}

var glossaryMatchCmd = &cobra.Command{
	Use:   `match "<text>"`,
	Short: "Match glossary terms against input text",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		entries, err := glossary.Parse(root)
		if err != nil {
			return err
		}

		matched := glossary.Match(entries, args[0])

		// Emit trace event for glossary match
		tokensMatched := 0
		for _, e := range matched {
			tokensMatched += trace.EstimateTokens(e.Term + " " + e.Target)
		}
		trace.Emit(trace.NewEvent("glossary.match").
			WithTokens(tokensMatched).
			WithData(map[string]any{
				"query":          args[0],
				"hit_count":      len(matched),
				"tokens_matched": tokensMatched,
			}))

		if len(matched) == 0 {
			fmt.Println("No matching terms found.")
			return nil
		}

		for _, e := range matched {
			fmt.Printf("  %s -> %s\n", e.Term, e.Target)
		}
		fmt.Printf("\n%d matches\n", len(matched))
		return nil
	},
}

var glossaryShowCmd = &cobra.Command{
	Use:   `show <term>`,
	Short: "Display the documentation section linked to a glossary term",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			return err
		}

		entries, err := glossary.Parse(root)
		if err != nil {
			return err
		}

		termName := args[0]
		var entry *glossary.Entry
		termLower := strings.ToLower(termName)
		for i := range entries {
			if strings.ToLower(entries[i].Term) == termLower {
				entry = &entries[i]
				break
			}
		}

		if entry == nil {
			return fmt.Errorf("term %q not found in glossary — run 'mindspec glossary list' to see available terms", termName)
		}

		section, err := glossary.ExtractSection(root, entry.FilePath, entry.Anchor)
		if err != nil {
			return err
		}

		fmt.Println(section)
		return nil
	},
}

func findRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}
	root, err := workspace.FindRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("workspace not found: %w", err)
	}
	return root, nil
}

func init() {
	glossaryCmd.AddCommand(glossaryListCmd)
	glossaryCmd.AddCommand(glossaryMatchCmd)
	glossaryCmd.AddCommand(glossaryShowCmd)
}
