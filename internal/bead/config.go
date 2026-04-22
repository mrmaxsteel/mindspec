package bead

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RequiredBeadsConfigKeys lists the mindspec-required keys and their canonical
// values for .beads/config.yaml, excluding issue-prefix (which is derived from
// the repo directory name and never overwritten).
//
// Exported so downstream checks (e.g. doctor) can reference the same source.
var RequiredBeadsConfigKeys = []RequiredKey{
	{Key: "types.custom", Value: "gate"},
	{Key: "status.custom", Value: "resolved"},
	{Key: "export.git-add", Value: false},
}

// RequiredKey is one mindspec-required setting for .beads/config.yaml.
type RequiredKey struct {
	Key   string
	Value any
}

// ConfigDrift records a required key whose user-authored value disagrees with
// the mindspec canonical value. It is returned for reporting; the value is
// left alone unless EnsureBeadsConfig is called with force=true.
//
// HaveRaw is a best-effort YAML rendering of the user's current value — for
// scalars it is the literal string, for sequences/mappings it is a compact
// rendering from yaml.Marshal (trailing newline stripped).
type ConfigDrift struct {
	Key     string
	Want    any
	HaveRaw string
}

// ConfigResult summarizes the effect of an EnsureBeadsConfig call.
type ConfigResult struct {
	Added          []string
	AlreadyCorrect []string
	UserAuthored   []ConfigDrift
	CreatedFile    bool
}

// HasBeadsDir reports whether root contains a .beads/ directory. Callers use
// it to decide whether it's worth invoking EnsureBeadsConfig or ScanBeadsConfig
// at all — projects that haven't run `bd init` have nothing to patch.
func HasBeadsDir(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".beads"))
	return err == nil && info.IsDir()
}

// FormatSummary renders a ConfigResult as a short human-readable block for
// init/setup/doctor output. Returns an empty string when the result carries no
// news (no additions, no drift, no fresh-file creation) so callers can omit a
// whole section instead of printing an empty header.
//
// The header line is caller-agnostic ("Beads config (.beads/config.yaml):");
// if a caller wants to disambiguate scan vs. mutate modes it can prepend its
// own line before calling FormatSummary.
func (r *ConfigResult) FormatSummary() string {
	if r == nil {
		return ""
	}
	if len(r.Added) == 0 && len(r.UserAuthored) == 0 && !r.CreatedFile {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Beads config (.beads/config.yaml):\n")
	if r.CreatedFile {
		sb.WriteString("  + created with mindspec-required keys\n")
	}
	for _, k := range r.Added {
		sb.WriteString("  + ")
		sb.WriteString(k)
		sb.WriteString("\n")
	}
	for _, drift := range r.UserAuthored {
		// %q on both sides so quoting is consistent; Want is typically a
		// string or bool, %q handles both via fmt's default formatting of
		// the generic any.
		fmt.Fprintf(&sb, "  ! %s: user value %q left in place (mindspec wants %q)\n",
			drift.Key, drift.HaveRaw, fmt.Sprint(drift.Want))
	}
	return sb.String()
}

// ScanBeadsConfig is the read-only variant of EnsureBeadsConfig. It returns
// the same ConfigResult describing what EnsureBeadsConfig would add, preserve,
// or flag as drift — without writing to disk. Callers use it to report drift
// without side effects (e.g. `mindspec doctor` without `--fix`).
func ScanBeadsConfig(root string) (*ConfigResult, error) {
	cfgPath := filepath.Join(root, ".beads", "config.yaml")

	data, err := os.ReadFile(cfgPath)
	created := false
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		created = true
	}
	if !created && len(bytes.TrimSpace(data)) == 0 {
		created = true
	}

	res := &ConfigResult{CreatedFile: created}
	if created {
		res.Added = append(res.Added, "issue-prefix")
		for _, rk := range RequiredBeadsConfigKeys {
			res.Added = append(res.Added, rk.Key)
		}
		return res, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	mapping := rootMapping(&doc)
	if mapping == nil {
		return nil, fmt.Errorf("config root is not a YAML mapping")
	}

	if _, ok := findMapEntry(mapping, "issue-prefix"); !ok {
		res.Added = append(res.Added, "issue-prefix")
	}
	for _, rk := range RequiredBeadsConfigKeys {
		valNode, ok := findMapEntry(mapping, rk.Key)
		switch {
		case !ok:
			res.Added = append(res.Added, rk.Key)
		case scalarEquals(valNode, rk.Value):
			res.AlreadyCorrect = append(res.AlreadyCorrect, rk.Key)
		default:
			res.UserAuthored = append(res.UserAuthored, ConfigDrift{
				Key:     rk.Key,
				Want:    rk.Value,
				HaveRaw: renderNodeValue(valNode),
			})
		}
	}
	return res, nil
}

// EnsureBeadsConfig idempotently applies the mindspec-required keys to
// <root>/.beads/config.yaml. Existing keys, values, and comments outside the
// mindspec-required set are preserved byte-for-byte via yaml.v3 Node editing.
//
// Behavior per key:
//   - issue-prefix: written only when absent; the value defaults to
//     filepath.Base(root). An existing issue-prefix is never overwritten and
//     never reported as drift (the user's project-naming choice is sovereign).
//   - types.custom / status.custom / export.git-add: if absent, added. If
//     present with the canonical value, counted in AlreadyCorrect. If present
//     with a different value, recorded in UserAuthored and left alone unless
//     force=true, in which case they are replaced and reported in Added.
//
// If the file does not exist, it is created with a brief header comment.
// Writes are atomic (temp file + os.Rename).
func EnsureBeadsConfig(root string, force bool) (*ConfigResult, error) {
	beadsDir := filepath.Join(root, ".beads")
	// bd recommends 0o700 on .beads (contains SQLite/Dolt runtime state).
	if err := os.MkdirAll(beadsDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir .beads: %w", err)
	}
	cfgPath := filepath.Join(beadsDir, "config.yaml")

	data, err := os.ReadFile(cfgPath)
	created := false
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		created = true
	}
	// Treat an empty existing file the same as a missing one: an empty YAML
	// document has no root mapping to merge into, and writing a fresh template
	// is the obviously correct outcome.
	if !created && len(bytes.TrimSpace(data)) == 0 {
		created = true
	}

	res := &ConfigResult{CreatedFile: created}
	defaultPrefix := filepath.Base(root)

	var out []byte
	if created {
		out, err = freshConfig(defaultPrefix, res)
		if err != nil {
			return nil, err
		}
	} else {
		out, err = mergeConfig(data, defaultPrefix, force, res)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(out, data) {
			return res, nil
		}
	}

	if err := atomicWrite(cfgPath, out); err != nil {
		return nil, err
	}
	return res, nil
}

const freshHeader = "# .beads/config.yaml — managed by mindspec\n" +
	"# User-authored keys are preserved; mindspec only manages the keys in\n" +
	"# RequiredBeadsConfigKeys plus issue-prefix (set once, never overwritten).\n"

// freshConfig builds a new config file with the mindspec-required keys in a
// deterministic order: issue-prefix first, then RequiredBeadsConfigKeys in
// declaration order. Built via yaml.Node so map-iteration randomness can't
// leak into the output.
func freshConfig(defaultPrefix string, res *ConfigResult) ([]byte, error) {
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendScalarEntry(mapping, "issue-prefix", defaultPrefix)
	res.Added = append(res.Added, "issue-prefix")
	for _, rk := range RequiredBeadsConfigKeys {
		appendScalarEntry(mapping, rk.Key, rk.Value)
		res.Added = append(res.Added, rk.Key)
	}
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping}}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encode fresh config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("close encoder: %w", err)
	}
	return append([]byte(freshHeader), buf.Bytes()...), nil
}

// mergeConfig applies required keys to an existing config while preserving
// user-authored keys and comments byte-for-byte.
func mergeConfig(data []byte, defaultPrefix string, force bool, res *ConfigResult) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	mapping := rootMapping(&doc)
	if mapping == nil {
		return nil, fmt.Errorf("config root is not a YAML mapping")
	}

	if _, ok := findMapEntry(mapping, "issue-prefix"); !ok {
		appendScalarEntry(mapping, "issue-prefix", defaultPrefix)
		res.Added = append(res.Added, "issue-prefix")
	}

	for _, rk := range RequiredBeadsConfigKeys {
		applyRequiredKey(mapping, rk, force, res)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("close encoder: %w", err)
	}
	return buf.Bytes(), nil
}

func applyRequiredKey(mapping *yaml.Node, rk RequiredKey, force bool, res *ConfigResult) {
	valNode, ok := findMapEntry(mapping, rk.Key)
	if !ok {
		appendScalarEntry(mapping, rk.Key, rk.Value)
		res.Added = append(res.Added, rk.Key)
		return
	}
	if scalarEquals(valNode, rk.Value) {
		res.AlreadyCorrect = append(res.AlreadyCorrect, rk.Key)
		return
	}
	if !force {
		res.UserAuthored = append(res.UserAuthored, ConfigDrift{
			Key:     rk.Key,
			Want:    rk.Value,
			HaveRaw: renderNodeValue(valNode),
		})
		return
	}
	setScalarValue(valNode, rk.Value)
	res.Added = append(res.Added, rk.Key)
}

// renderNodeValue returns a best-effort string rendering of a YAML node's
// value. Scalars return their literal .Value; sequences and mappings are
// rendered via yaml.Marshal (trailing newline stripped) so callers can see
// what the user actually wrote even when it's not a scalar.
func renderNodeValue(n *yaml.Node) string {
	if n.Kind == yaml.ScalarNode {
		return n.Value
	}
	out, err := yaml.Marshal(n)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

// rootMapping returns the top-level mapping node from a parsed document.
// A zero-content document (empty file) is promoted to an empty mapping in
// place so callers can append keys.
func rootMapping(doc *yaml.Node) *yaml.Node {
	if doc.Kind != yaml.DocumentNode {
		return nil
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	return root
}

// findMapEntry walks a MappingNode's Content (alternating key/value nodes)
// and returns the value node and whether the key was found.
func findMapEntry(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1], true
		}
	}
	return nil, false
}

func appendScalarEntry(mapping *yaml.Node, key string, value any) {
	k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	v := scalarNodeFor(value)
	mapping.Content = append(mapping.Content, k, v)
}

func scalarNodeFor(value any) *yaml.Node {
	switch v := value.(type) {
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v, Style: yaml.DoubleQuotedStyle}
	case bool:
		out := "false"
		if v {
			out = "true"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: out}
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprintf("%v", v)}
	}
}

func setScalarValue(n *yaml.Node, value any) {
	replacement := scalarNodeFor(value)
	n.Kind = replacement.Kind
	n.Tag = replacement.Tag
	n.Value = replacement.Value
	n.Style = replacement.Style
	n.Content = nil
	n.Alias = nil
}

func scalarEquals(n *yaml.Node, want any) bool {
	if n.Kind != yaml.ScalarNode {
		return false
	}
	switch w := want.(type) {
	case string:
		return n.Value == w
	case bool:
		if w {
			return n.Value == "true"
		}
		return n.Value == "false"
	default:
		return n.Value == fmt.Sprintf("%v", w)
	}
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config.yaml.*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

// builtinStatuses lists the statuses bd recognizes out of the box. Callers
// that need to iterate every possible bead status combine this with
// CustomStatuses for a complete picture.
var builtinStatuses = []string{"open", "in_progress", "blocked", "closed"}

// BuiltinStatuses returns a fresh copy of the built-in bd status set.
func BuiltinStatuses() []string {
	out := make([]string, len(builtinStatuses))
	copy(out, builtinStatuses)
	return out
}

// CustomStatuses reads `status.custom` from <root>/.beads/config.yaml and
// returns the declared custom statuses. bd accepts either a scalar string
// ("resolved") or a comma-separated list ("resolved,paused"); both are
// normalised into individual trimmed entries here.
//
// An empty slice is returned if the file is missing, malformed, or
// declares no custom statuses — callers should tolerate that gracefully
// and fall back to BuiltinStatuses.
func CustomStatuses(root string) []string {
	path := filepath.Join(root, ".beads", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	raw, ok := cfg["status.custom"]
	if !ok {
		return nil
	}
	return splitCustomList(raw)
}

// AllStatuses returns the union of built-in and custom statuses for the
// project rooted at root. Order is stable: built-ins first, customs in
// declaration order, with case-insensitive dedup.
func AllStatuses(root string) []string {
	all := BuiltinStatuses()
	seen := make(map[string]bool, len(all))
	for _, s := range all {
		seen[strings.ToLower(s)] = true
	}
	for _, s := range CustomStatuses(root) {
		key := strings.ToLower(s)
		if seen[key] {
			continue
		}
		seen[key] = true
		all = append(all, s)
	}
	return all
}

// splitCustomList accepts either a single string or a []interface{} of
// strings from the YAML decoder and returns a normalised slice.
func splitCustomList(raw interface{}) []string {
	var out []string
	switch v := raw.(type) {
	case string:
		for _, piece := range strings.Split(v, ",") {
			if s := strings.TrimSpace(piece); s != "" {
				out = append(out, s)
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
		}
	}
	return out
}
