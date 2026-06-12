package version

import "testing"

func TestCurrent_DefaultsToDev(t *testing.T) {
	// The bare version var (cmd/mindspec/root.go:35) defaults to "dev"
	// on every non-release/local/test build.
	if got := Current(); got != "dev" {
		t.Fatalf("Current() = %q, want %q (the root.go:35 default)", got, "dev")
	}
}

func TestSet_InjectsBareVersion(t *testing.T) {
	orig := current
	t.Cleanup(func() { current = orig })

	Set("1.4.2")
	if got := Current(); got != "1.4.2" {
		t.Fatalf("after Set, Current() = %q, want %q", got, "1.4.2")
	}
	// A blank Set must not clobber the existing value.
	Set("   ")
	if got := Current(); got != "1.4.2" {
		t.Fatalf("blank Set clobbered Current() = %q, want %q", got, "1.4.2")
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		ok   bool
		want Semver
	}{
		{"1.2.3", true, Semver{1, 2, 3}},
		{"v1.2.3", true, Semver{1, 2, 3}},
		{"0.0.0", true, Semver{0, 0, 0}},
		{"2.10", true, Semver{2, 10, 0}},
		{"3", true, Semver{3, 0, 0}},
		{"1.2.3-rc1", true, Semver{1, 2, 3}},
		{"1.2.3+build.5", true, Semver{1, 2, 3}},
		{"  1.2.3 ", true, Semver{1, 2, 3}},
		// dev / unparseable → ok == false (the DQ4 unbounded-newest class).
		{"dev", false, Semver{}},
		{"DEV", false, Semver{}},
		{"", false, Semver{}},
		{"abc", false, Semver{}},
		{"1.x.3", false, Semver{}},
		{"1.2.3.4", false, Semver{}},
		{"-1.2.3", false, Semver{}},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		if ok != c.ok {
			t.Errorf("Parse(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestCompare_ConcreteOrdering(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b string
		cmp  int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.4", "1.2.3", 1},
		{"1.2.3", "1.2.4", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.10.0", "1.9.0", 1},
		{"v2.0.0", "2.0.0", 0},
	}
	for _, c := range cases {
		got, ok := Compare(c.a, c.b)
		if !ok {
			t.Errorf("Compare(%q,%q) ok=false, want comparable", c.a, c.b)
			continue
		}
		if got != c.cmp {
			t.Errorf("Compare(%q,%q) = %d, want %d", c.a, c.b, got, c.cmp)
		}
	}
}

// TestCompare_DevPolicy pins the DQ4 dev→unbounded-newest seam: whenever
// either operand is dev/unparseable, Compare reports not-comparable
// (ok==false), which the caller resolves as "fail toward surfacing"
// (REGRESSION). A total order cannot satisfy both DQ4 statements, so the
// not-comparable signal is the only self-consistent reading.
func TestCompare_DevPolicy(t *testing.T) {
	t.Parallel()
	cases := []struct{ a, b string }{
		{"dev", "1.2.3"},   // running dev vs resolved vX → regression
		{"1.2.3", "dev"},   // resolved dev vs running vX → regression
		{"dev", "dev"},     // both dev
		{"garbage", "1.0"}, // unparseable running
		{"1.0", "garbage"}, // unparseable resolved
	}
	for _, c := range cases {
		cmp, ok := Compare(c.a, c.b)
		if ok {
			t.Errorf("Compare(%q,%q) ok=true, want ok=false (dev/unparseable)", c.a, c.b)
		}
		if cmp != 0 {
			t.Errorf("Compare(%q,%q) cmp=%d, want 0 when not comparable", c.a, c.b, cmp)
		}
	}
}

// TestRegressionClassification exercises the full DQ4 truth table the way
// Bead 3 will: regression iff !comparable OR running >= resolved.
func TestRegressionClassification(t *testing.T) {
	t.Parallel()
	regression := func(running, resolved string) bool {
		cmp, ok := Compare(running, resolved)
		if !ok {
			return true // fail toward surfacing
		}
		return cmp >= 0
	}
	cases := []struct {
		running, resolved string
		wantRegression    bool
	}{
		{"2.0.0", "2.0.0", true},  // == boundary → regression
		{"2.0.1", "2.0.0", true},  // > → regression
		{"1.9.9", "2.0.0", false}, // < → stale
		{"dev", "2.0.0", true},    // running dev → regression
		{"3.0.0", "dev", true},    // resolved dev → regression
	}
	for _, c := range cases {
		if got := regression(c.running, c.resolved); got != c.wantRegression {
			t.Errorf("regression(running=%q, resolved=%q) = %v, want %v",
				c.running, c.resolved, got, c.wantRegression)
		}
	}
}
