package readiness

import (
	"reflect"
	"sort"
	"testing"
)

func TestMF2_FormSymmetry(t *testing.T) {
	for _, tc := range []struct {
		text string
		keys []string
	}{
		{"R9a", []string{"R9a"}},
		{"R9(a)", []string{"R9a"}},
		{"AC-2b", []string{"AC-2b"}},
		{"AC-2(b)", []string{"AC-2b"}},
		{"R5(ii)", []string{"R5"}},
		{"R5(i)", []string{"R5"}},
		{"R9", []string{"R9"}},
		{"AC9", []string{"AC-9"}},
		{"AC-9", []string{"AC-9"}},
		{"AC2(b)", []string{"AC-2b"}},
		{"R6.3", []string{"R6"}},
		{"AC-2.3", []string{"AC-2"}},
		{"R3(c)", []string{"R3c"}},
		{"R3c", []string{"R3c"}},
		{"AC-2b R9(a) R6.3 AC9", []string{"AC-2b", "AC-9", "R6", "R9a"}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			claims := harvestClaims(tc.text, 129)
			if len(claims) == 0 {
				t.Fatal("harvest must not be empty")
			}
			var harvested []string
			for _, c := range claims {
				harvested = append(harvested, c.base+c.letter)
			}
			harvested = sortedUnique(harvested)
			bases, exact := extractSpecTokens(tc.text)
			// The index also retains bases for letterless-claim resolution.
			// Compare that map independently, then compare the actual keys.
			wantBases := map[string]bool{}
			wantExact := map[string]bool{}
			for _, c := range claims {
				wantBases[c.base] = true
				if c.subLettered {
					wantExact[c.base+c.letter] = true
				}
			}
			var indexed []string
			for key := range exact {
				indexed = append(indexed, key)
			}
			for key := range bases {
				hasLetter := false
				for _, c := range claims {
					if c.base == key && c.subLettered {
						hasLetter = true
					}
				}
				if !hasLetter {
					indexed = append(indexed, key)
				}
			}
			sort.Strings(indexed)
			if !reflect.DeepEqual(harvested, tc.keys) || !reflect.DeepEqual(indexed, tc.keys) {
				t.Fatalf("harvest=%v index=%v, want exact keys %v", harvested, indexed, tc.keys)
			}
			if !reflect.DeepEqual(bases, wantBases) || !reflect.DeepEqual(exact, wantExact) {
				t.Fatalf("index maps=(%v,%v), want (%v,%v)", bases, exact, wantBases, wantExact)
			}
		})
	}
}
