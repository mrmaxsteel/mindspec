package next

import "testing"

func TestS134Title_WorkItemTag(t *testing.T) {
	for title, want := range map[string]string{
		"[mindspec] [042-example] Heading":        "042-example",
		"[mindspec] [SPEC 042-example] Heading":   "042-example",
		"[042-example] Heading":                   "042-example",
		"[mindspec] [IMPL 042-example.2] Heading": "042-example",
		"[mindspec] [../../hostile] Heading":      "",
	} {
		if got := parseSpecID(title); got != want {
			t.Fatalf("%q -> %q, want %q", title, got, want)
		}
	}
}
