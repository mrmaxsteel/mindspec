package phase

import "testing"

func TestS134Title_EpicTag(t *testing.T) {
	for _, title := range []string{"[SPEC 042-example] Example", "[mindspec] [SPEC 042-example] Example"} {
		if id := titleSpecID(title); id != "042-example" {
			t.Fatalf("%q -> %q", title, id)
		}
		num, slug := ParseSpecFromTitle(title)
		if num != 42 || slug != "example" {
			t.Fatalf("%q -> %d %s", title, num, slug)
		}
	}
}
