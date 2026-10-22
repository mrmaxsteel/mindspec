package doctor

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/shellquote"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func checkStaleCheckouts(r *Report, root string, cfg *config.Config) {
	base := filepath.Join(root, cfg.Panel.IsolationRoot)
	_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if !os.IsNotExist(walkErr) {
				r.Checks = append(r.Checks, Check{Name: "Panel checkouts", Status: Warn, Message: walkErr.Error()})
			}
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		parts := strings.Split(rel, string(filepath.Separator))
		isMetadata := len(parts) == 5 && entry.Name() == "slot.json"
		if entry.IsDir() && !isMetadata {
			if entry.Name() == ".locks" || len(parts) > 4 {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMetadata {
			return nil
		}
		data, err := workspace.ReadSlotMetadata(path)
		if err != nil {
			r.Checks = append(r.Checks, Check{Name: "Panel checkouts", Status: Warn, Message: fmt.Sprintf("%s: %s\nrecovery: repair the slot metadata", termsafe.Escape(path), termsafe.Escape(err.Error()))})
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		var metadata panel.SlotMetadata
		if json.Unmarshal(data, &metadata) != nil {
			return nil
		}
		if metadata.Spec != "" && metadata.Spec != "adhoc" {
			if err := idvalidate.SpecID(metadata.Spec); err != nil {
				r.Checks = append(r.Checks, Check{Name: "Panel checkouts", Status: Warn, Message: fmt.Sprintf("%s: %v\nrecovery: repair the slot metadata", path, err)})
				return nil
			}
		}
		layout, err := panel.NewSlotLayout(root, strings.TrimSuffix(cfg.Panel.IsolationRoot, "/"), metadata.Spec, metadata.Slug, metadata.Round, metadata.Slot)
		if err != nil || layout.Metadata != path || filepath.Clean(metadata.MainRoot) != filepath.Clean(root) {
			return nil
		}
		roots := []string{root}
		if metadata.Spec != "" && metadata.Spec != "adhoc" {
			if specDir, err := workspace.SpecDir(root, metadata.Spec); err == nil {
				roots = append(roots, specDir)
			}
		}
		reason := "panel is missing"
		for _, registration := range panel.Scan(roots...) {
			if registration.Slug() != metadata.Slug || registration.Err != nil {
				continue
			}
			p := registration.Panel
			if p.Spec != metadata.Spec && !(p.Spec == "" && metadata.Spec == "adhoc") {
				continue
			}
			switch {
			case p.Abandoned:
				reason = "panel is abandoned"
			case p.Round > metadata.Round:
				reason = "panel is at a later round"
			default:
				return nil
			}
			break
		}
		recovery := "mindspec panel checkout " + shellquote.Quote(termsafe.Escape(metadata.Slug)) + " --prune"
		if metadata.Spec != "" && metadata.Spec != "adhoc" {
			recovery += " --spec " + shellquote.Quote(metadata.Spec)
		}
		r.Checks = append(r.Checks, Check{Name: "Stale panel checkout", Status: Warn, Message: fmt.Sprintf("%s: %s\nrecovery: %s", layout.Dir, reason, recovery)})
		return nil
	})
}
