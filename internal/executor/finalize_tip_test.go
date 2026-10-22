package executor

import (
	"errors"
	"testing"
)

func TestFinalizeEpic_FinalizeTipIsPublishedTip(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "push"}[remote], func(t *testing.T) {
			g, fake, root := finalizePreflightFixture(t)
			// Keep the scoped bead only. Other fixture rows test scoping elsewhere.
			fake.listEntries = fake.listEntries[:1]
			before := refHash(t, root, "spec/077-test")
			var origin string
			if remote {
				origin = t.TempDir()
				runGitIn(t, origin, "init", "--bare", "-b", "main")
				runGitIn(t, root, "remote", "add", "origin", "file://"+origin)
				runGitIn(t, root, "push", "origin", "main")
			}
			oldRead, oldWrite := finalizeMetadataReadFn, finalizeMetadataWriteFn
			t.Cleanup(func() { finalizeMetadataReadFn, finalizeMetadataWriteFn = oldRead, oldWrite })
			metadata := map[string]map[string]interface{}{}
			finalizeMetadataWriteFn = func(id string, updates map[string]interface{}) error {
				if metadata[id] == nil {
					metadata[id] = map[string]interface{}{}
				}
				for key, value := range updates {
					metadata[id][key] = value
				}
				return nil
			}
			finalizeMetadataReadFn = func(id string) (map[string]interface{}, error) { return metadata[id], nil }
			stop := errors.New("stop before cleanup")
			t.Cleanup(SetFinalizeStepHookForTest(func(stage string) error {
				if stage == "pre_cleanup" {
					return stop
				}
				return nil
			}))
			_, err := g.FinalizeEpic("epic-1", "077-test", "spec/077-test", []string{"bead-1"}, "", false, remote)
			if !errors.Is(err, stop) {
				t.Fatalf("finalize: %v", err)
			}
			published := refHash(t, root, "spec/077-test")
			if published == before {
				t.Fatal("fixture made no finalize-time commit or auto-merge")
			}
			if metadata["epic-1"]["mindspec_finalize_tip"] != published {
				t.Fatalf("finalize tip = %v, want %s", metadata["epic-1"]["mindspec_finalize_tip"], published)
			}
			if metadata["epic-1"]["mindspec_pre_finalize_tip"] != before {
				t.Fatal("routing checkpoint was overwritten")
			}
			if remote {
				if got := refHash(t, origin, "spec/077-test"); got != published {
					t.Fatalf("pushed tip = %s, want %s", got, published)
				}
			} else if !isAncestorIn(t, root, published, "main") {
				t.Fatal("finalize tip did not reach main")
			}
		})
	}
}
