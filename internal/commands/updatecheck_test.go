package commands

import "testing"

func TestParseSemver(t *testing.T) {
	cases := []struct {
		in    string
		want  [3]int
		ok    bool
	}{
		{"0.1.3", [3]int{0, 1, 3}, true},
		{"1.0.0", [3]int{1, 0, 0}, true},
		{"10.20.30", [3]int{10, 20, 30}, true},
		{"1.2.3-beta", [3]int{1, 2, 3}, true},     // pre-release suffix stripped
		{"1.2.3+build.5", [3]int{1, 2, 3}, true},  // build metadata stripped
		{"", [3]int{}, false},
		{"1.2", [3]int{}, false},                  // too few components
		{"1.2.3.4", [3]int{}, false},              // too many
		{"a.b.c", [3]int{}, false},
		{"1.2.x", [3]int{}, false},
		{"-1.0.0", [3]int{}, false},               // negative
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := parseSemver(tc.in)
			if ok != tc.ok {
				t.Errorf("parseSemver(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("parseSemver(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsNewerTag(t *testing.T) {
	cases := []struct {
		tag     string
		current string
		want    bool
	}{
		{"v0.1.4", "0.1.3", true},
		{"v0.1.4", "0.1.4", false},    // same version — not newer
		{"v0.1.3", "0.1.4", false},    // older tag
		{"v0.2.0", "0.1.99", true},    // minor bump
		{"v1.0.0", "0.99.99", true},   // major bump
		{"0.1.4", "0.1.3", true},      // tag without v prefix
		{"v0.1.4", "", false},         // unparseable current
		{"garbage", "0.1.3", false},   // unparseable tag
		{"v0.1.4-beta", "0.1.3", true}, // pre-release tag still compares by MMP
		{"v0.1.3-beta", "0.1.3", false}, // same MMP, pre-release treated as equal (not newer)
	}
	for _, tc := range cases {
		t.Run(tc.tag+"_vs_"+tc.current, func(t *testing.T) {
			if got := isNewerTag(tc.tag, tc.current); got != tc.want {
				t.Errorf("isNewerTag(%q, %q) = %v, want %v", tc.tag, tc.current, got, tc.want)
			}
		})
	}
}
